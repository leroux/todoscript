package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Copy the exact types from the main codebase
type Task struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
	ProjectID   string   `json:"project_id"`
	Due         *DueDate `json:"due,omitempty"`
	ParentID    *string  `json:"parent_id"`
}

type DueDate struct {
	Recurring bool   `json:"is_recurring"`
	Date      string `json:"date,omitempty"`
}

type Config struct {
	AutoAgeByDefault bool
}

const (
	labelRecurring = "recurring"
	labelNoAutoAge = "no-autoage"
	labelAutoAge   = "autoage"
)

func main() {
	// Load .env file
	if err := godotenv.Load("../../.env"); err != nil {
		fmt.Printf("Warning: Could not load .env file: %v\n", err)
	}

	token := os.Getenv("TODOIST_TOKEN")
	if token == "" {
		fmt.Println("TODOIST_TOKEN environment variable not set")
		os.Exit(1)
	}

	taskID := "9279333897" // THIS IS WATER task
	
	// Get the specific task
	task, err := getTask(token, taskID)
	if err != nil {
		fmt.Printf("Error getting task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== TASK DEBUG FOR %s ===\n", taskID)
	fmt.Printf("Content: %s\n", task.Content)
	fmt.Printf("Labels: %v\n", task.Labels)
	fmt.Printf("Due: %+v\n", task.Due)
	fmt.Printf("ParentID: %v\n", task.ParentID)
	fmt.Println()

	// Test isRecurringTask logic
	isRecurring := isRecurringTask(*task)
	fmt.Printf("isRecurringTask() result: %v\n", isRecurring)
	
	if task.Due != nil {
		fmt.Printf("  - task.Due.Recurring: %v\n", task.Due.Recurring)
	} else {
		fmt.Printf("  - task.Due is nil\n")
	}
	
	hasRecurringLabel := false
	for _, label := range task.Labels {
		if label == labelRecurring {
			hasRecurringLabel = true
			break
		}
	}
	fmt.Printf("  - has @recurring label: %v\n", hasRecurringLabel)
	fmt.Println()

	// Test shouldProcessTask logic with different config values
	configs := []Config{
		{AutoAgeByDefault: false},
		{AutoAgeByDefault: true},
	}

	for _, config := range configs {
		shouldProcess := shouldProcessTask(*task, &config)
		fmt.Printf("shouldProcessTask() with AutoAgeByDefault=%v: %v\n", 
			config.AutoAgeByDefault, shouldProcess)
		
		// Show the logic breakdown
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
		
		fmt.Printf("  - hasNoAutoAgeLabel: %v\n", hasNoAutoAgeLabel)
		fmt.Printf("  - hasAutoAgeLabel: %v\n", hasAutoAgeLabel)
		fmt.Printf("  - ParentID is nil: %v\n", task.ParentID == nil)
		fmt.Println()
	}
}

func getTask(token, taskID string) (*Task, error) {
	url := fmt.Sprintf("https://api.todoist.com/rest/v2/tasks/%s", taskID)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return &task, nil
}

// Copy exact functions from main codebase
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