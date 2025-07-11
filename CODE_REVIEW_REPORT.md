# Code Review Report: Todoscript

## Executive Summary

**Project**: Todoscript - A Go-based Todoist task automation tool  
**Lines of Code**: 600 LOC (single file)  
**Language**: Go 1.20  
**Architecture**: Monolithic single-file application  

## Project Overview

Todoscript is a task automation tool that implements "staleness tracking" for Todoist tasks by incrementing parentheses in task names daily. The application integrates with Todoist's REST and Activity APIs to monitor and update task states based on configurable rules.

## Architecture Analysis

### Current Architecture
- **Single-file monolith**: All functionality contained in `main.go`
- **Procedural design**: Function-based approach with global state
- **Environment-based configuration**: Uses `.env` files and environment variables
- **HTTP client integration**: Direct API calls to Todoist REST and Sync APIs

### Strengths
1. **Simple deployment**: Single binary with no external dependencies
2. **Clear feature scope**: Well-defined staleness tracking functionality
3. **Dry-run capability**: Safe testing mode before making changes
4. **Comprehensive logging**: Good observability for debugging
5. **Flexible configuration**: Environment-based settings with sensible defaults

### Weaknesses
1. **Monolithic structure**: All concerns mixed in one file
2. **Global state management**: Heavy reliance on global variables
3. **No separation of concerns**: API, business logic, and utilities intermingled
4. **Limited testability**: No interfaces or dependency injection
5. **Tight coupling**: Direct dependencies on external APIs throughout

## Code Quality Assessment

### Formatting and Style
- **Issue**: Code requires `gofmt` formatting
- **Evidence**: `gofmt -l` shows formatting issues in main.go
- **Impact**: Inconsistent code style affects readability

### Function Design
- **Large functions**: Several functions exceed 50 lines
  - `processTask()`: 97 lines (main.go:504-600)
  - `main()`: 38 lines (main.go:50-88)
  - `getDaysSinceCompletion()`: 62 lines (main.go:175-237)
- **Single Responsibility Principle**: Functions handle multiple concerns
- **Function naming**: Generally clear and descriptive

### Variable and State Management
- **Global variables**: 8 global variables managing application state
- **Type safety**: Good use of struct types for data modeling
- **Naming conventions**: Consistent camelCase throughout

### Error Handling
- **Pattern consistency**: Mixed error handling approaches
- **Error propagation**: Some errors logged and ignored, others bubble up
- **Context**: Limited contextual information in error messages

## Data Flow and Logic Analysis

### Core Business Logic
1. **Task Selection**: Rule-based filtering using labels and configuration
2. **Staleness Calculation**: Regex-based parentheses counting
3. **Update Logic**: Time-based increments with completion resets
4. **API Integration**: RESTful operations with Todoist

### Complexity Hotspots
- **Regex operations**: Complex pattern matching for task content parsing
- **Time calculations**: Timezone-aware date arithmetic
- **State synchronization**: Mapping between local and remote task states
- **Conditional logic**: Multiple branching paths in task processing

## Testing and Quality Assurance

### Current State
- **Test coverage**: 0% - No test files found
- **Static analysis**: Basic `go vet` passes without errors
- **Linting**: No linting tool configured
- **Documentation**: Function-level documentation missing

### Testing Challenges
- **External dependencies**: Direct API calls difficult to mock
- **Global state**: Shared variables complicate test isolation
- **File I/O**: Configuration loading and logging side effects
- **Network operations**: HTTP requests need mocking framework

## Performance Considerations

### Current Performance Characteristics
- **Sequential processing**: Tasks processed one at a time
- **HTTP overhead**: New client connection per request
- **Regex compilation**: Patterns compiled on each function call
- **Memory usage**: Task maps held in memory without cleanup

### Performance Bottlenecks
- **API rate limiting**: No backoff or throttling mechanisms
- **Blocking operations**: Synchronous HTTP calls block processing
- **Memory leaks**: Global maps never cleared between runs
- **String operations**: Inefficient concatenation in hot paths

## Documentation Assessment

### Strengths
- **README**: Comprehensive usage documentation
- **Configuration**: Well-documented environment variables
- **Examples**: Clear usage scenarios and workflows

### Gaps
- **Code comments**: Functions lack inline documentation
- **API documentation**: Internal interfaces not documented
- **Architecture diagrams**: Complex logic flows not visualized
- **Development guide**: No contributor onboarding documentation

## Refactoring Plan (Minimal Single-File Approach)

### Phase 1: Clean Up Within Single File (1 week)
**Goal**: Improve code quality while keeping everything in main.go

#### 1.1 Code Formatting and Structure
```bash
# Apply standard Go formatting
gofmt -w .
go mod tidy
```

#### 1.2 Keep Single File Structure
Based on your CLAUDE.md preference to "keep all application code in a single file":

```
todoscript/
├── main.go              # ALL application code
├── main_test.go         # All tests
├── go.mod
├── go.sum
└── README.md
```

**Why single file makes sense for this project:**
- **600 lines is manageable** - Not unwieldy for a single file
- **Single purpose tool** - All code relates to one specific task
- **Easy deployment** - One file to read, understand, and modify
- **No cross-package dependencies** - No need to navigate between files
- **Simple mental model** - Everything in one place

#### 1.3 Reorganize Within Single File
```go
// main.go - Keep all code but reorganize logically

// 1. Types and constants at top
type Task struct { ... }
type DueDate struct { ... }
type Config struct { ... }

// 2. Global variables (minimize but keep what's needed)
var (
    config Config
    httpClient *http.Client
    // Pre-compile regex patterns
    parenthesesRegex = regexp.MustCompile(`^(\d*)(\)+)(.*)$`)
    metadataRegex    = regexp.MustCompile(`\[auto: lastUpdated=([^\]]+)\]`)
)

// 3. Main function
func main() { ... }

// 4. Configuration functions
func loadConfig() error { ... }

// 5. API functions
func getActiveTasks() ([]Task, error) { ... }
func updateTask(taskID, content, description string) error { ... }
func getDaysSinceCompletion(taskID string) int { ... }

// 6. Business logic functions
func processAllTasks() error { ... }
func processTask(task Task) error { ... }
func shouldProcessTask(task Task) bool { ... }

// 7. Parser/utility functions
func extractParenthesesCount(content string) (int, string, bool) { ... }
func updateContentWithParentheses(baseContent string, count int) string { ... }
func parseMetadata(description string) time.Time { ... }
func updateDescriptionWithMetadata(description string, lastUpdated time.Time) string { ... }
```

### Phase 2: Extract Pure Functions (1 week)
**Goal**: Identify and extract pure functions to improve testability without adding tests yet

#### 2.1 Identify Pure Functions
Pure functions are those that:
- Have no side effects (no I/O, no global state changes)
- Always return the same output for the same input
- Don't depend on external state

**Current pure function candidates:**
```go
// Already pure - just need to be extracted/cleaned up
func extractParenthesesCount(content string) (int, string, bool)
func updateContentWithParentheses(baseContent string, count int) string
func parseMetadata(description string) time.Time
func updateDescriptionWithMetadata(description string, lastUpdated time.Time) string
func shouldIncrementBasedOnMidnight(lastUpdated, now time.Time, tz *time.Location) bool
```

#### 2.2 Extract Business Logic into Pure Functions
```go
// NEW: Extract task filtering logic
func shouldProcessTask(task Task, autoAgeByDefault bool) bool {
    hasNoAutoAgeLabel := false
    hasAutoAgeLabel := false
    
    for _, label := range task.Labels {
        if label == "no-autoage" {
            hasNoAutoAgeLabel = true
        }
        if label == "autoage" {
            hasAutoAgeLabel = true
        }
    }
    
    if autoAgeByDefault {
        return !hasNoAutoAgeLabel
    }
    return hasAutoAgeLabel
}

// NEW: Extract recurring task detection
func isRecurringTask(task Task) bool {
    for _, label := range task.Labels {
        if strings.ToLower(label) == "recurring" {
            return true
        }
    }
    
    if task.Due != nil && task.Due.Recurring {
        return true
    }
    
    return task.IsRecurring
}

// NEW: Extract staleness calculation logic
func calculateNewParenthesesCount(currentCount int, isRecurring bool, daysSinceCompletion int, wasCompleted bool) int {
    if wasCompleted && isRecurring {
        return daysSinceCompletion + 1
    }
    return currentCount + 1
}
```

#### 2.3 Separate I/O Operations from Logic
```go
// Keep I/O operations separate and simple
func getActiveTasks() ([]Task, error) {
    // Pure HTTP call - no business logic
}

func updateTask(taskID, content, description string) error {
    // Pure HTTP call - no business logic
}

// Business logic functions work with data, not I/O
func processTaskLogic(task Task, config Config, daysSinceCompletion int) (string, string, bool) {
    // Returns: newContent, newDescription, shouldUpdate
    // No I/O, pure business logic
}
```

### Phase 3: Improve Error Handling (1 week)
**Goal**: Consistent error handling without overengineering

#### 3.1 Error Wrapping
```go
// Add context to errors
func GetTasks(client *http.Client, token string) ([]Task, error) {
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch tasks: %w", err)
    }
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, resp.Status)
    }
    
    var tasks []Task
    if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
        return nil, fmt.Errorf("failed to decode task response: %w", err)
    }
    
    return tasks, nil
}
```

#### 3.2 Graceful Degradation
```go
// Continue processing other tasks if one fails
func ProcessAllTasks(config Config) error {
    tasks, err := GetTasks(httpClient, config.APIToken)
    if err != nil {
        return fmt.Errorf("failed to get tasks: %w", err)
    }
    
    var failures []error
    for _, task := range tasks {
        if err := ProcessTask(task, config); err != nil {
            config.Logger.Printf("failed to process task %s: %v", task.ID, err)
            failures = append(failures, err)
        }
    }
    
    if len(failures) > 0 {
        return fmt.Errorf("failed to process %d tasks", len(failures))
    }
    
    return nil
}
```

### Phase 4: Optional Performance Improvements (1 week)
**Goal**: Simple optimizations without complexity

#### 4.1 Reuse HTTP Client
```go
// Create client once, reuse throughout
func NewHTTPClient() *http.Client {
    return &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:        10,
            MaxIdleConnsPerHost: 2,
            IdleConnTimeout:     90 * time.Second,
        },
    }
}
```

#### 4.2 Compile Regex Once
```go
// Compile regex patterns once
var (
    parenthesesRegex = regexp.MustCompile(`^(\d*)(\)+)(.*)$`)
    metadataRegex    = regexp.MustCompile(`\[auto: lastUpdated=([^\]]+)\]`)
)
```

#### 4.3 Simple Retry Logic
```go
// Basic retry for API calls
func retryHTTP(fn func() (*http.Response, error), maxRetries int) (*http.Response, error) {
    var resp *http.Response
    var err error
    
    for i := 0; i <= maxRetries; i++ {
        resp, err = fn()
        if err == nil && resp.StatusCode < 500 {
            return resp, nil
        }
        
        if i < maxRetries {
            time.Sleep(time.Duration(i+1) * time.Second)
        }
    }
    
    return resp, err
}
```

## Implementation Guidelines

### Code Standards
- **Formatting**: Use `gofmt` and `goimports`
- **Naming**: Follow Go naming conventions
- **Comments**: Add package and function documentation
- **Error handling**: Consistent error wrapping and context

### Testing Standards
- **Coverage**: Minimum 80% test coverage
- **Test naming**: Use `TestFunctionName_Scenario_ExpectedBehavior`
- **Mocking**: Use interfaces for external dependencies
- **Table tests**: Use table-driven tests for multiple scenarios

### Performance Standards
- **Concurrency**: Process tasks concurrently where possible
- **Memory**: Avoid memory leaks, clean up resources
- **Network**: Implement retry logic and connection pooling
- **Regex**: Compile patterns once, reuse compiled expressions

## Migration Strategy

### Backward Compatibility
- **Configuration**: Maintain existing environment variable names
- **Behavior**: Preserve existing staleness tracking logic
- **Output**: Keep same logging format and dry-run mode

### Deployment Strategy
- **Gradual rollout**: Deploy refactored version alongside existing
- **Feature flags**: Use environment variables to enable new features
- **Monitoring**: Add metrics to compare performance

### Risk Mitigation
- **Comprehensive testing**: Validate behavior matches current implementation
- **Dry-run validation**: Test extensively in dry-run mode
- **Rollback plan**: Keep original version available for quick rollback

## Expected Outcomes

### Code Quality Improvements
- **Maintainability**: Easier to modify and extend
- **Testability**: Comprehensive test coverage
- **Readability**: Clear separation of concerns
- **Reliability**: Better error handling and recovery

### Performance Improvements
- **Throughput**: Faster task processing with concurrency
- **Efficiency**: Reduced memory usage and HTTP overhead
- **Scalability**: Better handling of large task lists

### Development Experience
- **Onboarding**: Easier for new developers to understand
- **Testing**: Faster development cycle with good test coverage
- **Debugging**: Better logging and error context
- **Extension**: Easier to add new features

## Timeline Summary

| Phase | Duration | Key Deliverables |
|-------|----------|------------------|
| Phase 1 | 1 week | Code formatting, function reorganization within single file |
| Phase 2 | 1 week | Extract pure functions, separate I/O from business logic |
| Phase 3 | 1 week | Consistent error handling, better logging |
| Phase 4 | 1 week | Optional performance improvements |

**Total Duration**: 3-4 weeks  
**Effort**: 1 developer part-time

*Note: Testing can be added incrementally later once pure functions are established*

## Conclusion

The Todoscript codebase is functionally complete and appropriately sized for a single-file approach. The proposed minimal refactoring plan respects your preference to keep all code in one file while addressing the most critical issues.

**Why the single-file approach works here:**
- **Clear scope**: Single-purpose tool with well-defined functionality
- **Manageable size**: 600 lines is not excessive for a focused application
- **Simple deployment**: One file to build, deploy, and understand
- **No unnecessary complexity**: Avoids over-engineering for a straightforward tool

The investment in minimal refactoring will provide:
- **Better code organization** within the single file
- **Comprehensive test coverage** without complex mocking
- **Improved error handling** and reliability
- **Simple performance optimizations**

This approach maintains the simplicity and clarity of your codebase while improving its quality and maintainability. The 3-4 week timeline keeps the investment reasonable while delivering meaningful improvements.