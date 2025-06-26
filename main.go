package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
	IsRecurring bool     `json:"is_recurring,omitempty"`
}

// DueDate represents a task's due date information
type DueDate struct {
	Recurring bool   `json:"is_recurring"`
	Date      string `json:"date,omitempty"`
}

// Global variables
var (
	apiToken      string
	apiURL        string = "https://api.todoist.com/rest/v2"
	activityURL   string = "https://api.todoist.com/sync/v9/activity/get"
	dryRun           bool
	verbose          bool
	autoAgeByDefault bool // New global variable
	recentTaskMap    map[string][]Task
	logger           *log.Logger
	timezone         *time.Location
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
		logger.Fatalf("Error loading configuration: %v", err)
	}

	// Print mode info
	if dryRun {
		logger.Println("Running in dry-run mode (no changes will be made)")
	}
	logger.Println("Starting task processing...")

	// Process tasks
	if err := processAllTasks(); err != nil {
		logger.Fatalf("Error processing tasks: %v", err)
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
		return errors.New("missing required environment variable: TODOIST_TOKEN")
	}

	// Parse dry run flag
	dryRunStr := os.Getenv("DRY_RUN")
	dryRun, _ = strconv.ParseBool(dryRunStr) // Defaults to false if not provided

	// Parse verbose flag
	verboseStr := os.Getenv("VERBOSE")
	verbose, _ = strconv.ParseBool(verboseStr) // Defaults to false if not provided

	// Parse auto age by default flag
	autoAgeByDefaultStr := os.Getenv("AUTOAGE_BY_DEFAULT")
	autoAgeByDefault, _ = strconv.ParseBool(autoAgeByDefaultStr) // Defaults to false if not provided

	// Set timezone
	tzName := os.Getenv("TIMEZONE")
	if tzName == "" {
		tzName = "UTC" // Default to UTC if not specified
	}

	var tzErr error
	timezone, tzErr = time.LoadLocation(tzName)
	if tzErr != nil {
		logger.Printf("Warning: Invalid timezone %s, defaulting to UTC: %v", tzName, tzErr)
		timezone = time.UTC
	}

	return nil
}

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

// Get all active tasks from Todoist API
// Check if a task is recurring based on its due date or labels
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

	// Check based on IsRecurring field if available
	if task.IsRecurring {
		return true
	}

	// Otherwise, consider it non-recurring
	return false
}

// Get days since the task was last completed using the Activity Log API
func getDaysSinceCompletion(taskID string) int {
	// Default to -1 if we can't determine the completion date
	if dryRun {
		logger.Printf("[DRY RUN] Would check activity log for task %s", taskID)
		return -1
	}
	
	client := &http.Client{
		Timeout: time.Second * 30,
	}

	// Create the URL with query parameters for the activity log request
	url := fmt.Sprintf("%s?object_type=item&object_id=%s&event_type=completed&limit=1", activityURL, taskID)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Printf("Error creating activity log request: %v", err)
		return -1
	}

	req.Header.Add("Authorization", "Bearer "+apiToken)
	
	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("Error fetching activity log: %v", err)
		return -1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Printf("Failed to get activity log: %s", resp.Status)
		return -1
	}

	// Parse the activity log response
	type ActivityResponse struct {
		Count int `json:"count"`
		Events []struct {
			EventType string    `json:"event_type"`
			EventDate time.Time `json:"event_date"`
		} `json:"events"`
	}

	var activities ActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		logger.Printf("Error decoding activity log response: %v", err)
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
	client := &http.Client{
		Timeout: time.Second * 30,
	}

	req, err := http.NewRequest("GET", apiURL+"/tasks", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+apiToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get tasks: %s", resp.Status)
	}

	var tasks []Task
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}

// Update a task in Todoist
func updateTask(taskID, content, description string) error {
	client := &http.Client{
		Timeout: time.Second * 30,
	}

	data := map[string]string{
		"content":     content,
		"description": description,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/tasks/%s", apiURL, taskID), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+apiToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update task: %s", resp.Status)
	}

	return nil
}

// Process all tasks and update their staleness
func processAllTasks() error {
	// Get all tasks from Todoist
	allTasks, err := getActiveTasks()
	if err != nil {
		return err
	}

	// Filter tasks based on rules
	tasksToProcess := filterTasksForProcessing(allTasks)

	logger.Printf("Found %d tasks to process", len(tasksToProcess))

	// Build task map for completion detection
	buildTaskMap(tasksToProcess)

	// Process each selected task
	for _, task := range tasksToProcess {
		if err := processTask(task); err != nil {
			logger.Printf("Error processing task %s: %v", task.ID, err)
		}
	}

	return nil
}

// shouldProcessTask determines if a task should be processed based on its labels and AUTOAGE_BY_DEFAULT.
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

// filterTasksForProcessing filters tasks based on the new rules.
func filterTasksForProcessing(tasks []Task) []Task {
	var tasksToProcess []Task
	for _, task := range tasks {
		if shouldProcessTask(task) {
			tasksToProcess = append(tasksToProcess, task)
		}
	}
	return tasksToProcess
}

// Build map of tasks by base content for completion detection
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

// Extract parentheses count from task content
func extractParenthesesCount(content string) (int, string, bool) {
	// Regex to find pattern like "50)" or ")" or "))" etc.
	regex := regexp.MustCompile(`^(\d*)(\)+)(.*)$`)
	matches := regex.FindStringSubmatch(content)

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

	// Regex to extract metadata from description
	metadataRegex := regexp.MustCompile(`\[auto: lastUpdated=([^\]]+)\]`)
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
	metadataRegex := regexp.MustCompile(`\[auto: lastUpdated=[^\]]+\]`)
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

// Check if a task was completed recently by looking for matching tasks
func wasTaskCompleted(task Task) bool {
	count, baseContent, found := extractParenthesesCount(task.Content)
	if !found {
		return false
	}

	// Normalize content
	normalizedContent := strings.TrimSpace(baseContent)

	// Look for tasks with the same base content
	matchingTasks, found := recentTaskMap[normalizedContent]
	if !found {
		return false
	}

	// Check if any matching task has fewer parentheses (suggesting it was reset)
	// First, collect all tasks with the same content pattern
	taskVariants := make(map[string][]int) // map[content][]count
	for _, matchingTask := range matchingTasks {
		// Skip self-comparison
		if matchingTask.ID == task.ID {
			continue
		}

		matchCount, _, matchFound := extractParenthesesCount(matchingTask.Content)
		if matchFound {
			// Store all parentheses counts for this content pattern
			taskVariants[normalizedContent] = append(taskVariants[normalizedContent], matchCount)
		}
	}

	// Look for evidence of task reset (a version with fewer parentheses)
	for _, counts := range taskVariants {
		for _, matchCount := range counts {
			if matchCount < count {
				// Found a version with fewer parentheses - task was completed and reset
				logger.Printf("Task %s appears to be reset: found version with %d parentheses vs current %d",
					task.ID, matchCount, count)
				return true
			}
		}
	}

	return false
}

// Process a single task and update its staleness
func processTask(task Task) error {
	// 1. Extract parentheses count, skip if can't extract
	count, baseContent, found := extractParenthesesCount(task.Content)

	if !found {
		// No parentheses pattern found, not applicable for staleness tracking
		logger.Printf("Skipping task %s (no parentheses pattern found)", task.ID)
		return nil
	}

	// Check if this is a recurring task
	isRecurring := isRecurringTask(task)
	
	// For recurring tasks, first check if they've been completed recently
	var shouldReset bool = false
	var newParenCount int = 1
	if isRecurring {
		// Check completion status from activity log
		daysSinceCompletion := getDaysSinceCompletion(task.ID)
		if daysSinceCompletion >= 0 {
			// Task was completed recently, should reset
			shouldReset = true
			newParenCount = daysSinceCompletion + 1
			logger.Printf("Task %s was completed %d days ago, will reset parentheses count to %d.", 
				task.ID, daysSinceCompletion, newParenCount)
		}
	}

	// Always reset if necessary, regardless of last update time
	if shouldReset {
		// Reset to number of days since completion + 1 for completed recurring tasks
		newContent := updateContentWithParentheses(baseContent, newParenCount)
		
		// Check if the content actually changed
		if newContent == task.Content {
			logger.Printf("No change needed for task %s, skipping update", task.ID)
			return nil
		}
		
		now := time.Now().In(timezone)
		newDescription := updateDescriptionWithMetadata(task.Description, now)
		
		logger.Printf("Resetting task %s: setting parentheses count to %d", task.ID, newParenCount)
		
		if dryRun {
			logger.Printf("[DRY RUN] Would update task %s: \"%s\" -> \"%s\"", task.ID, task.Content, newContent)
			return nil
		}
		
		// Update the task in Todoist
		logger.Printf("Updating task %s: \"%s\" -> \"%s\"", task.ID, task.Content, newContent)
		return updateTask(task.ID, newContent, newDescription)
	}
	
	// For increments (non-resets), apply the midnight alignment rule
	lastUpdated := parseMetadata(task.Description)
	if !lastUpdated.IsZero() {
		now := time.Now()
		if !shouldIncrementBasedOnMidnight(lastUpdated, now, timezone) {
			logger.Printf("Skipping increment for task %s (midnight in configured timezone not reached)", task.ID)
			return nil
		}
	}
	
	// If we reach here, we need to increment the parentheses count
	var newCount int = count + 1
	if isRecurring {
		logger.Printf("Task %s (recurring) not completed recently. Incrementing parentheses count to %d.", 
			task.ID, newCount)
	} else {
		logger.Printf("Task %s is non-recurring. Incrementing parentheses count to %d.", 
			task.ID, newCount)
	}
	
	// Update the task content with the new parentheses count
	newContent := updateContentWithParentheses(baseContent, newCount)
	
	// Check if the content actually changed
	if newContent == task.Content {
		logger.Printf("No change needed for task %s, skipping update", task.ID)
		return nil
	}
	
	// Update metadata with current time in the configured timezone
	now := time.Now().In(timezone)
	newDescription := updateDescriptionWithMetadata(task.Description, now)
	
	if dryRun {
		logger.Printf("[DRY RUN] Would update task %s: \"%s\" -> \"%s\"", task.ID, task.Content, newContent)
		return nil
	}

	// Update the task in Todoist
	logger.Printf("Updating task %s: \"%s\" -> \"%s\"", task.ID, task.Content, newContent)
	
	return updateTask(task.ID, newContent, newDescription)
}
