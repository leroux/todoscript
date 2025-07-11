# Test Plan for TodoScript

## Overview
This document outlines a minimal testing approach for the TodoScript application, which manages aging of Todoist tasks.

## Getting Started With Testing

To begin implementing tests for TodoScript immediately:

1. Create a `main_test.go` file in the project root
2. Add initial tests for pure functions
3. Run tests with the Go test command

```bash
# Create test file
touch main_test.go

# Run tests
go test -v ./...

# Run with coverage
go test -cover ./...
```

### Quick Test Setup Example

```go
// main_test.go
package main

import (
	"testing"
)

func Test_extractTaskAgingInfo(t *testing.T) {
	// Test a simple case to start
	input := "2)) Do something"
	result := extractTaskAgingInfo(input)
	
	if result.AgeCount != 2 {
		t.Errorf("Expected age count 2, got %d", result.AgeCount)
	}
	
	if result.ContentWithoutAge != "2 Do something" {
		t.Errorf("Expected content '2 Do something', got '%s'", result.ContentWithoutAge)
	}
	
	if !result.HasAgeMarkers {
		t.Error("Expected HasAgeMarkers to be true")
	}
}

## Core Testing Goals
- Verify correct task aging behavior
- Ensure proper API interaction
- Validate configuration handling

## Test Categories

### 1. Task Processing
Tests for the core functionality of parsing and updating task age markers.

### 2. API Communication
Tests for Todoist API requests and response handling.

### 3. Configuration Management
Tests for environment variable loading and configuration setup.

### 4. Error Handling
Tests for proper handling of error conditions and edge cases.

## Key Test Scenarios

### Task Processing
1. Extract age markers from task content with various formats
2. Add age markers to task content with correct count
3. Decide whether to increment, reset, or skip based on task context
4. Calculate task updates based on aging rules
5. Process tasks with different label combinations

### API Communication
1. Authenticate API requests correctly
2. Handle successful HTTP responses and decode JSON
3. Handle API errors and non-OK status codes
4. Process specific Todoist API endpoints (tasks, activities)

### Configuration Management
1. Load environment variables correctly
2. Handle missing required variables
3. Apply default values for optional settings
4. Parse boolean flags correctly

### Error Handling
1. Handle network connectivity issues
2. Manage API rate limiting
3. Report meaningful error messages
4. Continue processing despite individual task failures

## Testing Approach

### Unit Testing
- Test individual functions in isolation
- Use mocks for external dependencies (API, filesystem)
- Focus on pure functions first (e.g., task aging calculations)

### Integration Testing
- Test API interaction using mock HTTP responses
- Verify end-to-end workflow with controlled inputs

### Manual Testing
- Test with real Todoist account using a dedicated test project
- Verify dry-run mode works correctly

## Test Tools & Frameworks

### Recommended Tools
- **Testing Framework**: Go's built-in testing package
- **HTTP Mocking**: httptest package
- **Assertions**: testify package for enhanced assertions
- **Mocks**: gomock for interface mocking

### Example Test Structure
```
todoscript/
├── main.go
├── main_test.go        # Tests for main package functions
├── todoist/            # Package for Todoist API interactions 
│   ├── client.go
│   └── client_test.go  # API client tests
├── task/               # Package for task processing logic
│   ├── processor.go
│   └── processor_test.go
└── config/             # Configuration management
    ├── loader.go
    └── loader_test.go
```

## Implementation Notes

### Current Challenges
- The codebase is organized as a single file with many functions
- External dependencies (API, filesystem) are not easily mockable
- Global state makes unit testing difficult

### Recommended Refactoring for Testability
1. Extract functions into logical packages
2. Use interfaces for external dependencies
3. Inject dependencies rather than using globals

## Next Steps

### Phase 1: Initial Test Setup
1. Create basic test file structure
2. Write tests for pure functions
3. Set up CI pipeline for running tests

### Phase 2: Improve Testability
1. Extract HTTP client into interface
2. Create mock implementations for testing
3. Refactor global state to use dependency injection

### Phase 3: Comprehensive Test Coverage
1. Implement integration tests
2. Add end-to-end tests with mock API
3. Set up code coverage reporting

## Sample Unit Test

Below is a sample unit test for the `extractTaskAgingInfo` function:

```go
package main

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
)

func TestExtractTaskAgingInfo(t *testing.T) {
	// Test cases table
	testCases := []struct {
		name           string
		input          string
		expectedAge    int
		expectedContent string
		hasAgeMarkers  bool
	}{
		{
			name:           "Basic task with age markers",
			input:          "3))) Do something",
			expectedAge:    3,
			expectedContent: "3 Do something",
			hasAgeMarkers:  true,
		},
		{
			name:           "Task with no age markers",
			input:          "Do something",
			expectedAge:    0,
			expectedContent: "Do something",
			hasAgeMarkers:  false,
		},
		{
			name:           "Task with multiple markers",
			input:          "2))))) Complete this task",
			expectedAge:    5,
			expectedContent: "2 Complete this task",
			hasAgeMarkers:  true,
		},
		{
			name:           "Task with just markers",
			input:          "))) Task",
			expectedAge:    3,
			expectedContent: " Task",
			hasAgeMarkers:  true,
		},
	}
	
	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function
			result := extractTaskAgingInfo(tc.input)
			
			// Assert expectations
			assert.Equal(t, tc.expectedAge, result.AgeCount)
			assert.Equal(t, tc.expectedContent, result.ContentWithoutAge)
			assert.Equal(t, tc.hasAgeMarkers, result.HasAgeMarkers)
		})
	}
}
```

## Sample Integration Test

Below is a sample integration test that demonstrates how to test the Todoist API interaction:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	
	"github.com/stretchr/testify/assert"
)

func TestGetActiveTasks(t *testing.T) {
	// Setup mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/rest/v2/tasks", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		
		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{
				"id": "123",
				"content": "2)) Test task",
				"description": "[auto: lastUpdated=2023-01-01T10:00:00Z]",
				"labels": ["autoage"],
				"is_completed": false
			},
			{
				"id": "456",
				"content": "Regular task",
				"description": "",
				"labels": [],
				"is_completed": false
			}
		]`))
	}))
	defer mockServer.Close()
	
	// Setup test environment
	originalAPIURL := apiURL
	originalAPIToken := apiToken
	defer func() {
		// Restore original values
		apiURL = originalAPIURL
		apiToken = originalAPIToken
	}()
	
	// Override for test
	apiURL = mockServer.URL
	apiToken = "test-token"
	
	// Call function under test
	tasks, err := getActiveTasks()
	
	// Assertions
	assert.NoError(t, err)
	assert.Len(t, tasks, 2)
	
	// Check first task
	assert.Equal(t, "123", tasks[0].ID)
	assert.Equal(t, "2)) Test task", tasks[0].Content)
	assert.Contains(t, tasks[0].Labels, "autoage")
	
	// Check second task
	assert.Equal(t, "456", tasks[1].ID)
	assert.Equal(t, "Regular task", tasks[1].Content)
	assert.Empty(t, tasks[1].Labels)
}
```

## Testing Priorities

When implementing tests for TodoScript, focus on these priorities:

1. **Start with pure functions**: Begin with testing functions that have minimal dependencies and deterministic output:
   - `extractTaskAgingInfo`
   - `addAgingMarkersToContent`
   - `decideUpdateAction`
   - `updateDescriptionWithMetadata`

2. **Test critical business logic**: Ensure that the core task aging rules are working correctly:
   - Task aging after midnight
   - Reset behavior for recurring tasks
   - Label-based opt-in/opt-out

3. **API integration**: Test proper interaction with the Todoist API:
   - Request authentication
   - Response parsing
   - Error handling

## Conclusion

This test plan provides a roadmap for implementing comprehensive testing for TodoScript. By following the phased approach outlined here, we can gradually improve test coverage and code quality while maintaining the application's functionality.

The immediate priority should be implementing unit tests for pure functions, followed by refactoring to improve testability. Long-term, the codebase would benefit from being reorganized into packages with clear interfaces between them.