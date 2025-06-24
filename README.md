# Todoscript

A Go-based automation platform for Todoist that currently implements task staleness tracking with more features planned for future development.

## Overview

Todoscript is a flexible platform for automating Todoist tasks. The current implementation focuses on managing tasks tagged with `@auto`, but the architecture is designed to support multiple automation features in the future.

### Current Feature: Task Staleness Tracking
The initial implementation tracks task staleness by incrementing parentheses in task names daily and resetting the counter when recurring tasks are completed.

## Features

### Core Platform Features
- **Extensible Architecture**: Designed to support multiple Todoist automation features
- **Selective Processing**: Uses tags to determine which tasks to process
- **Dry Run Mode**: Preview changes without modifying actual tasks
- **Configurable Logging**: Console and file-based logging options
- **Activity Log Integration**: Uses Todoist API to monitor task status changes

### Staleness Tracking Feature
- **Visual Staleness Indicator**: Adds parentheses daily to task names (e.g., "50)" → "50))" → "50)))")
- **Recurring Task Support**: Resets parentheses count when a recurring task is completed
- **Daily Update Limit**: Only updates staleness indicators once per 24 hours
- **Metadata Storage**: Tracks last update time in task descriptions

## Requirements

- Go 1.20 or higher
- Todoist API token
- Tasks following the pattern: `<number>)<text>` (e.g., "10) Do laundry")

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/todoscript.git
cd todoscript

# Build the binary
go build -o todoscript
```

## Configuration

Create a `.env` file in the project root:

```
# Todoist API token (required)
TODOIST_TOKEN=your_todoist_api_token_here

# Set to true for dry run mode (no changes will be made)
DRY_RUN=false

# Set to true for verbose logging
VERBOSE=true

# Log file path (optional, logs to stdout if not specified)
LOG_FILE=
```

## Usage

```bash
# Run with default settings (from .env file)
./todoscript

# Run in dry-run mode (preview changes without modifying tasks)
DRY_RUN=true ./todoscript

# Run with verbose logging
VERBOSE=true ./todoscript

# Run in dry-run mode with verbose logging
DRY_RUN=true VERBOSE=true ./todoscript
```

## How It Works

### Staleness Tracking

1. **Task Selection**: The script processes tasks tagged with `@auto`.
2. **Parentheses Tracking**: Tasks following the pattern `<number>)<text>` get an additional parenthesis daily to track staleness.
3. **Recurring Tasks**: For recurring tasks (identified by `Due.Recurring = true`), the parentheses count resets to 1 when the task is completed.
4. **Completion Detection**: The script uses the Todoist Activity Log to detect when recurring tasks were last completed.
5. **Metadata**: The script stores metadata in the task description to track when the task was last updated.

### Platform Architecture

Todoscript is built with extensibility in mind:

1. **API Integration**: Robust integration with both REST and Sync Todoist APIs
2. **Modular Design**: Core functionality is separated from specific automation features
3. **Configuration System**: Flexible environment-based configuration
4. **Logging Framework**: Comprehensive logging for debugging and monitoring

## Example

If you have a task "10) Write blog post" with the `@auto` tag:

- Day 1: "10) Write blog post"
- Day 2: "10)) Write blog post" (one more parenthesis)
- Day 3: "10))) Write blog post" (one more parenthesis)
- After you complete the task (if recurring): "10) Write blog post" (reset to one parenthesis)

## Makefile Commands

```bash
# Build the application
make build

# Run the application
make run

# Run in dry-run mode
make dry-run

# Run in verbose mode
make verbose

# Run in debug mode (dry-run + verbose)
make debug

# Clean build artifacts
make clean

# Install the application globally
make install

# Display help message
make help
```

## Contributing

Contributions are welcome! Here are some ways you can contribute:

1. **New Automation Features**: Implement additional Todoist automation features
2. **Bug Fixes**: Help identify and fix issues
3. **Documentation**: Improve the documentation or add examples
4. **Testing**: Add tests for existing functionality

Please feel free to submit a Pull Request or open an Issue for discussion.

## License

This project is open source and available under the [MIT License](LICENSE).