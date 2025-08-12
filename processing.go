package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// processAllTasks orchestrates the main task processing workflow.
func processAllTasks(config *Config) error {
	allTasks, err := getActiveTasks(config)
	if err != nil {
		return fmt.Errorf("failed to retrieve tasks from Todoist: %w", err)
	}

	tasksToProcess := filterTasksForProcessing(allTasks, config)
	config.Logger.Printf("Found %d tasks to process out of %d total tasks", len(tasksToProcess), len(allTasks))

	return processTaskBatch(config, tasksToProcess)
}

// processTaskBatch handles batch processing with error collection using concurrent goroutines.
func processTaskBatch(config *Config, tasks []Task) error {
	const maxConcurrency = 25
	
	var failures []error
	var failuresMutex sync.Mutex
	successCount := 0
	var successMutex sync.Mutex
	
	// Create a semaphore to limit concurrent goroutines
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			if err := processTask(config, t); err != nil {
				config.Logger.Printf("Failed to process task %s (%s): %v", t.ID, t.Content, err)
				failuresMutex.Lock()
				failures = append(failures, fmt.Errorf("task %s: %w", t.ID, err))
				failuresMutex.Unlock()
			} else {
				successMutex.Lock()
				successCount++
				successMutex.Unlock()
			}
		}(task)
	}

	wg.Wait()
	return reportProcessingResults(config, successCount, len(tasks), failures)
}

// reportProcessingResults logs results and returns appropriate error.
func reportProcessingResults(config *Config, successCount, totalCount int, failures []error) error {
	config.Logger.Printf("Successfully processed %d tasks", successCount)

	if len(failures) == 0 {
		return nil
	}

	config.Logger.Printf("Failed to process %d tasks", len(failures))
	return fmt.Errorf("task processing failed: %d out of %d tasks failed: %w", len(failures), totalCount, failures[0])
}

// shouldProcessTask determines if a task should be processed based on auto-aging labels.
func shouldProcessTask(task Task, config *Config) bool {
	// Skip subtasks (tasks with a parent_id)
	if task.ParentID != nil {
		return false
	}

	hasNoAutoAgeLabel := false
	hasAutoAgeLabel := false

	for _, label := range task.Labels {
		switch label {
		case labelNoAutoAge:
			hasNoAutoAgeLabel = true
		case labelAutoAge:
			hasAutoAgeLabel = true
		}
	}

	// If both labels are present, prioritize no-autoage (safer default)
	if hasNoAutoAgeLabel {
		return false
	}

	// If autoage label is present, always process
	if hasAutoAgeLabel {
		return true
	}

	if config.AutoAgeByDefault {
		// If auto-aging is default, process unless explicitly opted out with @no-autoage
		return !hasNoAutoAgeLabel
	}
	// If auto-aging is not default, process only if explicitly opted in with @autoage
	return hasAutoAgeLabel
}

// filterTasksForProcessing filters tasks based on processing rules.
func filterTasksForProcessing(tasks []Task, config *Config) []Task {
	var tasksToProcess []Task
	for _, task := range tasks {
		if shouldProcessTask(task, config) {
			tasksToProcess = append(tasksToProcess, task)
			if config.Verbose {
				config.Logger.Printf("✅ Task %s marked for processing: %s", task.ID, task.Content)
			}
		} else {
			if config.Verbose {
				config.Logger.Printf("❌ Task %s filtered out: %s", task.ID, task.Content)
			}
		}
	}
	return tasksToProcess
}

// calculateTaskUpdate determines the new content and description for a task based on aging rules.
func calculateTaskUpdate(ctx TaskContext, now time.Time) TaskUpdateInfo {
	ageInfo := extractTaskAgingInfo(ctx.Task.Content)
	lastUpdated := extractMetadataTimestamp(ctx.Task.Description)
	nowInTZ := now.In(ctx.Timezone)

	// Start with current state
	result := TaskUpdateInfo{
		NewContent:     ctx.Task.Content,
		NewDescription: ctx.Task.Description,
		ShouldUpdate:   false,
	}

	// Always update description with current timestamp if we're making any change
	newDescription := updateDescriptionWithMetadata(ctx.Task.Description, nowInTZ)

	// Handle first-time tasks (no metadata)
	if lastUpdated.IsZero() {
		result.NewDescription = newDescription
		result.ShouldUpdate = true
		return result
	}

	// Check if enough time has passed to make changes (only for non-recurring tasks)
	// Recurring tasks use completion-based logic regardless of metadata timing
	if !ctx.IsRecurring {
		shouldIncrement := shouldIncrementBasedOnMidnight(lastUpdated, now, ctx.Timezone)
		if !shouldIncrement {
			return result // No change needed for non-recurring tasks
		}
	}

	// Calculate the new age count
	newCount := calculateNewAgeCount(ageInfo.AgeCount, ctx, lastUpdated, now)

	// Build new content
	switch {
	case newCount == 0:
		// When resetting, preserve any numbered format (e.g., "1) ") from original content
		result.NewContent = preserveNumberedFormatOnReset(ctx.Task.Content, ageInfo.ContentWithoutAge)
	case ageInfo.HasAgeMarkers:
		result.NewContent = addAgingMarkersToContent(ageInfo.ContentWithoutAge, newCount)
	default:
		result.NewContent = addAgingMarkersToContent(ctx.Task.Content, newCount)
	}

	// Only update if content actually changed
	if result.NewContent != ctx.Task.Content {
		result.NewDescription = newDescription
		result.ShouldUpdate = true
	}

	return result
}

// calculateNewAgeCount determines the new age count using linear logic.
func calculateNewAgeCount(currentCount int, ctx TaskContext, lastUpdated time.Time, now time.Time) int {
	// Handle recurring tasks first - they have special reset logic
	if ctx.IsRecurring && ctx.DaysSinceCompletion >= 0 {
		if ctx.DaysSinceCompletion == 0 {
			return 0 // Tasks completed today get reset to 0
		}
		return ctx.DaysSinceCompletion + 1 // Days since completion + 1
	}

	// Handle regular tasks - increment based on days passed
	daysSinceUpdate := calculateDaysSinceUpdate(lastUpdated, now, ctx.Timezone)
	if daysSinceUpdate > 0 {
		return currentCount + daysSinceUpdate
	}

	// Default: add first parenthesis for tasks without any
	if currentCount == 0 {
		return 1
	}

	return currentCount
}

// logTaskUpdate logs the action being taken on a task.
func logTaskUpdate(config *Config, task Task, ctx TaskContext, updateInfo TaskUpdateInfo) {
	ageInfo := extractTaskAgingInfo(task.Content)
	lastUpdated := extractMetadataTimestamp(task.Description)

	// Determine action type linearly
	actionType := "updating task"

	// First-time task
	if !ageInfo.HasAgeMarkers && lastUpdated.IsZero() {
		actionType = "first-time processing"
	}

	// Adding first parenthesis
	if !ageInfo.HasAgeMarkers && !lastUpdated.IsZero() {
		actionType = "adding first parenthesis"
	}

	// Recurring task reset
	if ctx.IsRecurring && ctx.DaysSinceCompletion >= 0 {
		if ctx.DaysSinceCompletion == 0 {
			actionType = "resetting recurring task (completed today)"
		} else {
			actionType = fmt.Sprintf("resetting recurring task (completed %d days ago)", ctx.DaysSinceCompletion)
		}
	}

	// Regular increment
	if !ctx.IsRecurring && ageInfo.HasAgeMarkers {
		actionType = "incrementing task"
	}

	// Capitalize first letter
	if len(actionType) > 0 {
		actionType = strings.ToUpper(actionType[:1]) + actionType[1:]
	}

	config.Logger.Printf("%s %s: \"%s\" -> \"%s\"",
		actionType, task.ID, task.Content, updateInfo.NewContent)
}

// processTask processes a single task according to aging rules.
func processTask(config *Config, task Task) error {
	now := time.Now()

	// Determine task characteristics
	isRecurring := isRecurringTask(task)
	daysSinceCompletion := -1

	// For recurring tasks, check completion status
	if isRecurring {
		var err error
		daysSinceCompletion, err = getDaysSinceCompletion(config, task.ID)
		if err != nil {
			config.Logger.Printf("Warning: Failed to get completion data for recurring task %s: %v", task.ID, err)
			// Use -1 to indicate failure - task will be processed as normal non-recurring task
			daysSinceCompletion = -1
		}
	}

	// Use pure function to determine what to do
	ctx := TaskContext{
		Task:                task,
		IsRecurring:         isRecurring,
		DaysSinceCompletion: daysSinceCompletion,
		Timezone:            config.Timezone,
	}

	updateInfo := calculateTaskUpdate(ctx, now)

	if config.Verbose {
		config.Logger.Printf("Task %s debug: isRecurring=%v, daysSinceCompletion=%d, shouldUpdate=%v", 
			task.ID, isRecurring, daysSinceCompletion, updateInfo.ShouldUpdate)
	}

	if !updateInfo.ShouldUpdate {
		if config.Verbose {
			config.Logger.Printf("Skipping task %s: no update needed", task.ID)
		}
		return nil
	}

	// Log what we're doing based on the specific action
	logTaskUpdate(config, task, ctx, updateInfo)

	// Handle dry run mode
	if config.DryRun {
		config.Logger.Printf("DRY RUN: Would update task %s", task.ID)
		return nil
	}

	// Perform the actual update
	return updateTask(config, task.ID, updateInfo.NewContent, updateInfo.NewDescription)
}
