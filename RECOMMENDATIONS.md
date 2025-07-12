# Todoscript Code Review and Recommendations

## Overview
This document provides targeted recommendations for improving the todoscript codebase. The focus is on **simple, practical improvements** that enhance maintainability without over-engineering a focused CLI tool.

**Philosophy**: Keep it simple. Todoscript is a single-purpose CLI tool - recommendations should match that scope and complexity level.

---

## HIGH-PRIORITY CHANGES (Simple but Impactful)

### 1. Return Config Instead of Setting Globals
**Current Issue**: `loadConfig()` sets many global variables, making testing harder.

**Simple Fix**: Return a config struct instead.

```go
type Config struct {
    TodoistToken     string
    DryRun          bool
    Verbose         bool
    AutoAgeByDefault bool
    Timezone        *time.Location
}

// Change from:
func loadConfig() error { /* sets globals */ }

// To:
func loadConfig() (*Config, error) { /* returns config */ }

// Then pass config to functions that need it:
func processAllTasks(config *Config) error
func processTask(config *Config, task Task) error
```

**Benefits**: Easier testing, clearer dependencies, no global state changes after startup.

### 2. Remove Unused Code
**Current Issue**: `buildTaskMap()` and `tasksByContent` appear unused.

**Simple Fix**: Delete unused code.
- Remove `buildTaskMap()` function if duplicate detection isn't used
- Remove `tasksByContent` global map
- Clean up any related unused variables

**Benefits**: Less code to maintain, clearer intent.

### 3. Consistent Error Handling
**Current Issue**: `getDaysSinceCompletion()` returns -1 on error instead of explicit error.

**Simple Fix**: Return explicit errors consistently.

```go
// Change from:
func getDaysSinceCompletion(taskID string) int  // returns -1 on error

// To:
func getDaysSinceCompletion(config *Config, taskID string) (int, error)
```

**Benefits**: Clearer error handling, consistent patterns throughout codebase.

### 4. Extract Long Functions
**Current Issue**: `calculateTaskUpdate()` and `processTask()` are quite long.

**Simple Fix**: Break into smaller, focused functions.

```go
// Instead of one 80-line calculateTaskUpdate(), split into:
func calculateTaskUpdate(ctx TaskContext, now time.Time) TaskUpdateInfo {
    ageInfo := extractTaskAgingInfo(ctx.Task.Content)
    
    if !ageInfo.HasAgeMarkers {
        return handleFirstTimeTask(ctx, now)
    }
    
    return handleExistingTask(ctx, ageInfo, now)
}

func handleFirstTimeTask(ctx TaskContext, now time.Time) TaskUpdateInfo { /* ... */ }
func handleExistingTask(ctx TaskContext, ageInfo TaskAgeInfo, now time.Time) TaskUpdateInfo { /* ... */ }
```

**Benefits**: Easier to understand, test, and modify individual logic paths.

---

## MEDIUM-PRIORITY CHANGES (Code Quality)

### 5. Extract Constants
**Current Issue**: Magic strings and numbers scattered throughout.

**Simple Fix**: Move to constants section.

```go
const (
    MetadataTimeFormat = time.RFC3339
    LabelRecurring = "recurring"
    LabelNoAutoAge = "no-autoage"
    LabelAutoAge = "autoage"
    HTTPTimeoutSeconds = 30
)
```

### 6. Improve Error Messages
**Current Issue**: Some error messages lack context.

**Simple Fix**: Add more context to errors.

```go
// Instead of:
return fmt.Errorf("API request failed: %w", err)

// Use:
return fmt.Errorf("failed to update task %s: %w", taskID, err)
```

### 7. Simplify HTTP Functions
**Current Issue**: HTTP logic is scattered but not complex enough to need abstraction.

**Simple Fix**: Just consolidate similar patterns.

```go
// Keep the existing getTodoistData/postTodoistData pattern
// But add simple retry for transient failures:
func getTodoistDataWithRetry(url string, target any) error {
    for i := 0; i < 3; i++ {
        err := getTodoistData(url, target)
        if err == nil {
            return nil
        }
        if !isRetryableError(err) {
            return err
        }
        time.Sleep(time.Duration(i+1) * time.Second)
    }
    return fmt.Errorf("max retries exceeded")
}
```

---

## LOW-PRIORITY CHANGES (Polish)

### 8. Better Documentation
**Current Issue**: Some complex business logic lacks comments.

**Simple Fix**: Add comments explaining the business rules.

```go
// decideUpdateAction determines what to do with a task based on:
// 1. Recurring tasks: Reset to days-since-completion + 1
// 2. Same-day updates: Skip to avoid double-processing  
// 3. Midnight passed: Increment by days elapsed
// 4. First-time tasks: Default increment by 1
func decideUpdateAction(currentCount int, ctx TaskContext, lastUpdated time.Time, now time.Time) UpdateAction
```

### 9. Minor Performance Improvements
**Current Issue**: A few opportunities for small optimizations.

**Simple Fixes**:
- Use `strings.Builder` for content generation with many parentheses
- Pre-allocate slices when size is known: `make([]Task, 0, len(allTasks))`
- Add context cancellation to HTTP requests for timeout handling

### 10. Test Improvements
**Current Issue**: Some edge cases could use better test coverage.

**Simple Fix**: Add a few more test cases for error scenarios.

```go
func TestCalculateTaskUpdate_EdgeCases(t *testing.T) {
    tests := []struct {
        name string
        task Task
        want string
    }{
        {"empty content", Task{Content: ""}, ""},
        {"very long content", Task{Content: strings.Repeat("a", 100)}, "..."},
        {"malformed parentheses", Task{Content: ")))invalid"}, "..."},
    }
    // ...
}
```

---

## WHAT NOT TO CHANGE

### Things That Are Fine As-Is
1. **Global HTTP client** - Shared client with connection pooling is good for a CLI tool
2. **Pre-compiled regex patterns** - Good performance optimization
3. **Current test structure** - Comprehensive table test works well
4. **Business logic purity** - Most business logic functions are already pure
5. **Simple main() function** - Linear flow is appropriate for CLI tool

### Avoid Over-Engineering
- **No dependency injection frameworks** - Overkill for a CLI tool
- **No complex interfaces** - Current concrete types are fine
- **No pipeline patterns** - Linear processing is easier to follow
- **No structured logging libraries** - Standard logger is sufficient
- **No configuration frameworks** - Environment variables work fine

---

## IMPLEMENTATION ORDER

### Phase 1 (Do First)
1. Return config struct instead of setting globals
2. Remove unused `buildTaskMap()` code
3. Fix `getDaysSinceCompletion()` error handling

### Phase 2 (After Phase 1)
1. Extract long functions into smaller ones
2. Extract constants
3. Improve error messages

### Phase 3 (Polish)
1. Documentation improvements
2. Minor performance optimizations
3. Additional test coverage

---

## CONCLUSION

The todoscript codebase is already well-structured for its purpose. The main improvements focus on:

1. **Eliminating global state** (config struct)
2. **Cleaning up unused code** 
3. **Consistent error handling**
4. **Breaking down long functions**

These changes will improve maintainability while keeping the code simple and appropriate for a focused CLI tool. Avoid the temptation to over-engineer - the current architecture is fundamentally sound.