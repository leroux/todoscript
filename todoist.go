package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

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
		return nil, fmt.Errorf("HTTP request to %s failed: %w", url, err)
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

// isRecurringTask checks if a task is marked as recurring.
func isRecurringTask(task Task) bool {
	// Check if the task has the 'recurring' label
	for _, label := range task.Labels {
		if label == labelRecurring {
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
// was last completed.
//
// For recurring tasks, instead of continuing to age indefinitely, we reset their
// age markers based on how many days have passed since completion. This provides
// a more accurate representation of task staleness for recurring items.
func getDaysSinceCompletion(config *Config, taskID string) (int, error) {
	// Create the URL with query parameters for the activity log request
	url := fmt.Sprintf("%s?object_type=item&object_id=%s&event_type=completed", config.ActivityURL, taskID)

	// Parse the activity log response
	var activities ActivityResponse
	if err := getTodoistData(config, url, &activities); err != nil {
		return -1, fmt.Errorf("failed to get task completion history: %w", err)
	}

	// Check if we have completion events
	if activities.Count == 0 {
		config.Logger.Printf("No completion events found for recurring task %s", taskID)
		return -1, nil
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
