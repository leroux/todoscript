// Package main implements todoscript - a Todoist task aging automation tool.
//
// This tool automatically increments visual "age markers" (parentheses) on Todoist tasks
// to help track how long tasks have been sitting in your todo list. The concept is simple:
// tasks get more parentheses the longer they remain incomplete, creating visual pressure
// to either complete them or remove them.
//
// How it works:
// - Tasks with pattern ") Do something" become ")) Do something" after midnight
// - Recurring tasks reset their age when completed: "))))) Task" → "))) Task"
// - Tasks can opt-in with @autoage label or opt-out with @no-autoage label
// - Dry-run mode available for testing changes before applying them
//
// The aging concept creates a visual indication of task staleness, encouraging you to
// either complete long-standing tasks or remove them from your list entirely.
package main

// Main function.
func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		panic(err)
	}

	// Print mode info
	if config.DryRun {
		config.Logger.Printf("Running in DRY RUN mode - no changes will be made")
	}

	config.Logger.Printf("Starting task processing...")

	// Process tasks
	if processErr := processAllTasks(config); processErr != nil {
		config.Logger.Fatalf("Failed to process tasks: %v", processErr)
	}

	config.Logger.Printf("Task processing completed successfully")
}
