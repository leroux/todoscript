package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
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

// Global variables
var (
	apiToken         string
	apiURL           string = "https://api.todoist.com/rest/v2"
	activityURL      string = "https://api.todoist.com/sync/v9/activity/get"
	dryRun           bool
	verbose          bool
	autoAgeByDefault bool
	recentTaskMap    map[string][]Task
	logger           *log.Logger
	timezone         *time.Location
	// Pre-compiled regex patterns
	parenthesesRegex = regexp.MustCompile(`^(\d*)(\)+)(.*)$`)
	metadataRegex    = regexp.MustCompile(`\[auto: lastUpdated=([^\]]+)\]`)
	// Shared HTTP client for better performance
	httpClient = &http.Client{
		Timeout: time.Second * 30,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     90 * time.Second,
		},
	}
)

// Main function
func main() {
	// Initialize task map
	recentTaskMap = make(map[string][]Task)

	// Initialize logger
	// Check if log file is specified
	logFile := os.Getenv("LOG_FILE")
	var logOutput *os.File = os.Stdout

	if logFile != "" {
		var err error
		logOutput, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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
		logger.Println("Running in dry-run mode (no changes will be made)")
	}
	logger.Println("Starting task processing...")

	// Process tasks
	if err := processAllTasks(); err != nil {
		logger.Fatalf("Failed to process tasks: %v", err)
	}

	logger.Println("Task processing completed successfully")
}

// Load configuration from environment variables
func loadConfig() error {
	// Load .env file if it exists
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		logger.Printf("Warning: Error loading .env file: %v", err)
	}

	// Get API token
	apiToken = os.Getenv("TODOIST_TOKEN")
	if apiToken == "" {
		return fmt.Errorf("missing required environment variable: TODOIST_TOKEN")
	}

	// Parse dry run flag
	dryRunStr := os.Getenv("DRY_RUN")
	if dryRunStr != "" {
		dryRun, err = strconv.ParseBool(dryRunStr)
		if err != nil {
			logger.Printf("Warning: Invalid DRY_RUN value '%s', defaulting to false: %v", dryRunStr, err)
			dryRun = false
		}
	}

	// Parse verbose flag
	verboseStr := os.Getenv("VERBOSE")
	if verboseStr != "" {
		verbose, err = strconv.ParseBool(verboseStr)
		if err != nil {
			logger.Printf("Warning: Invalid VERBOSE value '%s', defaulting to false: %v", verboseStr, err)
			verbose = false
		}
	}

	// Parse auto age by default flag
	autoAgeByDefaultStr := os.Getenv("AUTOAGE_BY_DEFAULT")
	if autoAgeByDefaultStr != "" {
		autoAgeByDefault, err = strconv.ParseBool(autoAgeByDefaultStr)
		if err != nil {
			logger.Printf("Warning: Invalid AUTOAGE_BY_DEFAULT value '%s', defaulting to false: %v", autoAgeByDefaultStr, err)
			autoAgeByDefault = false
		}
	}

	// Set timezone
	tzName := os.Getenv("TIMEZONE")
	if tzName == "" {
		tzName = "UTC" // Default to UTC if not specified
	}

	var tzErr error
	timezone, tzErr = time.LoadLocation(tzName)
	if tzErr != nil {
		logger.Printf("Warning: Invalid timezone '%s', defaulting to UTC: %v", tzName, tzErr)
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
// TODOIST API FUNCTIONS
// ============================================================================

func isRecurringTask(task Task) bool {
	// Check if the task has the 'recurring' label
	for _, label := range task.Labels {
		if strings.ToLower(label) == "recurring" {
			return true
		}
	}

	// Check if the task has a recurring due date
	if task.Due != nil && task.Due.Recurring {
		return true
	}

	return false
}

// Get days since the task was last completed using the Activity Log API
func getDaysSinceCompletion(taskID string) int {
	// Default to -1 if we can't determine the completion date
	if dryRun {
		logger.Printf("[DRY RUN] Would check activity log for task %s", taskID)
		return -1
	}

	// Create the URL with query parameters for the activity log request
	url := fmt.Sprintf("%s?object_type=item&object_id=%s&event_type=completed&limit=1", activityURL, taskID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Printf("Failed to create activity log request for task %s: %v", taskID, err)
		return -1
	}

	req.Header.Add("Authorization", "Bearer "+apiToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Printf("Failed to fetch activity log for task %s: %v", taskID, err)
		return -1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Printf("Failed to get activity log for task %s: API returned status %d: %s", taskID, resp.StatusCode, resp.Status)
		return -1
	}

	// Parse the activity log response
	type ActivityResponse struct {
		Count  int `json:"count"`
		Events []struct {
			EventType string    `json:"event_type"`
			EventDate time.Time `json:"event_date"`
		} `json:"events"`
	}

	var activities ActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		logger.Printf("Failed to decode activity log response for task %s: %v", taskID, err)
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
	daysSince := int(time.Since(latestEvent.EventDate).Hours() / 24)
	return daysSince
}

func getActiveTasks() ([]Task, error) {
	req, err := http.NewRequest("GET", apiURL+"/tasks", nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+apiToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch tasks failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-OK status %d: %s", resp.StatusCode, resp.Status)
	}

	var tasks []Task
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, fmt.Errorf("decode tasks failed: %w", err)
	}

	return tasks, nil
}

// Update a task in Todoist
func updateTask(taskID, content, description string) error {
	data := map[string]string{
		"content":     content,
		"description": description,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal update data failed: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/tasks/%s", apiURL, taskID), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create update request failed: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+apiToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send update request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update task failed: API returned status %d: %s", resp.StatusCode, resp.Status)
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
			failures = append(failures, fmt.Errorf("task %s: %w", task.ID, err))
		} else {
			successCount++
		}
	}

	logger.Printf("Successfully processed %d tasks", successCount)

	// Clear task map to free memory
	for k := range recentTaskMap {
		delete(recentTaskMap, k)
	}

	if len(failures) > 0 {
		logger.Printf("Failed to process %d tasks", len(failures))
		// Return first error but continue processing others
		return fmt.Errorf("process tasks failed: %d out of %d tasks: %w", len(failures), len(tasksToProcess), failures[0])
	}

	return nil
}

func shouldProcessTask(task Task) bool {
	hasNoAutoAgeLabel := false
	hasAutoAgeLabel := false

	for _, label := range task.Labels {
		if label == "no-autoage" {
			hasNoAutoAgeLabel = true
		}
		if label == "autoage" {
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

func filterTasksForProcessing(tasks []Task) []Task {
	var tasksToProcess []Task
	for _, task := range tasks {
		if shouldProcessTask(task) {
			tasksToProcess = append(tasksToProcess, task)
		}
	}
	return tasksToProcess
}

func buildTaskMap(tasks []Task) {
	for _, task := range tasks {
		_, baseContent, found := extractParenthesesCount(task.Content)
		if !found {
			continue
		}

		// Normalize content by trimming spaces
		normalizedContent := strings.TrimSpace(baseContent)
		recentTaskMap[normalizedContent] = append(recentTaskMap[normalizedContent], task)
	}
}

// ============================================================================
// PARSER/UTILITY FUNCTIONS
// ============================================================================

func extractParenthesesCount(content string) (int, string, bool) {
	// Use pre-compiled regex pattern
	matches := parenthesesRegex.FindStringSubmatch(content)

	if len(matches) != 4 {
		return 0, content, false
	}

	// Extract components
	number := matches[1]
	parentheses := matches[2]
	remainingContent := matches[3]

	count := len(parentheses)
	baseContent := number + remainingContent

	return count, baseContent, true
}

// Parse metadata from task description
func parseMetadata(description string) time.Time {
	var lastUpdated time.Time

	// Use pre-compiled regex pattern
	matches := metadataRegex.FindStringSubmatch(description)

	if len(matches) == 2 {
		// Parse last updated timestamp
		parsed, err := time.Parse(time.RFC3339, matches[1])
		if err == nil {
			lastUpdated = parsed
		}
	}

	return lastUpdated
}

// Update description with new metadata
func updateDescriptionWithMetadata(description string, lastUpdated time.Time) string {
	metadataStr := fmt.Sprintf("[auto: lastUpdated=%s]", lastUpdated.Format(time.RFC3339))

	// If existing metadata found, replace it
	if metadataRegex.MatchString(description) {
		return metadataRegex.ReplaceAllString(description, metadataStr)
	}

	// Otherwise, append metadata to description
	if description == "" {
		return metadataStr
	}
	return description + "\n\n" + metadataStr
}

// Update task content with new parentheses count
func updateContentWithParentheses(baseContent string, count int) string {
	// Find the optional number in the content
	regex := regexp.MustCompile(`^(\d*)(.*)$`)
	matches := regex.FindStringSubmatch(baseContent)

	if len(matches) != 3 {
		// If regex fails, just return the original content
		return baseContent
	}

	number := matches[1]
	remainingContent := matches[2]

	// Create string with the specified number of parentheses
	parentheses := strings.Repeat(")", count)

	return number + parentheses + remainingContent
}


// Determine what action to take on a task based on its state
func determineTaskAction(task Task, currentCount int, isRecurring bool, daysSinceCompletion int, lastUpdated time.Time, timezone *time.Location) (action string, newCount int) {
	// Check for reset conditions first
	if isRecurring && daysSinceCompletion >= 0 {
		return "reset", daysSinceCompletion + 1
	}

	// Check if increment is needed based on midnight rule
	if !lastUpdated.IsZero() {
		now := time.Now()
		if !shouldIncrementBasedOnMidnight(lastUpdated, now, timezone) {
			return "skip", currentCount
		}
	}

	// Default action is increment
	return "increment", currentCount + 1
}

// Process task logic without side effects - returns new content, description, and whether to update
func processTaskLogic(task Task, isRecurring bool, daysSinceCompletion int, timezone *time.Location) (newContent string, newDescription string, shouldUpdate bool) {
	// Extract current parentheses count
	count, baseContent, found := extractParenthesesCount(task.Content)
	if !found {
		return task.Content, task.Description, false
	}

	// Parse existing metadata
	lastUpdated := parseMetadata(task.Description)

	// Determine what action to take
	action, newCount := determineTaskAction(task, count, isRecurring, daysSinceCompletion, lastUpdated, timezone)

	if action == "skip" {
		return task.Content, task.Description, false
	}

	// Calculate new content and description
	newContent = updateContentWithParentheses(baseContent, newCount)

	// Only update if content actually changed
	if newContent == task.Content {
		return task.Content, task.Description, false
	}

	// Update metadata with current time
	now := time.Now().In(timezone)
	newDescription = updateDescriptionWithMetadata(task.Description, now)

	return newContent, newDescription, true
}

func processTask(task Task) error {
	// Check if task has parentheses pattern
	_, _, found := extractParenthesesCount(task.Content)
	if !found {
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
	newContent, newDescription, shouldUpdate := processTaskLogic(task, isRecurring, daysSinceCompletion, timezone)

	if !shouldUpdate {
		logger.Printf("No change needed for task %s, skipping update", task.ID)
		return nil
	}

	// Log what we're doing
	if daysSinceCompletion >= 0 {
		logger.Printf("Resetting task %s: \"%s\" -> \"%s\"", task.ID, task.Content, newContent)
	} else {
		logger.Printf("Incrementing task %s: \"%s\" -> \"%s\"", task.ID, task.Content, newContent)
	}

	// Handle dry run mode
	if dryRun {
		logger.Printf("[DRY RUN] Would update task %s: \"%s\" -> \"%s\"", task.ID, task.Content, newContent)
		return nil
	}

	// Perform the actual update
	return updateTask(task.ID, newContent, newDescription)
}
