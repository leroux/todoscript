# Todoscript Product Requirements Document

## Overview

Todoscript is a task automation tool that provides visual staleness tracking for Todoist tasks. It automatically increments visual indicators on tasks to show how long they've been pending, and resets these indicators when recurring tasks are completed.

## Core Functionality

### Task Staleness Tracking

**Requirement**: Automatically track and visually indicate how long tasks have been pending.

**Implementation**: 
- Tasks have parentheses as staleness indicators, always at the beginning of the task
- Format: `[parentheses][content]` (e.g., ") Review quarterly reports")
- On Day 1, tasks have 0 parentheses: `Task name`
- Each additional day a task remains incomplete, add one parenthesis: `Task name → ) Task name → )) Task name`
- The parentheses count represents days of staleness minus one (e.g., 0 parentheses = day 1, 1 parenthesis = day 2)

**Task Aging Table**:

| Day | Parentheses Count | Example            |
|-----|------------------|--------------------|
| 1   | 0                | Task name          |
| 2   | 1                | ) Task name        |
| 3   | 2                | )) Task name       |
| 4   | 3                | ))) Task name      |
| 5   | 4                | )))) Task name     |
| 10  | 9                | ))))))))) Task name|

### Recurring Task Reset

**Requirement**: Reset staleness indicators when recurring tasks are completed.

**Implementation**:
- When a recurring task is completed, reset parentheses to reflect days since completion
- If completed today, reset to 0 parentheses (day 1)
- If completed 3 days ago, reset to 3 parentheses (day 4)
- This handles cases where the script runs after task completion

**Recurring Task Reset Example**:

| Completion Status | Days Since Completion | Parentheses Count | Example        |
|------------------|---------------------|------------------|----------------|
| Completed today  | 0                   | 0                | Task name      |
| Completed yesterday | 1                | 1                | ) Task name    |
| Completed 3 days ago | 3               | 3                | ))) Task name  |

### Task Selection

**Requirement**: Allow users to control which tasks are processed.

**Implementation**:
- Provide a configuration option for default behavior
- Support task-level labels to override defaults
- Two modes:
  - Opt-out: Process all tasks except those marked to exclude
  - Opt-in: Process only tasks explicitly marked for processing

## User Configuration

### Required Configuration
- **Todoist API Token**: Authentication for accessing Todoist account

### Optional Configuration
- **Processing Mode**: Choose between opt-in or opt-out task selection
- **Timezone**: For accurate day calculations (default: UTC)
- **Dry Run Mode**: Preview changes without making actual updates
- **Logging**: Control log output destination and verbosity

## Task Processing Rules

### Task Format Requirements
- On day 1, tasks have no parentheses
- From day 2 onward, closing parentheses `)` are added at the start of the task
- One space must follow the last parenthesis to separate it from the task content
- Pattern: `[parentheses][space][task-content]` (e.g., `)) Task content`)
- Spaces will be automatically normalized during processing

### Staleness Increment Rules
- Increment parentheses count based on days since last update
- If script hasn't run for multiple days, add correct number of parentheses for missed days
- Use timezone-aware day calculations
- Skip tasks already updated on the same day
- Do not apply aging to subtasks (tasks with a parent_id)

### Recurring Task Detection
- Support tasks marked as recurring in Todoist
- Support user-defined recurring labels
- Query completion history for recurring tasks

### Content Preservation
- Preserve task content except for parentheses count and required spacing
- Always maintain one space after the last parenthesis for readability
- Maintain task description and other metadata
- Store processing metadata for day calculation in task description
- Fix any inconsistent spacing during processing

## System Behavior

### Processing Flow
1. Authenticate with Todoist API
2. Fetch all active tasks
3. Filter tasks based on selection criteria
4. Process each eligible task:
   - For tasks with no age markers (first-time processing):
     - Add metadata but don't change visual appearance
     - Start tracking for future aging
   - For tasks with existing age markers or previously tracked tasks:
     - Determine if task is recurring
     - Calculate appropriate staleness count
     - Update task with correct parentheses count
     - Ensure proper spacing after parentheses
   - Fix any formatting inconsistencies
5. Report processing results

### Error Handling
- Continue processing if individual tasks fail
- Log all errors with context
- Provide summary of successes and failures
- Fail gracefully on configuration errors

### Performance Requirements
- Process tasks efficiently with reasonable API rate limits
- Handle typical personal task loads (hundreds of tasks)
- Minimize API calls through intelligent caching

## API Integration

### Todoist API Requirements
- **Task Management**: Read and update task content and descriptions
- **Activity Tracking**: Query task completion history for recurring tasks
- **Authentication**: Support Bearer token authentication

### Expected API Operations
- Fetch all active tasks
- Update individual task content and metadata
- Query task completion events (for recurring tasks)

## Operational Requirements

### Execution Model
- Designed for daily automated execution (cron, scheduled task, etc.)
- Support manual execution for testing and immediate updates
- Provide dry-run capability for safe testing

### Logging and Monitoring
- Log all processing activities
- Provide clear success/failure reporting
- Support different log levels and destinations
- Include dry-run simulation logging

### Data Storage
- No persistent data storage required
- All state derived from Todoist API on each run
- Temporary processing state only

## User Interface Requirements

### Configuration Interface
- Environment variable configuration
- Configuration file support (.env)
- Clear error messages for invalid configuration

### Execution Interface
- Command-line execution
- Clear progress indication during processing
- Informative completion summary

### Dry-Run Interface
- Preview all changes before execution
- Clearly indicate simulated actions
- Show before/after states for task updates

## Quality Requirements

### Reliability
- Handle API failures gracefully
- Recover from individual task processing errors
- Validate task format before processing

### Accuracy
- Precise day calculations using user timezone
- Accurate completion detection for recurring tasks
- Consistent staleness increment logic

### Maintainability
- Clear separation between configuration, business logic, and API integration
- Comprehensive error handling and logging
- Testable components with minimal external dependencies

## Future Considerations

### Extensibility
- Architecture should support additional task automation features
- Plugin system for custom staleness calculations
- Support for different task management systems

### Scalability
- Handle larger task volumes efficiently
- Support multiple Todoist accounts
- Optimize API usage patterns

### User Experience
- Web interface for configuration and monitoring
- Real-time processing status
- Historical processing reports

## Success Criteria

### Functional Success
- Tasks show accurate staleness indicators
- Recurring tasks reset properly upon completion
- User can control which tasks are processed
- System handles errors gracefully

### Technical Success
- Reliable daily execution
- Efficient API usage
- Clear logging and error reporting
- Easy configuration and deployment

### User Success
- Visual task staleness helps prioritize work
- Minimal setup and maintenance effort
- Predictable and reliable behavior
- Safe testing through dry-run mode

## Non-Requirements

### Out of Scope
- Task creation or deletion
- Complex task management features
- Real-time processing or webhooks
- Multi-user or team features
- Advanced analytics or reporting
- Integration with other task management systems

### Assumptions
- Users have basic command-line familiarity
- Daily execution frequency is sufficient
- Todoist API availability and stability