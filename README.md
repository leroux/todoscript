# todoscript

[![CI](https://github.com/leroux/todoscript/actions/workflows/ci.yml/badge.svg)](https://github.com/leroux/todoscript/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/leroux/todoscript)](https://goreportcard.com/report/github.com/leroux/todoscript)
[![codecov](https://codecov.io/gh/leroux/todoscript/branch/main/graph/badge.svg)](https://codecov.io/gh/leroux/todoscript)

A Todoist task aging automation tool that automatically increments visual "age markers" (parentheses) on tasks to help track how long they've been sitting in your todo list.

## How it works

- Tasks with pattern ") Do something" become ")) Do something" after midnight
- Recurring tasks reset their age when completed: "))))) Task" → "))) Task"  
- Tasks can opt-in with `@autoage` label or opt-out with `@no-autoage` label
- Dry-run mode available for testing changes before applying them

## Installation

### From Releases

Download the latest binary for your platform from [releases](https://github.com/leroux/todoscript/releases).

### Build from Source

```bash
go install github.com/leroux/todoscript@latest
```

Or clone and build:

```bash
git clone https://github.com/leroux/todoscript.git
cd todoscript
go build
```

## Configuration

Set your Todoist API token:

```bash
export TODOIST_TOKEN="your-api-token-here"
```

Optional environment variables:

- `DRY_RUN=true` - Preview changes without making them
- `VERBOSE=true` - Enable verbose logging  
- `AUTOAGE_BY_DEFAULT=true` - Age all tasks unless opted out with `@no-autoage`
- `TIMEZONE=America/New_York` - Set timezone for midnight calculations
- `LOG_FILE=/path/to/log` - Write logs to file instead of stdout

## Usage

```bash
# Run with current settings
./todoscript

# Dry run to preview changes
DRY_RUN=true ./todoscript

# Run with verbose logging
VERBOSE=true ./todoscript
```

## Development

```bash
# Run tests
make test

# Run linter
make lint

# Build binary
make build

# Run with coverage
make test-coverage
```

## Architecture

The codebase is organized into focused modules:

- `main.go` - Entry point
- `types.go` - Data structures and constants
- `config.go` - Configuration loading
- `todoist.go` - HTTP client and Todoist API
- `aging.go` - Task aging logic and parsing
- `processing.go` - Business logic orchestration