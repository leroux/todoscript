package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestTaskProcessing is a comprehensive table test covering all main business logic scenarios
func TestTaskProcessing(t *testing.T) {
	// Set up timezone for consistent test results
	tz, _ := time.LoadLocation("UTC")
	
	// Base times for testing
	now := time.Date(2023, 1, 5, 12, 0, 0, 0, tz)
	yesterday := now.Add(-24 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)
	threeDaysAgo := now.Add(-72 * time.Hour)
	
	tests := []struct {
		name                string
		taskContent         string
		taskDescription     string
		isRecurring         bool
		daysSinceCompletion int
		expectedContent     string
		expectedShouldUpdate bool
		description         string
	}{
		// First-time tasks (no parentheses, no metadata)
		{
			name:                "First-time task - adds metadata only",
			taskContent:         "New task without parentheses",
			taskDescription:     "",
			isRecurring:         false,
			daysSinceCompletion: -1,
			expectedContent:     "New task without parentheses",
			expectedShouldUpdate: true,
			description:         "First-time task should get metadata but no content change",
		},
		
		// Tasks ready for first parenthesis
		{
			name:                "Day 2 task - adds first parenthesis",
			taskContent:         "Task ready for first parenthesis",
			taskDescription:     "[auto: lastUpdated=" + yesterday.Format(time.RFC3339) + "]",
			isRecurring:         false,
			daysSinceCompletion: -1,
			expectedContent:     ") Task ready for first parenthesis",
			expectedShouldUpdate: true,
			description:         "Task with metadata but no parentheses should get first parenthesis after midnight",
		},
		
		// Tasks that should increment
		{
			name:                "Increment existing parentheses",
			taskContent:         ")) Existing task",
			taskDescription:     "[auto: lastUpdated=" + yesterday.Format(time.RFC3339) + "]",
			isRecurring:         false,
			daysSinceCompletion: -1,
			expectedContent:     "))) Existing task",
			expectedShouldUpdate: true,
			description:         "Task with parentheses should increment after midnight",
		},
		
		// Tasks that should skip (same day)
		{
			name:                "Skip - already updated today",
			taskContent:         ")) Task updated today",
			taskDescription:     "[auto: lastUpdated=" + now.Format(time.RFC3339) + "]",
			isRecurring:         false,
			daysSinceCompletion: -1,
			expectedContent:     ")) Task updated today",
			expectedShouldUpdate: false,
			description:         "Task already updated today should be skipped",
		},
		
		// Recurring tasks - reset scenarios
		{
			name:                "Recurring task reset - completed today",
			taskContent:         ")))) Recurring task",
			taskDescription:     "[auto: lastUpdated=" + twoDaysAgo.Format(time.RFC3339) + "]",
			isRecurring:         true,
			daysSinceCompletion: 0,
			expectedContent:     "Recurring task",
			expectedShouldUpdate: true,
			description:         "Recurring task completed today should reset to no parentheses",
		},
		
		{
			name:                "Recurring task reset - completed 3 days ago",
			taskContent:         ")) Recurring task",
			taskDescription:     "[auto: lastUpdated=" + twoDaysAgo.Format(time.RFC3339) + "]",
			isRecurring:         true,
			daysSinceCompletion: 3,
			expectedContent:     ")))) Recurring task",
			expectedShouldUpdate: true,
			description:         "Recurring task completed 3 days ago should reset to 4 parentheses",
		},
		
		// Recurring tasks - skip reset when already correct
		{
			name:                "Recurring task skip reset - already correct count",
			taskContent:         ")))) Recurring task",
			taskDescription:     "[auto: lastUpdated=" + twoDaysAgo.Format(time.RFC3339) + "]",
			isRecurring:         true,
			daysSinceCompletion: 3,
			expectedContent:     ")))) Recurring task",
			expectedShouldUpdate: false,
			description:         "Recurring task with correct parentheses count should be skipped",
		},
		
		// Multi-day catch-up scenarios
		{
			name:                "Multi-day increment",
			taskContent:         ") Task",
			taskDescription:     "[auto: lastUpdated=" + threeDaysAgo.Format(time.RFC3339) + "]",
			isRecurring:         false,
			daysSinceCompletion: -1,
			expectedContent:     ")))) Task",
			expectedShouldUpdate: true,
			description:         "Task should increment by 3 for 3 days missed",
		},
		
		// Edge cases
		{
			name:                "Empty task content",
			taskContent:         "",
			taskDescription:     "",
			isRecurring:         false,
			daysSinceCompletion: -1,
			expectedContent:     "",
			expectedShouldUpdate: true,
			description:         "Empty task should still get metadata",
		},
		
		{
			name:                "Task with just parentheses",
			taskContent:         ")))",
			taskDescription:     "[auto: lastUpdated=" + yesterday.Format(time.RFC3339) + "]",
			isRecurring:         false,
			daysSinceCompletion: -1,
			expectedContent:     ")))) ",
			expectedShouldUpdate: true,
			description:         "Task with only parentheses should increment normally",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create task context
			task := Task{
				ID:          "test-id",
				Content:     tt.taskContent,
				Description: tt.taskDescription,
			}
			
			ctx := TaskContext{
				Task:                task,
				IsRecurring:         tt.isRecurring,
				DaysSinceCompletion: tt.daysSinceCompletion,
				Timezone:            tz,
			}
			
			// Test the main business logic
			result := calculateTaskUpdate(ctx, now)
			
			// Verify results
			if result.ShouldUpdate != tt.expectedShouldUpdate {
				t.Errorf("Expected ShouldUpdate=%v, got %v", tt.expectedShouldUpdate, result.ShouldUpdate)
			}
			
			if result.NewContent != tt.expectedContent {
				t.Errorf("Expected content=%q, got %q", tt.expectedContent, result.NewContent)
			}
			
			// Verify metadata is always updated when ShouldUpdate is true
			if result.ShouldUpdate && result.NewDescription == task.Description {
				t.Error("Expected description to be updated when ShouldUpdate is true")
			}
		})
	}
}

// Keep minimal tests for HTTP functionality that requires mocking
func TestHTTPFunctionality(t *testing.T) {
	t.Run("GetActiveTasks", func(t *testing.T) {
		// Mock HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			mockTasks := []Task{
				{
					ID:      "123",
					Content: ")) Test task",
					Labels:  []string{"autoage"},
				},
			}
			json.NewEncoder(w).Encode(mockTasks)
		}))
		defer server.Close()
		
		// Override API URL for testing
		originalURL := apiURL
		apiURL = server.URL
		defer func() { apiURL = originalURL }()
		
		// Test
		tasks, err := getActiveTasks()
		if err != nil {
			t.Errorf("getActiveTasks() error = %v", err)
		}
		if len(tasks) != 1 {
			t.Errorf("Expected 1 task, got %d", len(tasks))
		}
	})
	
	t.Run("ValidateHTTPResponse", func(t *testing.T) {
		tests := []struct {
			name       string
			statusCode int
			wantError  bool
		}{
			{"OK status", http.StatusOK, false},
			{"Not Found", http.StatusNotFound, true},
			{"Server Error", http.StatusInternalServerError, true},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp := &http.Response{StatusCode: tt.statusCode, Status: http.StatusText(tt.statusCode)}
				err := validateHTTPResponse(resp)
				
				if (err != nil) != tt.wantError {
					t.Errorf("validateHTTPResponse() error = %v, wantError %v", err, tt.wantError)
				}
			})
		}
	})
}

// Keep test for task filtering logic
func TestTaskFiltering(t *testing.T) {
	tests := []struct {
		name      string
		task      Task
		autoAge   bool
		wantProcess bool
	}{
		{
			name:        "Regular task with autoage label",
			task:        Task{Labels: []string{"autoage"}, ParentID: nil},
			autoAge:     false,
			wantProcess: true,
		},
		{
			name:        "Subtask should not be processed",
			task:        Task{Labels: []string{"autoage"}, ParentID: stringPtr("parent-id")},
			autoAge:     false,
			wantProcess: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalAutoAge := autoAgeByDefault
			autoAgeByDefault = tt.autoAge
			defer func() { autoAgeByDefault = originalAutoAge }()
			
			result := shouldProcessTask(tt.task)
			if result != tt.wantProcess {
				t.Errorf("shouldProcessTask() = %v, want %v", result, tt.wantProcess)
			}
		})
	}
}

// Helper function for string pointer
func stringPtr(s string) *string {
	return &s
}