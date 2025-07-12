// Package main implements todoscript - a Todoist task aging automation tool.
//
// This tool automatically increments visual "age markers" (parentheses) on Todoist tasks
// to help track how long tasks have been sitting in your todo list. The concept is simple:
// tasks get more parentheses the longer they remain incomplete, creating visual pressure
// to either complete them or remove them.
//
// How it works:
// - Tasks with pattern ") Do something" become ")) Do something" after midnight
// - Recurring tasks reset their age when completed: "))))) Task" → "))) Task"
// - Tasks can opt-in with @autoage label or opt-out with @no-autoage label
// - Dry-run mode available for testing changes before applying them
//
// The aging concept creates a visual indication of task staleness, encouraging you to
// either complete long-standing tasks or remove them from your list entirely.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Constants for configuration and business logic.
const (
	// Core business logic.
	taskAgingMarker  = ")"
	metadataTemplate = "[auto: lastUpdated=%s]"

	// HTTP client configuration.
	httpTimeoutSeconds     = 30
	maxIdleConnections     = 10
	maxIdleConnsPerHost    = 2
	idleConnTimeoutSeconds = 90

	// Environment variables.
	envLogFile        = "LOG_FILE"
	envTodoistToken   = "TODOIST_TOKEN"
	envDryRun         = "DRY_RUN"
	envVerbose        = "VERBOSE"
	envAutoAgeDefault = "AUTOAGE_BY_DEFAULT"
	envTimezone       = "TIMEZONE"

	// Default values.
	defaultTimezone = "UTC"

	// Task labels.
	labelRecurring = "recurring"
	labelNoAutoAge = "no-autoage"
	labelAutoAge   = "autoage"

	// API URLs.
	defaultAPIURL      = "https://api.todoist.com/rest/v2"
	defaultActivityURL = "https://api.todoist.com/sync/v9/activity/get"

	// API endpoints.
	apiEndpointTasks = "/tasks"
	apiEndpointTask  = "/tasks/%s"
)

// Config holds all application configuration.
type Config struct {
	TodoistToken     string
	APIURL           string
	ActivityURL      string
	DryRun           bool
	Verbose          bool
	AutoAgeByDefault bool
	Timezone         *time.Location
	Logger           *log.Logger
}

// Task represents a Todoist task.
type Task struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
	IsCompleted bool     `json:"is_completed"`
	Due         *DueDate `json:"due,omitempty"`
	ParentID    *string  `json:"parent_id"` // Pointer to allow for null values
}

// DueDate represents a task's due date information.
type DueDate struct {
	Recurring bool   `json:"is_recurring"`
	Date      string `json:"date,omitempty"`
}

// TaskAgeInfo represents the result of parsing age markers from a task.
type TaskAgeInfo struct {
	AgeCount          int    // Number of age markers (parentheses)
	ContentWithoutAge string // Task content with age markers removed
	HasAgeMarkers     bool   // Whether the task has age markers
}

// TaskUpdateInfo represents the result of calculating a task update.
type TaskUpdateInfo struct {
	NewContent     string // Updated task content
	NewDescription string // Updated task description
	ShouldUpdate   bool   // Whether the task should be updated
}

// UpdateAction represents the action to take on a task.
type UpdateAction struct {
	Action   string // "reset", "skip", or "increment"
	NewCount int    // New age count after action
}

// TaskContext contains all the information needed to process a task.
type TaskContext struct {
	Task                Task
	IsRecurring         bool
	DaysSinceCompletion int
	Timezone            *time.Location
}

// ActivityResponse represents the response from Todoist's activity API.
type ActivityResponse struct {
	Count  int `json:"count"`
	Events []struct {
		EventType string    `json:"event_type"`
		EventDate time.Time `json:"event_date"`
	} `json:"events"`
}

// Global variables for compiled patterns and HTTP client.
var (
	// Pre-compiled regex patterns for task aging
	// taskAgePattern matches tasks with age markers: "))) Do something"
	// Groups: (1) optional number, (2) parentheses markers, (3) remaining content.
	taskAgePattern = regexp.MustCompile(`^(\d*)([` + taskAgingMarker + `]+)(.*)$`)

	// metadataPattern matches our auto-generated metadata in task descriptions
	// Matches: "[auto: lastUpdated=2023-12-25T10:30:00Z]".
	metadataPattern = regexp.MustCompile(`\[auto: lastUpdated=([^\]]+)\]`)
	// contentStartRegex matches the optional number at the start of content.
	contentStartRegex = regexp.MustCompile(`^(\d*)(.*)$`)

	// Shared HTTP client for better performance.
	httpClient = &http.Client{ //nolint:gochecknoglobals // HTTP client should be shared for connection pooling
		Timeout: time.Second * httpTimeoutSeconds,
		Transport: &http.Transport{
			MaxIdleConns:        maxIdleConnections,
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
			IdleConnTimeout:     idleConnTimeoutSeconds * time.Second,
		},
	}
)

// Main function.
func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Print mode info
	if config.DryRun {
		config.Logger.Printf("Running in dry-run mode (no changes will be made)")
	}
	config.Logger.Printf("Starting task processing...")

	// Process tasks
	if processErr := processAllTasks(config); processErr != nil {
		config.Logger.Fatalf("Failed to process tasks: %v", processErr)
	}

	config.Logger.Printf("Task processing completed successfully")
}

// Load configuration from environment variables.
func loadConfig() (*Config, error) {
	// Initialize logger first
	logFile := os.Getenv(envLogFile)
	var logOutput = os.Stdout

	if logFile != "" {
		var err error
		logOutput, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666) //nolint:gosec,golines // Log files need group write permissions
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", logFile, err)
		}
	}

	logger := log.New(logOutput, "[todoscript] ", log.LstdFlags|log.Lshortfile)

	// Load .env file if it exists
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		logger.Printf("Warning: Error loading .env file: %v", err)
	}

	config := &Config{
		APIURL:      defaultAPIURL,
		ActivityURL: defaultActivityURL,
		Logger:      logger,
	}

	// Get API token
	config.TodoistToken = os.Getenv(envTodoistToken)
	if config.TodoistToken == "" {
		return nil, fmt.Errorf("loading configuration failed: missing required environment variable %s",
			envTodoistToken)
	}

	// Parse dry run flag
	dryRunStr := os.Getenv(envDryRun)
	if dryRunStr != "" {
		config.DryRun, err = strconv.ParseBool(dryRunStr)
		if err != nil {
			logger.Printf("Warning: Invalid DRY_RUN value '%s', defaulting to false: %v", dryRunStr, err)
			config.DryRun = false
		}
	}

	// Parse verbose flag
	verboseStr := os.Getenv(envVerbose)
	if verboseStr != "" {
		config.Verbose, err = strconv.ParseBool(verboseStr)
		if err != nil {
			logger.Printf("Warning: Invalid VERBOSE value '%s', defaulting to false: %v", verboseStr, err)
			config.Verbose = false
		}
	}

	// Parse auto age by default flag
	autoAgeByDefaultStr := os.Getenv(envAutoAgeDefault)
	if autoAgeByDefaultStr != "" {
		config.AutoAgeByDefault, err = strconv.ParseBool(autoAgeByDefaultStr)
		if err != nil {
			logger.Printf("Warning: Invalid AUTOAGE_BY_DEFAULT value '%s', defaulting to false: %v",
				autoAgeByDefaultStr, err)
			config.AutoAgeByDefault = false
		}
	}

	// Set timezone
	timezoneName := os.Getenv(envTimezone)
	if timezoneName == "" {
		timezoneName = defaultTimezone // Default to UTC if not specified
	}

	config.Timezone, err = time.LoadLocation(timezoneName)
	if err != nil {
		logger.Printf("Warning: Invalid timezone '%s', defaulting to UTC: %v", timezoneName, err)
		config.Timezone = time.UTC
	}

	return config, nil
}

// ============================================================================
// CONFIGURATION FUNCTIONS
// ============================================================================

// shouldIncrementBasedOnMidnight determines if enough time has passed since the
// last update to increment the parentheses count. It checks if the current time
// has passed midnight in the specified timezone since the last update.
//
// This implements the core aging rule: tasks age once per day at midnight.
// This prevents tasks from aging multiple times if the script runs multiple times per day.
func shouldIncrementBasedOnMidnight(lastUpdated, now time.Time, tz *time.Location) bool {
	// Convert last update to configured timezone
	lastUpdatedInTZ := lastUpdated.In(tz)

	// Calculate the next midnight after last update
	nextMidnight := time.Date(
		lastUpdatedInTZ.Year(), lastUpdatedInTZ.Month(), lastUpdatedInTZ.Day()+1,
		0, 0, 0, 0, tz,
	)

	// Check if current time has passed that midnight
	nowInTZ := now.In(tz)
	return nowInTZ.After(nextMidnight) || nowInTZ.Equal(nextMidnight)
}

// calculateDaysSinceUpdate calculates the number of days that have passed
// between the lastUpdated time and now in the specified timezone.
// This is used to correctly increment the age markers when the script hasn't run for multiple days.
func calculateDaysSinceUpdate(lastUpdated, now time.Time, tz *time.Location) int {
	// Convert times to the configured timezone
	lastUpdatedInTZ := lastUpdated.In(tz)
	nowInTZ := now.In(tz)

	// Truncate to days to ensure we're counting full days
	lastUpdatedDay := time.Date(
		lastUpdatedInTZ.Year(), lastUpdatedInTZ.Month(), lastUpdatedInTZ.Day(),
		0, 0, 0, 0, tz,
	)

	nowDay := time.Date(
		nowInTZ.Year(), nowInTZ.Month(), nowInTZ.Day(),
		0, 0, 0, 0, tz,
	)

	// Calculate days difference and ensure non-negative
	days := int(nowDay.Sub(lastUpdatedDay).Hours() / 24) //nolint:mnd // hours to days conversion
	return max(0, days)
}

// ============================================================================
// HTTP HELPER FUNCTIONS
// ============================================================================

// makeAuthenticatedRequest handles common HTTP request setup with authentication.
func makeAuthenticatedRequest(config *Config, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for %s %s: %w", method, url, err)
	}

	req.Header.Add("Authorization", "Bearer "+config.TodoistToken)
	if body != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request to %s: %w", url, err)
	}

	return resp, nil
}

// validateHTTPResponse validates that an HTTP response has a success status code.
func validateHTTPResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d (%s)", resp.StatusCode, resp.Status)
	}
	return nil
}

// getTodoistData handles GET requests with JSON decoding.
func getTodoistData(config *Config, url string, target any) error {
	resp, err := makeAuthenticatedRequest(config, "GET", url, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if validateErr := validateHTTPResponse(resp); validateErr != nil {
		return validateErr
	}

	if decodeErr := json.NewDecoder(resp.Body).Decode(target); decodeErr != nil {
		return fmt.Errorf("failed to decode JSON response from %s: %w", url, decodeErr)
	}

	return nil
}

// postTodoistData handles POST requests with JSON payload.
func postTodoistData(config *Config, url string, payload any) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize JSON request payload: %w", err)
	}

	resp, err := makeAuthenticatedRequest(config, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if validateErr := validateHTTPResponse(resp); validateErr != nil {
		return validateErr
	}

	return nil
}

// ============================================================================
// TODOIST API FUNCTIONS
// ============================================================================

// isRecurringTask checks if a task is marked as recurring through labels or due date settings.
func isRecurringTask(task Task) bool {
	// Check if the task has the 'recurring' label
	for _, label := range task.Labels {
		if strings.ToLower(label) == labelRecurring {
			return true
		}
	}

	// Check if the task has a recurring due date
	if task.Due != nil && task.Due.Recurring {
		return true
	}

	return false
}

// getDaysSinceCompletion calculates how many days have passed since a recurring task
// was last completed. This is used to reset the age markers for recurring tasks.
//
// For recurring tasks, instead of continuing to age indefinitely, we reset their
// age markers based on how many days have passed since completion. This provides
// a fresh start while still indicating how long it's been since the last completion.
func getDaysSinceCompletion(config *Config, taskID string) (int, error) {
	if config.DryRun {
		config.Logger.Printf("[DRY RUN] Would check activity log for task %s", taskID)
		return -1, nil // -1 indicates dry run mode
	}

	// Create the URL with query parameters for the activity log request
	url := fmt.Sprintf("%s?object_type=item&object_id=%s&event_type=completed&limit=1", config.ActivityURL, taskID)

	// Parse the activity log response
	var activities ActivityResponse
	if err := getTodoistData(config, url, &activities); err != nil {
		return -1, fmt.Errorf("failed to fetch activity log for task %s: %w", taskID, err)
	}

	// Check if we have completion events
	if activities.Count == 0 || len(activities.Events) == 0 {
		return -1, fmt.Errorf("no completion events found for task %s", taskID)
	}

	// Get the most recent completion event
	latestEvent := activities.Events[0]
	config.Logger.Printf("Latest completion for task %s: %s", taskID, latestEvent.EventDate.Format(time.RFC3339))

	// Calculate days since completion
	daysSince := int(time.Since(latestEvent.EventDate).Hours() / 24) //nolint:mnd // hours to days conversion
	return daysSince, nil
}

// getActiveTasks retrieves all active tasks from the Todoist API.
func getActiveTasks(config *Config) ([]Task, error) {
	var tasks []Task
	if err := getTodoistData(config, config.APIURL+apiEndpointTasks, &tasks); err != nil {
		return nil, fmt.Errorf("todoist task retrieval failed: %w", err)
	}

	return tasks, nil
}

// updateTask updates a task's content and description in Todoist.
func updateTask(config *Config, taskID, content, description string) error {
	data := map[string]string{
		"content":     content,
		"description": description,
	}

	url := fmt.Sprintf(config.APIURL+apiEndpointTask, taskID)
	if err := postTodoistData(config, url, data); err != nil {
		return fmt.Errorf("failed to update task %s in Todoist: %w", taskID, err)
	}

	return nil
}

// ============================================================================
// BUSINESS LOGIC FUNCTIONS
// ============================================================================

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

// processTaskBatch handles batch processing with error collection.
func processTaskBatch(config *Config, tasks []Task) error {
	var failures []error
	successCount := 0

	for _, task := range tasks {
		if err := processTask(config, task); err != nil {
			config.Logger.Printf("Failed to process task %s (%s): %v", task.ID, task.Content, err)
			failures = append(failures, fmt.Errorf("task %s: %w", task.ID, err))
		} else {
			successCount++
		}
	}

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

// shouldProcessTask determines if a task should be processed based on auto-aging labels
// and subtask status.
func shouldProcessTask(task Task, config *Config) bool {
	// Skip subtasks (tasks with a parent_id)
	if task.ParentID != nil {
		return false
	}

	hasNoAutoAgeLabel := false
	hasAutoAgeLabel := false

	for _, label := range task.Labels {
		if label == labelNoAutoAge {
			hasNoAutoAgeLabel = true
		}
		if label == labelAutoAge {
			hasAutoAgeLabel = true
		}
	}

	if config.AutoAgeByDefault {
		// If auto-aging is default, process unless explicitly opted out with @no-autoage
		return !hasNoAutoAgeLabel
	}
	// If auto-aging is not default, process only if explicitly opted in with @autoage
	return hasAutoAgeLabel
}

// filterTasksForProcessing filters tasks to only include those marked for auto-aging.
func filterTasksForProcessing(tasks []Task, config *Config) []Task {
	var tasksToProcess []Task
	for _, task := range tasks {
		if shouldProcessTask(task, config) {
			tasksToProcess = append(tasksToProcess, task)
		}
	}
	return tasksToProcess
}

// ============================================================================
// PARSER/UTILITY FUNCTIONS
// ============================================================================

// extractTaskAgingInfo extracts the age count from a task's parentheses markers.
// Example: "))) Do something" → TaskAgeInfo{AgeCount: 3, ContentWithoutAge: " Do something", HasAgeMarkers: true}.
// extractMetadataTimestamp extracts the last updated timestamp from task description metadata.
func extractMetadataTimestamp(description string) time.Time {
	var lastUpdated time.Time

	// Use pre-compiled regex pattern
	matches := metadataPattern.FindStringSubmatch(description)

	if len(matches) == 2 { //nolint:mnd // 2 groups expected from regex (total + captured group)
		// Group 1 contains the timestamp string
		timestampStr := matches[1]
		parsed, err := time.Parse(time.RFC3339, timestampStr)
		if err == nil {
			lastUpdated = parsed
		}
	}

	return lastUpdated
}

// updateDescriptionWithMetadata adds or updates the metadata timestamp in a task description.
func updateDescriptionWithMetadata(description string, lastUpdated time.Time) string {
	metadataStr := fmt.Sprintf(metadataTemplate, lastUpdated.Format(time.RFC3339))

	// If existing metadata found, replace it
	if metadataPattern.MatchString(description) {
		return metadataPattern.ReplaceAllString(description, metadataStr)
	}

	// Otherwise, append metadata to description
	if description == "" {
		return metadataStr
	}
	return description + "\n\n" + metadataStr
}

// extractTaskAgingInfo extracts the age count from a task's parentheses markers.
// Example: "))) Do something" → TaskAgeInfo{AgeCount: 3, ContentWithoutAge: " Do something", HasAgeMarkers: true}.
func extractTaskAgingInfo(content string) TaskAgeInfo {
	// Use pre-compiled regex pattern
	matches := taskAgePattern.FindStringSubmatch(content)

	if len(matches) != 4 { //nolint:mnd // 4 groups expected from regex (total + 3 captured groups)
		return TaskAgeInfo{
			AgeCount:          0,
			ContentWithoutAge: content,
			HasAgeMarkers:     false,
		}
	}

	// Extract components from regex groups
	// Group 1: optional number prefix (no longer used)
	// Group 2: parentheses markers (e.g., ")))" in "))) task")
	// Group 3: remaining content (e.g., " task" in "))) task")
	numberPrefix := matches[1]
	ageMarkers := matches[2]
	taskContent := matches[3]

	ageCount := len(ageMarkers)
	// Preserve spaces in content as expected by tests
	contentWithoutAge := numberPrefix + taskContent

	return TaskAgeInfo{
		AgeCount:          ageCount,
		ContentWithoutAge: contentWithoutAge,
		HasAgeMarkers:     true,
	}
}

// Update task content with new parentheses count
// addAgingMarkersToContent adds the specified number of parentheses age markers to task content.
// Example: addAgingMarkersToContent(" Do something", 4) → "))))) Do something".
func addAgingMarkersToContent(contentWithoutAge string, count int) string {
	// Find the optional number in the content
	matches := contentStartRegex.FindStringSubmatch(contentWithoutAge)

	if len(matches) != 3 { //nolint:mnd // 3 groups expected from regex (total + 2 captured groups)
		// If regex fails, just return the original content
		return contentWithoutAge
	}

	// Extract components from simple regex: "^(\d*)(.*)$"
	// Group 1: optional number prefix (e.g., "3" in "3 Do something")
	// Group 2: remaining content (e.g., " Do something")
	numberPrefix := matches[1]
	remainingContent := matches[2]

	// Create string with the specified number of parentheses
	ageMarkers := strings.Repeat(taskAgingMarker, count)

	// Ensure there's a space between the parentheses and the remaining content
	if count > 0 && !strings.HasPrefix(remainingContent, " ") {
		remainingContent = " " + remainingContent
	}

	return numberPrefix + ageMarkers + remainingContent
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

	// Check if enough time has passed to make changes
	if !shouldIncrementBasedOnMidnight(lastUpdated, now, ctx.Timezone) {
		return result // No change needed
	}

	// Calculate the new age count
	newCount := calculateNewAgeCount(ageInfo.AgeCount, ctx, lastUpdated, now)

	// Build new content
	switch {
	case newCount == 0:
		result.NewContent = strings.TrimSpace(ageInfo.ContentWithoutAge)
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

// processTask processes a single task for aging and updates it if needed.
func processTask(config *Config, task Task) error {
	// Determine task characteristics
	isRecurring := isRecurringTask(task)
	daysSinceCompletion := -1

	// For recurring tasks, check completion status
	if isRecurring {
		var err error
		daysSinceCompletion, err = getDaysSinceCompletion(config, task.ID)
		if err != nil {
			config.Logger.Printf("Warning: Failed to get completion date for recurring task %s: %v", task.ID, err)
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
	updateInfo := calculateTaskUpdate(ctx, time.Now())

	if !updateInfo.ShouldUpdate {
		config.Logger.Printf("No change needed for task %s, skipping update", task.ID)
		return nil
	}

	// Log what we're doing based on the specific action
	logTaskUpdate(config, task, ctx, updateInfo)

	// Handle dry run mode
	if config.DryRun {
		config.Logger.Printf("[DRY RUN] Would update task %s: \"%s\" -> \"%s\"",
			task.ID, task.Content, updateInfo.NewContent)
		return nil
	}

	// Perform the actual update
	return updateTask(config, task.ID, updateInfo.NewContent, updateInfo.NewDescription)
}
