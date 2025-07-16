package main

import (
	"fmt"
	"log" //nolint:depguard // Legacy log usage required for compatibility
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// loadConfig loads configuration from environment variables.
func loadConfig() (*Config, error) {
	// Initialize logger first
	logFile := os.Getenv(envLogFile)
	var logOutput = os.Stdout

	if logFile != "" {
		var err error
		logOutput, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666) //nolint:gosec,golines // Log files need group write permissions
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", logFile, err)
		}
	}

	logger := log.New(logOutput, "[todoscript] ", log.LstdFlags|log.Lshortfile)

	// Initialize config with defaults
	config := &Config{
		APIURL:      defaultAPIURL,
		ActivityURL: defaultActivityURL,
		Logger:      logger,
	}

	// Load .env file if it exists
	if _, err := os.Stat(".env"); err == nil {
		if loadErr := godotenv.Load(); loadErr != nil {
			logger.Printf("Warning: Error loading .env file: %v", loadErr)
		} else {
			logger.Printf("Loaded configuration from .env file")
		}
	}

	// Get API token
	config.TodoistToken = os.Getenv(envTodoistToken)
	if config.TodoistToken == "" {
		return nil, fmt.Errorf("loading configuration failed: missing required environment variable %s",
			envTodoistToken)
	}

	// Parse dry run flag
	dryRunStr := os.Getenv(envDryRun)
	if dryRunStr != "" {
		var err error
		config.DryRun, err = strconv.ParseBool(dryRunStr)
		if err != nil {
			logger.Printf("Warning: Invalid DRY_RUN value '%s', defaulting to false: %v", dryRunStr, err)
			config.DryRun = false
		}
	}

	// Parse verbose flag
	verboseStr := os.Getenv(envVerbose)
	if verboseStr != "" {
		var err error
		config.Verbose, err = strconv.ParseBool(verboseStr)
		if err != nil {
			logger.Printf("Warning: Invalid VERBOSE value '%s', defaulting to false: %v", verboseStr, err)
			config.Verbose = false
		}
	}

	// Parse auto age by default flag (defaults to true)
	config.AutoAgeByDefault = true // Default to true
	autoAgeByDefaultStr := os.Getenv(envAutoAgeDefault)
	if autoAgeByDefaultStr != "" {
		var err error
		config.AutoAgeByDefault, err = strconv.ParseBool(autoAgeByDefaultStr)
		if err != nil {
			logger.Printf("Warning: Invalid AUTOAGE_BY_DEFAULT value '%s', defaulting to true: %v",
				autoAgeByDefaultStr, err)
			config.AutoAgeByDefault = true
		}
	}

	// Set timezone
	timezoneStr := os.Getenv(envTimezone)
	if timezoneStr == "" {
		timezoneStr = defaultTimezone
	}

	timezone, err := time.LoadLocation(timezoneStr)
	if err != nil {
		logger.Printf("Warning: Invalid timezone '%s', using UTC: %v", timezoneStr, err)
		timezone, _ = time.LoadLocation(defaultTimezone)
	}
	config.Timezone = timezone

	return config, nil
}
