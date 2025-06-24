# Todoscript - Specification

## Overview
Todoscript is a Go program that interacts with the Todoist API to automatically manage tasks. Its primary purpose is to track the staleness/age of tasks marked with @auto by incrementing parentheses daily and to reset parentheses on recurring tasks when they are completed. The script strictly operates ONLY on tasks tagged with @auto.

## Core Requirements

### Task Identification and Filtering
1. Process ONLY tasks tagged with @auto - this is the fundamental requirement
2. Among the @auto tasks, identify recurring tasks solely based on Due.Recurring = true

### Staleness/Age Tracking
1. For tasks with a pattern like "50)" (number followed by parentheses):
   - Add an additional parenthesis daily to track task staleness/age
   - For example: "50)" → "50))" → "50)))" → etc.

### Recurring Task Handling
1. For tasks with Due.Recurring = true, treat them as recurring tasks
2. Track staleness of recurring tasks just like non-recurring tasks - by incrementing parentheses daily
3. Reset to a single parenthesis ONLY when the task is manually marked as completed by the user
4. For recurring tasks, we track how many days pass without completing the recurring task

#### Example Scenario

Let's follow a daily recurring task "50) Meditate" that was created on Monday:

- **Monday (Day X)**: Task appears as "50) Meditate" with one parenthesis
- **Tuesday (X+1)**: User doesn't complete the task, automation runs → "50)) Meditate" (2 parentheses)
- **Wednesday (X+2)**: Still not completed, automation runs → "50))) Meditate" (3 parentheses)
- **Thursday (X+3)**: Still not completed, automation runs → "50)))) Meditate" (4 parentheses)
- **Friday (X+4)**: User completes the task!
  - Todoist creates a new instance of this recurring task
  - Our automation detects completion and resets the parenthesis count
  - Task is now back to "50) Meditate" (1 parenthesis)
- **Saturday (X+5)**: If not completed, the cycle begins again → "50)) Meditate"

### Metadata Management
1. Store task automation metadata in the task description using a simple format
2. Track last automation run time in metadata
3. Parse metadata like: [auto: lastUpdated=<timestamp>]
4. Update metadata with each automation run

### Task Processing Rules
1. Skip processing tasks until midnight has passed in the configured timezone since last update
2. For both recurring and non-recurring tasks: Increment parentheses daily (when midnight has passed since last update) to track staleness
3. Reset to a single parenthesis ONLY when the task is completed by the user
4. This applies the same staleness tracking to all @auto tasks

### Runtime Options
1. Support a dry-run mode to preview changes without modifying tasks
2. Support a verbose mode for detailed logging
3. Configuration via environment variables (.env file)
4. Support timezone configuration for midnight alignment


## Data Models

### Task
- ID (string)
- Content (string, the task name)
- Description (string, contains metadata)
- Labels ([]string)
- IsCompleted (boolean)
- Due (optional: contains Recurring boolean and DueDate string)

## Processing Flow
1. Load configuration and authenticate with Todoist API
2. Fetch all tasks
3. Filter to only tasks with @auto tag or label
4. For each @auto task:
   - Extract parentheses count from task content
   - Check if the task is recurring (via Due.Recurring, labels, or IsRecurring flag)
   - For recurring tasks, check completion status using Todoist Activity Log API
   - If a recurring task was completed recently: Reset parentheses to a single parenthesis
   - For tasks that don't need to be reset and last updated >24 hours ago: Increment parentheses
   - If last updated <24 hours ago and not needing reset: Skip processing
   - Update task metadata
5. Log actions taken and summary

## Implementation Details
1. Completion detection:
   - Primary method: Check Todoist activity log for completion events via the Activity Log API
   - Fallback method: Match tasks by content (ignoring parentheses count) to connect instances
2. Handling of recurring tasks:
   - The 24-hour update limit only applies to increments, not to resets
   - Task completion checks are performed regardless of last update time
   - Parentheses counts are reset to 1 when a task is detected as completed
3. Logging:
   - Standard Go logger is used with configurable output (stdout or file)
   - Detailed logging in verbose mode
   - Dry-run mode for previewing changes without modifying tasks