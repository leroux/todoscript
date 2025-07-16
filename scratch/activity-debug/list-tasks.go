package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Task represents a Todoist task.
type Task struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
	ProjectID   string   `json:"project_id"`
	Due         *DueDate `json:"due,omitempty"`
	ParentID    *string  `json:"parent_id"`
}

// DueDate represents a task's due date information.
type DueDate struct {
	Recurring bool   `json:"is_recurring"`
	Date      string `json:"date,omitempty"`
}

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

	// List all tasks
	if err := listTasks(token); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func listTasks(token string) error {
	url := "https://api.todoist.com/rest/v2/tasks"

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var tasks []Task
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	fmt.Printf("Found %d tasks\n\n", len(tasks))

	// Look specifically for tasks containing "water" or similar
	fmt.Println("Tasks containing 'water' (case insensitive):")
	waterTasks := []Task{}
	for _, task := range tasks {
		if strings.Contains(strings.ToLower(task.Content), "water") {
			waterTasks = append(waterTasks, task)
		}
	}

	if len(waterTasks) == 0 {
		fmt.Println("No tasks found containing 'water'")
		fmt.Println("\nLet me show recurring tasks instead:")
		
		// Show recurring tasks
		for _, task := range tasks {
			isRecurring := false
			
			// Check if task has recurring label
			for _, label := range task.Labels {
				if label == "recurring" {
					isRecurring = true
					break
				}
			}
			
			// Check if task has recurring due date
			if task.Due != nil && task.Due.Recurring {
				isRecurring = true
			}
			
			if isRecurring {
				fmt.Printf("ID: %s | Content: %s | Labels: %v\n", 
					task.ID, task.Content, task.Labels)
			}
		}
	} else {
		for _, task := range waterTasks {
			fmt.Printf("ID: %s | Content: %s | Labels: %v | Recurring: %v\n", 
				task.ID, task.Content, task.Labels, 
				task.Due != nil && task.Due.Recurring)
		}
	}

	return nil
}