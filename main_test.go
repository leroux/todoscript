package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestExtractTaskAgingInfo tests the extractTaskAgingInfo function
func TestExtractTaskAgingInfo(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantAgeCount   int
		wantContent    string
		wantHasMarkers bool
	}{
		{
			name:           "Simple task with markers",
			input:          "2)) Do something",
			wantAgeCount:   2,
			wantContent:    "2 Do something",
			wantHasMarkers: true,
		},
		{
			name:           "Task without markers",
			input:          "Do something important",
			wantAgeCount:   0,
			wantContent:    "Do something important",
			wantHasMarkers: false,
		},
		{
			name:           "Task with many markers",
			input:          "3))))) Complete this",
			wantAgeCount:   5,
			wantContent:    "3 Complete this",
			wantHasMarkers: true,
		},
		{
			name:           "Task with just markers and no number",
			input:          "))) Task",
			wantAgeCount:   3,
			wantContent:    " Task",
			wantHasMarkers: true,
		},
		{
			name:           "Empty task",
			input:          "",
			wantAgeCount:   0,
			wantContent:    "",
			wantHasMarkers: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTaskAgingInfo(tt.input)
			
			if got.AgeCount != tt.wantAgeCount {
				t.Errorf("extractTaskAgingInfo() AgeCount = %v, want %v", got.AgeCount, tt.wantAgeCount)
			}
			
			if got.ContentWithoutAge != tt.wantContent {
				t.Errorf("extractTaskAgingInfo() ContentWithoutAge = %v, want %v", got.ContentWithoutAge, tt.wantContent)
			}
			
			if got.HasAgeMarkers != tt.wantHasMarkers {
				t.Errorf("extractTaskAgingInfo() HasAgeMarkers = %v, want %v", got.HasAgeMarkers, tt.wantHasMarkers)
			}
		})
	}
}

// TestAddAgingMarkersToContent tests the addAgingMarkersToContent function
func TestAddAgingMarkersToContent(t *testing.T) {
	tests := []struct {
		name             string
		contentWithoutAge string
		count            int
		want             string
	}{
		{
			name:             "Add markers to task with number",
			contentWithoutAge: "3 Do something",
			count:            4,
			want:             "3)))) Do something",
		},
		{
			name:             "Add markers to task without number",
			contentWithoutAge: "Do something",
			count:            2,
			want:             "))Do something",
		},
		{
			name:             "Add zero markers",
			contentWithoutAge: "3 Task",
			count:            0,
			want:             "3 Task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addAgingMarkersToContent(tt.contentWithoutAge, tt.count)
			if got != tt.want {
				t.Errorf("addAgingMarkersToContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDecideUpdateAction_Reset tests the reset action of decideUpdateAction
func TestDecideUpdateAction_Reset(t *testing.T) {
	tz, _ := time.LoadLocation("UTC")
	
	// Test case for reset due to recurring task
	ctx := TaskContext{
		IsRecurring:         true,
		DaysSinceCompletion: 3,
		Timezone:            tz,
	}
	
	got := decideUpdateAction(5, ctx, time.Time{})
	
	// Verify reset behavior
	if got.Action != actionReset {
		t.Errorf("Expected action %s, got %s", actionReset, got.Action)
	}
	
	if got.NewCount != 4 { // daysSinceCompletion + 1
		t.Errorf("Expected new count 4, got %d", got.NewCount)
	}
}

// We'll skip testing the increment and skip actions since they depend on 
// shouldIncrementBasedOnMidnight which is already tested separately

// TestShouldIncrementBasedOnMidnight tests the shouldIncrementBasedOnMidnight function
func TestShouldIncrementBasedOnMidnight(t *testing.T) {
	// Set up timezone for consistent test results
	tz, _ := time.LoadLocation("UTC")
	
	// Base time: 2023-01-01 12:00:00 UTC
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, tz)
	
	tests := []struct {
		name        string
		lastUpdated time.Time
		now         time.Time
		timezone    *time.Location
		want        bool
	}{
		{
			name:        "Should increment when past midnight",
			lastUpdated: baseTime,                                       // 2023-01-01 12:00:00
			now:         baseTime.Add(24 * time.Hour),                   // 2023-01-02 12:00:00
			timezone:    tz,
			want:        true,
		},
		{
			name:        "Should not increment when same day",
			lastUpdated: baseTime,                                       // 2023-01-01 12:00:00
			now:         baseTime.Add(6 * time.Hour),                    // 2023-01-01 18:00:00
			timezone:    tz,
			want:        false,
		},
		{
			name:        "Should increment when exactly midnight",
			lastUpdated: baseTime,                                       // 2023-01-01 12:00:00
			now:         time.Date(2023, 1, 2, 0, 0, 0, 0, tz),          // 2023-01-02 00:00:00
			timezone:    tz,
			want:        true,
		},
		{
			name:        "Should increment when multiple days passed",
			lastUpdated: baseTime,                                       // 2023-01-01 12:00:00
			now:         baseTime.Add(72 * time.Hour),                   // 2023-01-04 12:00:00
			timezone:    tz,
			want:        true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldIncrementBasedOnMidnight(tt.lastUpdated, tt.now, tt.timezone)
			if got != tt.want {
				t.Errorf("shouldIncrementBasedOnMidnight() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetActiveTasks tests the getActiveTasks function with a mock server
func TestGetActiveTasks(t *testing.T) {
	// Save original values to restore later
	originalAPIURL := apiURL
	originalAPIToken := apiToken
	defer func() {
		apiURL = originalAPIURL
		apiToken = originalAPIToken
	}()
	
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/tasks" {
			t.Errorf("Expected path %q, got %q", "/tasks", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header %q, got %q", "Bearer test-token", r.Header.Get("Authorization"))
		}
		
		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		mockTasks := []Task{
			{
				ID:       "123",
				Content:  "2)) Test task",
				Description: "[auto: lastUpdated=2023-01-01T10:00:00Z]",
				Labels:   []string{"autoage"},
			},
			{
				ID:       "456",
				Content:  "Regular task",
				Description: "",
				Labels:   []string{},
			},
		}
		json.NewEncoder(w).Encode(mockTasks)
	}))
	defer server.Close()
	
	// Override API URL and token for testing
	apiURL = server.URL
	apiToken = "test-token"
	
	// Call function under test
	tasks, err := getActiveTasks()
	
	// Verify results
	if err != nil {
		t.Errorf("getActiveTasks() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("getActiveTasks() returned %d tasks, expected 2", len(tasks))
	}
	
	// Check first task
	if tasks[0].ID != "123" {
		t.Errorf("First task ID = %v, want %v", tasks[0].ID, "123")
	}
	if tasks[0].Content != "2)) Test task" {
		t.Errorf("First task Content = %v, want %v", tasks[0].Content, "2)) Test task")
	}
	
	// Check second task
	if tasks[1].ID != "456" {
		t.Errorf("Second task ID = %v, want %v", tasks[1].ID, "456")
	}
	if tasks[1].Content != "Regular task" {
		t.Errorf("Second task Content = %v, want %v", tasks[1].Content, "Regular task")
	}
}

// TestValidateHTTPResponse tests the validateHTTPResponse function
func TestValidateHTTPResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "OK status",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "Not Found status",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "Server Error status",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Status:     http.StatusText(tt.statusCode),
			}
			
			err := validateHTTPResponse(resp)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("validateHTTPResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}