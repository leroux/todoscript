package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// ActivityResponse represents the response from Todoist's activity API.
type ActivityResponse struct {
	Count  int `json:"count"`
	Events []struct {
		EventType string    `json:"event_type"`
		EventDate time.Time `json:"event_date"`
		ObjectID  string    `json:"object_id"`
		Extra     struct {
			Content string `json:"content"`
		} `json:"extra_data"`
	} `json:"events"`
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

	// Get task ID from command line args
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <task_id>")
		fmt.Println("Example: go run main.go 1234567890")
		os.Exit(1)
	}

	taskID := os.Args[1]
	activityURL := "https://api.todoist.com/sync/v9/activity/get"

	// Debug specific task activity
	if err := debugTaskActivity(token, activityURL, taskID); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func debugTaskActivity(token, activityURL, taskID string) error {
	// Create the URL with query parameters for the activity log request
	url := fmt.Sprintf("%s?object_type=item&object_id=%s&event_type=completed", activityURL, taskID)

	fmt.Printf("Fetching activity for task %s...\n", taskID)
	fmt.Printf("URL: %s\n\n", url)

	// Make HTTP request
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

	// Read raw response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("Raw response:\n%s\n\n", string(body))

	// Parse JSON response
	var activities ActivityResponse
	if err := json.Unmarshal(body, &activities); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	fmt.Printf("Total completion events: %d\n\n", activities.Count)

	if activities.Count == 0 {
		fmt.Println("No completion events found")
		return nil
	}

	// Print all events with their order and timestamps
	fmt.Println("Completion events (in API response order):")
	fmt.Println("Index | Event Date (UTC)     | Content")
	fmt.Println("------|---------------------|--------")
	
	for i, event := range activities.Events {
		fmt.Printf("%-5d | %s | %s\n", 
			i, 
			event.EventDate.Format("2006-01-02 15:04:05"), 
			event.Extra.Content,
		)
	}

	fmt.Println()

	// Analysis
	if len(activities.Events) >= 2 {
		first := activities.Events[0].EventDate
		last := activities.Events[len(activities.Events)-1].EventDate
		
		fmt.Printf("Analysis:\n")
		fmt.Printf("First event (index 0): %s\n", first.Format("2006-01-02 15:04:05"))
		fmt.Printf("Last event (index %d): %s\n", len(activities.Events)-1, last.Format("2006-01-02 15:04:05"))
		
		if first.After(last) {
			fmt.Printf("✅ Events are ordered from RECENT to OLDEST (newest first)\n")
			fmt.Printf("   Code assumption is CORRECT: activities.Events[0] is most recent\n")
		} else {
			fmt.Printf("❌ Events are ordered from OLDEST to RECENT (oldest first)\n")
			fmt.Printf("   Code assumption is WRONG: activities.Events[0] is oldest, not most recent!\n")
			fmt.Printf("   Should use: activities.Events[len(activities.Events)-1] for most recent\n")
		}
	} else {
		fmt.Println("Only one event found, cannot determine order")
	}

	// Calculate days since most recent completion
	if len(activities.Events) > 0 {
		// Test both assumptions
		fmt.Printf("\nDays since completion calculations:\n")
		
		firstEvent := activities.Events[0]
		lastEvent := activities.Events[len(activities.Events)-1]
		
		daysSinceFirst := int(time.Since(firstEvent.EventDate).Hours() / 24)
		daysSinceLast := int(time.Since(lastEvent.EventDate).Hours() / 24)
		
		fmt.Printf("Days since first event (index 0): %d days\n", daysSinceFirst)
		fmt.Printf("Days since last event (index %d): %d days\n", len(activities.Events)-1, daysSinceLast)
		
		fmt.Printf("\nCurrent code uses: %d days (from index 0)\n", daysSinceFirst)
		if firstEvent.EventDate.After(lastEvent.EventDate) {
			fmt.Printf("This is correct! ✅\n")
		} else {
			fmt.Printf("This is wrong! Should use %d days from index %d ❌\n", daysSinceLast, len(activities.Events)-1)
		}
	}

	return nil
}