package main

import (
	"log" //nolint:depguard // Legacy log usage required for compatibility
	"net/http"
	"regexp"
	"time"
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
	defaultAPIURL      = "https://api.todoist.com/api/v1"
	defaultActivityURL = "https://api.todoist.com/api/v1/activities"

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
	ProjectID   string   `json:"project_id"`
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
	DaysSinceCompletion int // -1 indicates failure to get completion data
	Timezone            *time.Location
}

// PaginatedResponse represents Todoist's cursor-based list response shape.
type PaginatedResponse[T any] struct {
	Results    []T     `json:"results"`
	NextCursor *string `json:"next_cursor"`
}

// ActivityEvent represents a single Todoist activity log event.
type ActivityEvent struct {
	EventType string    `json:"event_type"`
	EventDate time.Time `json:"event_date"`
}

// ActivityResponse represents the response from Todoist's activity API.
type ActivityResponse = PaginatedResponse[ActivityEvent]

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
			IdleConnTimeout:     time.Second * idleConnTimeoutSeconds,
		},
	}
)
