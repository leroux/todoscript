# Todoscript Specification

This document provides a comprehensive specification of the todoscript application based on the current implementation. It serves as the definitive reference for understanding the exact behavior, rules, and functionality.

## Overview

Todoscript is a single-file Go application that automatically manages task staleness in Todoist by incrementing parentheses in task names daily and resetting them when recurring tasks are completed.

## Configuration

### Environment Variables

| Variable | Required | Type | Default | Description |
|----------|----------|------|---------|-------------|
| `TODOIST_TOKEN` | Yes | String | - | Bearer token for Todoist API authentication |
| `LOG_FILE` | No | String | stdout | Path to log file; uses stdout if not specified |
| `DRY_RUN` | No | Boolean | false | Enable dry-run mode (no actual API updates) |
| `VERBOSE` | No | Boolean | false | Enable verbose logging (parsed but unused) |
| `AUTOAGE_BY_DEFAULT` | No | Boolean | false | Changes default task processing behavior |
| `TIMEZONE` | No | String | UTC | Timezone for timestamp calculations |

### Configuration Loading Behavior

1. Attempts to load `.env` file using `godotenv.Load()`
2. Non-existent .env file errors are ignored
3. Other .env loading errors generate warnings but don't halt execution
4. Invalid boolean values default to `false` with warnings logged
5. Invalid timezone defaults to UTC with warning
6. Missing `TODOIST_TOKEN` causes fatal error and program termination

### Global State

```go
// API Configuration
apiToken: string              // Todoist API token
apiURL: "https://api.todoist.com/rest/v2"
activityURL: "https://api.todoist.com/sync/v9/activity/get"

// Runtime Configuration
dryRun: bool                  // Dry-run mode flag
verbose: bool                 // Verbose logging flag (unused)
autoAgeByDefault: bool        // Default processing behavior
timezone: *time.Location      // Timezone for calculations

// Runtime State
recentTaskMap: map[string][]Task  // Tasks grouped by content
logger: *log.Logger           // Configured logger instance

// Pre-compiled Regex Patterns
parenthesesRegex: `^(\d*)(\)+)(.*)$`
metadataRegex: `\[auto: lastUpdated=([^\]]+)\]`
```

## Data Structures

### Task
```go
type Task struct {
    ID          string   `json:"id"`           // Unique task identifier
    Content     string   `json:"content"`      // Task title/content
    Description string   `json:"description"`  // Task description
    Labels      []string `json:"labels"`       // Array of label strings
    IsCompleted bool     `json:"is_completed"` // Completion status
    Due         *DueDate `json:"due,omitempty"` // Due date information
    IsRecurring bool     `json:"is_recurring,omitempty"` // Recurring flag
}
```

### DueDate
```go
type DueDate struct {
    Recurring bool   `json:"is_recurring"`  // Whether due date is recurring
    Date      string `json:"date,omitempty"` // Due date string
}
```

## API Integration

### Todoist REST API v2

#### GET /tasks
- **URL**: `https://api.todoist.com/rest/v2/tasks`
- **Headers**: `Authorization: Bearer <token>`
- **Purpose**: Fetch all active tasks
- **Response**: Array of Task objects

#### POST /tasks/{taskId}
- **URL**: `https://api.todoist.com/rest/v2/tasks/{taskId}`
- **Headers**: 
  - `Authorization: Bearer <token>`
  - `Content-Type: application/json`
- **Body**: JSON with `content` and `description` fields
- **Purpose**: Update specific task content and description

### Todoist Sync API v9

#### GET /activity/get
- **URL**: `https://api.todoist.com/sync/v9/activity/get`
- **Query Parameters**:
  - `object_type=item`
  - `object_id={taskId}`
  - `event_type=completed`
  - `limit=1`
- **Headers**: `Authorization: Bearer <token>`
- **Purpose**: Fetch most recent completion event for a task
- **Response**: ActivityResponse with completion events

### HTTP Client Configuration
- **Timeout**: 30 seconds
- **Max Idle Connections**: 10
- **Max Idle Connections Per Host**: 2
- **Idle Connection Timeout**: 90 seconds

## Business Logic Rules

### Task Selection Logic

The application processes tasks based on labels and the `AUTOAGE_BY_DEFAULT` setting:

- **When `AUTOAGE_BY_DEFAULT` = true**: Process all tasks EXCEPT those with "no-autoage" label
- **When `AUTOAGE_BY_DEFAULT` = false**: Process ONLY tasks with "autoage" label

### Parentheses Pattern Detection

Tasks must match the regex pattern `^(\d*)(\)+)(.*)$`:
- Optional leading number
- One or more closing parentheses `)`
- Remaining content

Examples:
- `10) Task name` ✓
- `)) Task name` ✓
- `25))) Task name` ✓
- `Task name` ✗ (no parentheses)

### Recurring Task Detection

A task is considered recurring if ANY of these conditions are true:
1. Has "recurring" label (case-insensitive)
2. `Due.Recurring` field is true
3. `IsRecurring` field is true

### Task Action Determination

For each task, the system determines one of three actions:

1. **Reset**: If recurring AND completion detected
   - New count = `days_since_completion + 1`
   - Completion detected via Activity Log API

2. **Skip**: If last update was same day (midnight rule)
   - Uses timezone-aware midnight calculation
   - Prevents multiple updates per day

3. **Increment**: Default action
   - Increases parentheses count by 1

### Midnight Rule Implementation

```go
func shouldIncrementBasedOnMidnight(lastUpdated, now time.Time, tz *time.Location) bool {
    lastUpdatedInTZ := lastUpdated.In(tz)
    nextMidnight := time.Date(
        lastUpdatedInTZ.Year(), lastUpdatedInTZ.Month(), lastUpdatedInTZ.Day()+1,
        0, 0, 0, 0, tz,
    )
    nowInTZ := now.In(tz)
    return nowInTZ.After(nextMidnight) || nowInTZ.Equal(nextMidnight)
}
```

### Content Transformation

#### Parentheses Extraction
- Regex: `^(\d*)(\)+)(.*)$`
- Groups: [full_match, number, parentheses, remaining_content]
- Count = length of parentheses group
- Base content = number + remaining_content

#### Content Update
- Preserves optional leading number
- Rebuilds with new parentheses count
- Format: `{number}{parentheses}{remaining_content}`

#### Metadata Management
- Stored in task description as: `[auto: lastUpdated=2023-01-01T12:00:00Z]`
- Uses RFC3339 timestamp format
- Replaces existing metadata or appends if none exists

## Program Flow

### Main Execution Sequence

1. **Initialization**
   - Initialize `recentTaskMap = make(map[string][]Task)`
   - Setup logger with prefix `[todoscript]`
   - Load configuration from environment
   - Log mode information (dry-run status)

2. **Task Processing Pipeline**
   - Call `processAllTasks()`
   - Fetch all active tasks via `getActiveTasks()`
   - Filter tasks with `filterTasksForProcessing()`
   - Build task map with `buildTaskMap()`
   - Process each task individually with `processTask()`

3. **Individual Task Processing**
   - Check parentheses pattern with `extractParenthesesCount()`
   - Determine if recurring with `isRecurringTask()`
   - Get completion status with `getDaysSinceCompletion()` (recurring only)
   - Apply business logic with `processTaskLogic()`
   - Update task with `updateTask()` (unless dry-run)

4. **Cleanup and Reporting**
   - Clear `recentTaskMap` to free memory
   - Log success/failure counts
   - Return aggregated error status

### Error Handling Strategy

#### Fatal Errors (Program Termination)
- Log file opening failure
- Missing `TODOIST_TOKEN`
- Overall task processing failure from `processAllTasks()`

#### Non-Fatal Errors (Logged, Execution Continues)
- Individual task processing failures
- API request failures for activity log
- Invalid configuration values (with fallback defaults)
- JSON parsing errors

#### Error Propagation Pattern
- Individual task failures are collected in `failures []error`
- Processing continues for remaining tasks
- Final error report includes failure count and first error details
- HTTP errors are wrapped with context using `fmt.Errorf("context: %w", err)`

## Pure Functions

The application includes several pure functions for testability:

### `extractParenthesesCount(content string) (int, string, bool)`
- Extracts parentheses count and base content
- Returns: count, base_content, found
- No side effects

### `updateContentWithParentheses(baseContent string, count int) string`
- Rebuilds content with new parentheses count
- Preserves optional leading number
- No side effects

### `shouldIncrementBasedOnMidnight(lastUpdated, now time.Time, tz *time.Location) bool`
- Determines if enough time has passed for increment
- Timezone-aware midnight calculation
- No side effects

### `determineTaskAction(task, count, isRecurring, daysSinceCompletion, lastUpdated, timezone) (string, int)`
- Determines action: "reset", "skip", or "increment"
- Returns action and new count
- No side effects

### `processTaskLogic(task, isRecurring, daysSinceCompletion, timezone) (string, string, bool)`
- Complete business logic without I/O
- Returns: newContent, newDescription, shouldUpdate
- No side effects

## Logging Specification

### Logger Configuration
- **Prefix**: `[todoscript]`
- **Flags**: `log.LstdFlags|log.Lshortfile` (timestamp + source file)
- **Output**: Configurable via `LOG_FILE`, defaults to stdout

### Log Message Categories

#### Informational
- Program start/completion
- Task processing counts
- Mode announcements (dry-run)

#### Debug/Verbose
- Individual task actions
- API response details
- Dry-run simulations with `[DRY RUN]` prefix

#### Warnings
- Configuration parsing errors
- API request failures
- Invalid values with fallback defaults

#### Errors
- Individual task processing failures
- HTTP request failures with context

#### Fatal
- Critical errors causing program termination

### Dry-Run Behavior
- All log messages prefixed with `[DRY RUN]`
- API calls return default values (-1 for activity log)
- No actual HTTP POST requests sent
- All business logic executed except final update

## State Management

### Task Map (`recentTaskMap`)
- **Purpose**: Groups tasks by normalized content for duplicate detection
- **Key**: Trimmed base content (without parentheses)
- **Value**: Array of Task objects
- **Lifecycle**: Built before processing, cleared after completion

### Memory Management
- Task map is explicitly cleared: `for k := range recentTaskMap { delete(recentTaskMap, k) }`
- HTTP client reuses connections via connection pooling
- No memory leaks in normal operation

## Side Effects

### External API Calls
- **GET requests**: Fetch tasks and activity logs
- **POST requests**: Update task content and description
- **Network timeouts**: 30 seconds per request

### File System
- Log file creation/appending (if `LOG_FILE` specified)
- .env file reading (if present)

### Time Dependencies
- Midnight calculations use configured timezone
- Metadata timestamps use current time in timezone
- Activity log dates parsed as UTC, converted for calculations

## Implementation Constants

### API URLs
- REST API: `https://api.todoist.com/rest/v2`
- Sync API: `https://api.todoist.com/sync/v9/activity/get`

### Timeouts
- HTTP client: 30 seconds
- Connection idle: 90 seconds

### Regex Patterns
- Parentheses: `^(\d*)(\)+)(.*)$`
- Metadata: `\[auto: lastUpdated=([^\]]+)\]`

### Default Values
- Timezone: UTC
- Boolean flags: false
- Log output: stdout

## Error Scenarios

### Network Failures
- Connection timeouts return wrapped errors
- API rate limits logged as warnings
- DNS resolution failures terminate processing

### Invalid Data
- Malformed JSON responses logged as errors
- Invalid task IDs skip processing
- Missing required fields use zero values

### Configuration Issues
- Missing .env file ignored
- Invalid boolean values default to false
- Invalid timezone defaults to UTC
- Missing API token causes fatal error

This specification captures the complete behavior of the current todoscript implementation and can serve as the basis for any refactoring or reimplementation efforts.