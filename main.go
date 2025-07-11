// Package main implements todoscript - a Todoist task aging automation tool.
//
// This tool automatically increments visual "age markers" (parentheses) on Todoist tasks
// to help track how long tasks have been sitting in your todo list. The concept is simple:
// tasks get more parentheses the longer they remain incomplete, creating visual pressure
// to either complete them or remove them.
//
// How it works:
// - Tasks with pattern "2) Do something" become "2)) Do something" after midnight
// - Recurring tasks reset their age when completed: "5))))) Task" → "3))) Task"
// - Tasks can opt-in with @autoage label or opt-out with @no-autoage label
// - Dry-run mode available for testing changes before applying them
//
// The aging concept creates a visual indication of task staleness, encouraging you to
// either complete long-standing tasks or remove them from your list entirely.
package main

import (
	"bytes"
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

// Constants
const (
	// Task aging marker
	taskAgingMarker = ")"

	// HTTP client configuration
	httpTimeoutSeconds     = 30
	maxIdleConnections     = 10
	maxIdleConnsPerHost    = 2
	idleConnTimeoutSeconds = 90

	// File permissions
	logFilePermissions = 0666

	// Time calculations
	hoursPerDay = 24

	// Environment variables
	envLogFile        = "LOG_FILE"
	envTodoistToken   = "TODOIST_TOKEN"
	envDryRun         = "DRY_RUN"
	envVerbose        = "VERBOSE"
	envAutoAgeDefault = "AUTOAGE_BY_DEFAULT"
	envTimezone       = "TIMEZONE"

	// Default values
	defaultTimezone = "UTC"

	// Task labels
	labelRecurring = "recurring"
	labelNoAutoAge = "no-autoage"
	labelAutoAge   = "autoage"

	// Task actions
	actionReset     = "reset"
	actionSkip      = "skip"
	actionIncrement = "increment"

	// HTTP methods
	httpMethodGet  = "GET"
	httpMethodPost = "POST"

	// JSON fields
	jsonFieldContent     = "content"
	jsonFieldDescription = "description"
)

// Task represents a Todoist task
type Task struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
	IsCompleted bool     `json:"is_completed"`
	Due         *DueDate `json:"due,omitempty"`
}

// DueDate represents a task's due date information
type DueDate struct {
	Recurring bool   `json:"is_recurring"`
	Date      string `json:"date,omitempty"`
}

// TaskAgeInfo represents the result of parsing age markers from a task
type TaskAgeInfo struct {
	AgeCount          int    // Number of age markers (parentheses)
	ContentWithoutAge string // Task content with age markers removed
	HasAgeMarkers     bool   // Whether the task has age markers
}

// TaskUpdateInfo represents the result of calculating a task update
type TaskUpdateInfo struct {
	NewContent     string // Updated task content
	NewDescription string // Updated task description
	ShouldUpdate   bool   // Whether the task should be updated
}

// UpdateAction represents the action to take on a task
type UpdateAction struct {
	Action   string // "reset", "skip", or "increment"
	NewCount int    // New age count after action
}

// TaskContext contains all the information needed to process a task
type TaskContext struct {
	Task                Task
	IsRecurring         bool
	DaysSinceCompletion int
	Timezone            *time.Location
}

// ActivityResponse represents the response from Todoist's activity API
type ActivityResponse struct {
	Count  int `json:"count"`
	Events []struct {
		EventType string    `json:"event_type"`
		EventDate time.Time `json:"event_date"`
	} `json:"events"`
}

// Global variables
var (
	apiToken         string
	apiURL           string = "https://api.todoist.com/rest/v2"
	activityURL      string = "https://api.todoist.com/sync/v9/activity/get"
	dryRun           bool
	verbose          bool
	autoAgeByDefault bool
	tasksByContent   map[string][]Task // Maps task content to tasks for duplicate detection
	logger           *log.Logger
	timezone         *time.Location
	// Pre-compiled regex patterns for task aging
	// taskAgePattern matches tasks with age markers: "3))) Do something"
	// Groups: (1) optional number, (2) parentheses markers, (3) remaining content
	taskAgePattern = regexp.MustCompile(`^(\d*)([` + taskAgingMarker + `]+)(.*)$`)

	// metadataPattern matches our auto-generated metadata in task descriptions
	// Matches: "[auto: lastUpdated=2023-12-25T10:30:00Z]"
	metadataPattern = regexp.MustCompile(`\[auto: lastUpdated=([^\]]+)\]`)
	// contentStartRegex matches the optional number at the start of content
	contentStartRegex = regexp.MustCompile(`^(\d*)(.*)$`)

	// Shared HTTP client for better performance
	httpClient = &http.Client{
		Timeout: time.Second * httpTimeoutSeconds,
		Transport: &http.Transport{
			MaxIdleConns:        maxIdleConnections,
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
			IdleConnTimeout:     idleConnTimeoutSeconds * time.Second,
		},
	}
)

// Main function
func main() {
	// Initialize task map
	tasksByContent = make(map[string][]Task)

	// Initialize logger
	// Check if log file is specified
	logFile := os.Getenv(envLogFile)
	var logOutput *os.File = os.Stdout

	if logFile != "" {
		var err error
		logOutput, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFilePermissions)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
	}

	// Create logger
	logger = log.New(logOutput, "[todoscript] ", log.LstdFlags|log.Lshortfile)

	// Load configuration
	err := loadConfig()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Print mode info
	if dryRun {
		logger.Printf("Running in dry-run mode (no changes will be made)")
	}
	logger.Printf("Starting task processing...")

	// Process tasks
	if err := processAllTasks(); err != nil {
		logger.Fatalf("Failed to process tasks: %v", err)
	}

	logger.Printf("Task processing completed successfully")
}

// Load configuration from environment variables
func loadConfig() error {
	// Load .env file if it exists
	var err error
	err = godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		logger.Printf("Warning: Error loading .env file: %v", err)
	}

	// Get API token
	apiToken = os.Getenv(envTodoistToken)
	if apiToken == "" {
		return fmt.Errorf("loading configuration failed: missing required environment variable %s", envTodoistToken)
	}

	// Parse dry run flag
	dryRunStr := os.Getenv(envDryRun)
	if dryRunStr != "" {
		dryRun, err = strconv.ParseBool(dryRunStr)
		if err != nil {
			logger.Printf("Warning: Invalid DRY_RUN value '%s', defaulting to false: %v", dryRunStr, err)
			dryRun = false
		}
	}

	// Parse verbose flag
	verboseStr := os.Getenv(envVerbose)
	if verboseStr != "" {
		verbose, err = strconv.ParseBool(verboseStr)
		if err != nil {
			logger.Printf("Warning: Invalid VERBOSE value '%s', defaulting to false: %v", verboseStr, err)
			verbose = false
		}
	}

	// Parse auto age by default flag
	autoAgeByDefaultStr := os.Getenv(envAutoAgeDefault)
	if autoAgeByDefaultStr != "" {
		autoAgeByDefault, err = strconv.ParseBool(autoAgeByDefaultStr)
		if err != nil {
			logger.Printf("Warning: Invalid AUTOAGE_BY_DEFAULT value '%s', defaulting to false: %v", autoAgeByDefaultStr, err)
			autoAgeByDefault = false
		}
	}

	// Set timezone
	timezoneName := os.Getenv(envTimezone)
	if timezoneName == "" {
		timezoneName = defaultTimezone // Default to UTC if not specified
	}

	timezone, err = time.LoadLocation(timezoneName)
	if err != nil {
		logger.Printf("Warning: Invalid timezone '%s', defaulting to UTC: %v", timezoneName, err)
		timezone = time.UTC
	}

	return nil
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

// ============================================================================
// HTTP HELPER FUNCTIONS
// ============================================================================

// makeAuthenticatedRequest handles common HTTP request setup with authentication
func makeAuthenticatedRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("HTTP request creation failed: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+apiToken)
	if body != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request execution failed: %w", err)
	}

	return resp, nil
}

// validateHTTPResponse validates that an HTTP response has a success status code
func validateHTTPResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed: status %d: %s", resp.StatusCode, resp.Status)
	}
	return nil
}

// getTodoistData handles GET requests with JSON decoding
func getTodoistData(url string, target any) error {
	resp, err := makeAuthenticatedRequest(httpMethodGet, url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := validateHTTPResponse(resp); err != nil {
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("JSON response decoding failed: %w", err)
	}

	return nil
}

// postTodoistData handles POST requests with JSON payload
func postTodoistData(url string, payload any) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON request serialization failed: %w", err)
	}

	resp, err := makeAuthenticatedRequest(httpMethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := validateHTTPResponse(resp); err != nil {
		return err
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
func getDaysSinceCompletion(taskID string) int {
	// Default to -1 if we can't determine the completion date
	if dryRun {
		logger.Printf("[DRY RUN] Would check activity log for task %s", taskID)
		return -1
	}

	// Create the URL with query parameters for the activity log request
	url := fmt.Sprintf("%s?object_type=item&object_id=%s&event_type=completed&limit=1", activityURL, taskID)

	// Parse the activity log response

	var activities ActivityResponse
	if err := getTodoistData(url, &activities); err != nil {
		logger.Printf("Failed to fetch activity log for task %s: %v", taskID, err)
		return -1
	}

	// Check if we have completion events
	if activities.Count == 0 || len(activities.Events) == 0 {
		logger.Printf("No completion events found for task %s", taskID)
		return -1
	}

	// Get the most recent completion event
	latestEvent := activities.Events[0]
	logger.Printf("Latest completion for task %s: %s", taskID, latestEvent.EventDate.Format(time.RFC3339))

	// Calculate days since completion
	daysSince := int(time.Since(latestEvent.EventDate).Hours() / hoursPerDay)
	return daysSince
}

// getActiveTasks retrieves all active tasks from the Todoist API.
func getActiveTasks() ([]Task, error) {
	var tasks []Task
	if err := getTodoistData(apiURL+"/tasks", &tasks); err != nil {
		return nil, fmt.Errorf("Todoist task retrieval failed: %w", err)
	}

	return tasks, nil
}

// updateTask updates a task's content and description in Todoist.
func updateTask(taskID, content, description string) error {
	data := map[string]string{
		jsonFieldContent:     content,
		jsonFieldDescription: description,
	}

	url := fmt.Sprintf("%s/tasks/%s", apiURL, taskID)
	if err := postTodoistData(url, data); err != nil {
		return fmt.Errorf("Todoist task update failed: %w", err)
	}

	return nil
}

// ============================================================================
// BUSINESS LOGIC FUNCTIONS
// ============================================================================

func processAllTasks() error {
	// Get all tasks from Todoist
	allTasks, err := getActiveTasks()
	if err != nil {
		return fmt.Errorf("get tasks failed: %w", err)
	}

	// Filter tasks based on rules
	tasksToProcess := filterTasksForProcessing(allTasks)

	logger.Printf("Found %d tasks to process out of %d total tasks", len(tasksToProcess), len(allTasks))

	// Build task map for completion detection
	buildTaskMap(tasksToProcess)

	// Process each selected task - continue on individual failures
	var failures []error
	successCount := 0

	for _, task := range tasksToProcess {
		if err := processTask(task); err != nil {
			logger.Printf("Failed to process task %s (%s): %v", task.ID, task.Content, err)
			failures = append(failures, fmt.Errorf("task ID %s processing failed: %w", task.ID, err))
		} else {
			successCount++
		}
	}

	logger.Printf("Successfully processed %d tasks", successCount)

	// Clear task map to free memory
	for k := range tasksByContent {
		delete(tasksByContent, k)
	}

	if len(failures) > 0 {
		logger.Printf("Failed to process %d tasks", len(failures))
		// Return first error but continue processing others
		return fmt.Errorf("Task processing failed: %d out of %d tasks failed: %w", len(failures), len(tasksToProcess), failures[0])
	}

	return nil
}

// shouldProcessTask determines if a task should be processed based on auto-aging labels.
func shouldProcessTask(task Task) bool {
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

	if autoAgeByDefault {
		// If auto-aging is default, process unless explicitly opted out with @no-autoage
		return !hasNoAutoAgeLabel
	}
	// If auto-aging is not default, process only if explicitly opted in with @autoage
	return hasAutoAgeLabel
}

// filterTasksForProcessing filters tasks to only include those marked for auto-aging.
func filterTasksForProcessing(tasks []Task) []Task {
	var tasksToProcess []Task
	for _, task := range tasks {
		if shouldProcessTask(task) {
			tasksToProcess = append(tasksToProcess, task)
		}
	}
	return tasksToProcess
}

// buildTaskMap creates a map of task content to tasks for duplicate detection.
func buildTaskMap(tasks []Task) {
	for _, task := range tasks {
		ageInfo := extractTaskAgingInfo(task.Content)
		if !ageInfo.HasAgeMarkers {
			continue
		}

		// Normalize content by trimming spaces
		cleanContent := strings.TrimSpace(ageInfo.ContentWithoutAge)
		tasksByContent[cleanContent] = append(tasksByContent[cleanContent], task)
	}
}

// ============================================================================
// PARSER/UTILITY FUNCTIONS
// ============================================================================

// extractTaskAgingInfo extracts the age count from a task's parentheses markers.
// Example: "3))) Do something" → TaskAgeInfo{AgeCount: 3, ContentWithoutAge: "3 Do something", HasAgeMarkers: true}
// extractMetadataTimestamp extracts the last updated timestamp from task description metadata.
func extractMetadataTimestamp(description string) time.Time {
	var lastUpdated time.Time

	// Use pre-compiled regex pattern
	matches := metadataPattern.FindStringSubmatch(description)

	if len(matches) == 2 {
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
	metadataStr := fmt.Sprintf("[auto: lastUpdated=%s]", lastUpdated.Format(time.RFC3339))

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
// Example: "3))) Do something" → TaskAgeInfo{AgeCount: 3, ContentWithoutAge: "3 Do something", HasAgeMarkers: true}
func extractTaskAgingInfo(content string) TaskAgeInfo {
	// Use pre-compiled regex pattern
	matches := taskAgePattern.FindStringSubmatch(content)

	if len(matches) != 4 {
		return TaskAgeInfo{
			AgeCount:          0,
			ContentWithoutAge: content,
			HasAgeMarkers:     false,
		}
	}

	// Extract components from regex groups
	// Group 1: optional number prefix (e.g., "3" in "3))) task")
	// Group 2: parentheses markers (e.g., ")))" in "3))) task")
	// Group 3: remaining content (e.g., " task" in "3))) task")
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
// Example: addAgingMarkersToContent("3 Do something", 4) → "3)))) Do something"
func addAgingMarkersToContent(contentWithoutAge string, count int) string {
	// Find the optional number in the content
	matches := contentStartRegex.FindStringSubmatch(contentWithoutAge)

	if len(matches) != 3 {
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

// decideUpdateAction determines what action to take on a task based on its current state.
func decideUpdateAction(currentCount int, ctx TaskContext, lastUpdated time.Time) UpdateAction {
	// Check for reset conditions first
	if ctx.IsRecurring && ctx.DaysSinceCompletion >= 0 {
		// For tasks completed today (DaysSinceCompletion == 0), reset completely (no parentheses)
		if ctx.DaysSinceCompletion == 0 {
			return UpdateAction{
				Action:   actionReset,
				NewCount: 0,
			}
		}
		return UpdateAction{
			Action:   actionReset,
			NewCount: ctx.DaysSinceCompletion + 1,
		}
	}

	// Check if increment is needed based on midnight rule
	if !lastUpdated.IsZero() {
		now := time.Now()
		if !shouldIncrementBasedOnMidnight(lastUpdated, now, ctx.Timezone) {
			return UpdateAction{
				Action:   actionSkip,
				NewCount: currentCount,
			}
		}
	}

	// Default action is increment
	return UpdateAction{
		Action:   actionIncrement,
		NewCount: currentCount + 1,
	}
}

// calculateTaskUpdate determines the new content and description for a task based on aging rules.
func calculateTaskUpdate(ctx TaskContext) TaskUpdateInfo {
	// Extract current parentheses count
	ageInfo := extractTaskAgingInfo(ctx.Task.Content)

	// For tasks without age markers, we need to check if this is a first-time task
	// or a task that needs its first marker
	if !ageInfo.HasAgeMarkers {
		// Check if metadata exists
		lastUpdated := extractMetadataTimestamp(ctx.Task.Description)
		if lastUpdated.IsZero() {
			// First time task, add metadata but don't change content
			now := time.Now().In(ctx.Timezone)
			newDescription := updateDescriptionWithMetadata(ctx.Task.Description, now)
			return TaskUpdateInfo{
				NewContent:     ctx.Task.Content,
				NewDescription: newDescription,
				ShouldUpdate:   true,
			}
		} else if shouldIncrementBasedOnMidnight(lastUpdated, time.Now(), ctx.Timezone) {
			// Task has metadata but no markers yet - add first marker
			newContent := ") " + ctx.Task.Content
			now := time.Now().In(ctx.Timezone)
			newDescription := updateDescriptionWithMetadata(ctx.Task.Description, now)
			return TaskUpdateInfo{
				NewContent:     newContent,
				NewDescription: newDescription,
				ShouldUpdate:   true,
			}
		}

		return TaskUpdateInfo{
			NewContent:     ctx.Task.Content,
			NewDescription: ctx.Task.Description,
			ShouldUpdate:   false,
		}
	}

	// Parse existing metadata
	lastUpdated := extractMetadataTimestamp(ctx.Task.Description)

	// Determine what action to take
	updateAction := decideUpdateAction(ageInfo.AgeCount, ctx, lastUpdated)

	if updateAction.Action == actionSkip {
		return TaskUpdateInfo{
			NewContent:     ctx.Task.Content,
			NewDescription: ctx.Task.Description,
			ShouldUpdate:   false,
		}
	}

	// Special handling for reset action (for recurring tasks)
	if updateAction.Action == actionReset && ctx.DaysSinceCompletion == 0 {
		// Complete reset - no parentheses for tasks completed today
		now := time.Now().In(ctx.Timezone)
		newDescription := updateDescriptionWithMetadata(ctx.Task.Description, now)
		return TaskUpdateInfo{
			NewContent:     strings.TrimSpace(ageInfo.ContentWithoutAge), // Strip leading/trailing spaces
			NewDescription: newDescription,
			ShouldUpdate:   true,
		}
	}

	// Calculate new content and description
	newContent := addAgingMarkersToContent(ageInfo.ContentWithoutAge, updateAction.NewCount)

	// Only update if content actually changed
	if newContent == ctx.Task.Content {
		return TaskUpdateInfo{
			NewContent:     ctx.Task.Content,
			NewDescription: ctx.Task.Description,
			ShouldUpdate:   false,
		}
	}

	// Update metadata with current time
	now := time.Now().In(ctx.Timezone)
	newDescription := updateDescriptionWithMetadata(ctx.Task.Description, now)

	return TaskUpdateInfo{
		NewContent:     newContent,
		NewDescription: newDescription,
		ShouldUpdate:   true,
	}
}

// processTask processes a single task for aging and updates it if needed.
func processTask(task Task) error {
	// Check if task has parentheses pattern
	ageInfo := extractTaskAgingInfo(task.Content)
	if !ageInfo.HasAgeMarkers {
		logger.Printf("Skipping task %s (no parentheses pattern found)", task.ID)
		return nil
	}

	// Determine task characteristics
	isRecurring := isRecurringTask(task)
	daysSinceCompletion := -1

	// For recurring tasks, check completion status
	if isRecurring {
		daysSinceCompletion = getDaysSinceCompletion(task.ID)
		if daysSinceCompletion >= 0 {
			logger.Printf("Task %s was completed %d days ago, will reset parentheses count to %d.",
				task.ID, daysSinceCompletion, daysSinceCompletion+1)
		}
	}

	// Use pure function to determine what to do
	ctx := TaskContext{
		Task:                task,
		IsRecurring:         isRecurring,
		DaysSinceCompletion: daysSinceCompletion,
		Timezone:            timezone,
	}
	updateInfo := calculateTaskUpdate(ctx)

	if !updateInfo.ShouldUpdate {
		logger.Printf("No change needed for task %s, skipping update", task.ID)
		return nil
	}

	// Log what we're doing
	if daysSinceCompletion >= 0 {
		logger.Printf("Resetting task %s: \"%s\" -> \"%s\"", task.ID, task.Content, updateInfo.NewContent)
	} else {
		logger.Printf("Incrementing task %s: \"%s\" -> \"%s\"", task.ID, task.Content, updateInfo.NewContent)
	}

	// Handle dry run mode
	if dryRun {
		logger.Printf("[DRY RUN] Would update task %s: \"%s\" -> \"%s\"", task.ID, task.Content, updateInfo.NewContent)
		return nil
	}

	// Perform the actual update
	return updateTask(task.ID, updateInfo.NewContent, updateInfo.NewDescription)
}
