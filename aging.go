package main

import (
	"fmt"
	"strings"
	"time"
)

// shouldIncrementBasedOnMidnight determines if enough time has passed since the
// last update to increment the parentheses count. It checks if the current time
// has passed the midnight following the last update in the configured timezone.
func shouldIncrementBasedOnMidnight(lastUpdated, now time.Time, tz *time.Location) bool {
	// Convert last update to configured timezone
	lastUpdatedInTZ := lastUpdated.In(tz)

	// Calculate the next midnight after last update
	nextMidnight := time.Date(
		lastUpdatedInTZ.Year(), lastUpdatedInTZ.Month(), lastUpdatedInTZ.Day()+1,
		0, 0, 0, 0, tz,
	)

	// Check if current time has passed that midnight
	return now.In(tz).After(nextMidnight)
}

// calculateDaysSinceUpdate calculates the number of days that have passed
// since the last update, using the configured timezone for day boundaries.
func calculateDaysSinceUpdate(lastUpdated, now time.Time, tz *time.Location) int {
	// Convert times to the configured timezone
	lastUpdatedInTZ := lastUpdated.In(tz)
	nowInTZ := now.In(tz)

	// Truncate to days to ensure we're counting full days
	lastUpdatedDay := time.Date(
		lastUpdatedInTZ.Year(), lastUpdatedInTZ.Month(), lastUpdatedInTZ.Day(),
		0, 0, 0, 0, tz,
	)

	nowDay := time.Date(
		nowInTZ.Year(), nowInTZ.Month(), nowInTZ.Day(),
		0, 0, 0, 0, tz,
	)

	// Calculate days difference and ensure non-negative
	days := int(nowDay.Sub(lastUpdatedDay).Hours() / 24) //nolint:mnd // hours to days conversion
	return max(0, days)
}

// extractMetadataTimestamp extracts the last updated timestamp from task description metadata.
func extractMetadataTimestamp(description string) time.Time {
	var lastUpdated time.Time

	// Use pre-compiled regex pattern
	matches := metadataPattern.FindStringSubmatch(description)

	if len(matches) == 2 { //nolint:mnd // 2 groups expected from regex (total + captured group)
		// Group 1 contains the timestamp string
		timestampStr := matches[1]
		parsed, err := time.Parse(time.RFC3339, timestampStr)
		if err == nil {
			lastUpdated = parsed
		}
	}

	return lastUpdated
}

// updateDescriptionWithMetadata adds or updates the metadata timestamp in a task description.
func updateDescriptionWithMetadata(description string, lastUpdated time.Time) string {
	metadataStr := fmt.Sprintf(metadataTemplate, lastUpdated.Format(time.RFC3339))

	// If existing metadata found, replace it
	if metadataPattern.MatchString(description) {
		return metadataPattern.ReplaceAllString(description, metadataStr)
	}

	// Otherwise, append metadata to description
	if description == "" {
		return metadataStr
	}
	return description + "\n\n" + metadataStr
}

// extractTaskAgingInfo extracts the age count from a task's parentheses markers.
// Example: "))) Do something" → TaskAgeInfo{AgeCount: 3, ContentWithoutAge: " Do something", HasAgeMarkers: true}.
func extractTaskAgingInfo(content string) TaskAgeInfo {
	// Use pre-compiled regex pattern
	matches := taskAgePattern.FindStringSubmatch(content)

	if len(matches) != 4 { //nolint:mnd // 4 groups expected from regex (total + 3 captured groups)
		return TaskAgeInfo{
			AgeCount:          0,
			ContentWithoutAge: content,
			HasAgeMarkers:     false,
		}
	}

	// Extract components from regex groups
	// Group 1: optional number prefix (no longer used)
	// Group 2: parentheses markers (e.g., ")))" in "))) task")
	// Group 3: remaining content (e.g., " task" in "))) task")
	ageMarkers := matches[2]
	remainingContent := matches[3]

	// Count parentheses to get age count
	ageCount := len(ageMarkers)

	// Preserve spaces in content as expected by tests
	return TaskAgeInfo{
		AgeCount:          ageCount,
		ContentWithoutAge: remainingContent,
		HasAgeMarkers:     true,
	}
}

// addAgingMarkersToContent adds the specified number of parentheses age markers to task content.
// Example: addAgingMarkersToContent(" Do something", 4) → "))))) Do something".
func addAgingMarkersToContent(contentWithoutAge string, count int) string {
	// Find the optional number in the content
	matches := contentStartRegex.FindStringSubmatch(contentWithoutAge)

	if len(matches) != 3 { //nolint:mnd // 3 groups expected from regex (total + 2 captured groups)
		// If regex fails, just return the original content
		return contentWithoutAge
	}

	// Extract components from simple regex: "^(\d*)(.*)$"
	// Group 1: optional number prefix (e.g., "3" in "3 Do something")
	// Group 2: remaining content (e.g., " Do something")
	numberPrefix := matches[1]
	remainingContent := matches[2]

	// Create string with the specified number of parentheses
	ageMarkers := strings.Repeat(taskAgingMarker, count)

	// Ensure there's a space after the parentheses if we have any content
	// This maintains consistency with the original behavior
	if remainingContent == "" {
		remainingContent = " " // Add trailing space for consistency
	} else if !strings.HasPrefix(remainingContent, " ") {
		remainingContent = " " + remainingContent
	}

	return numberPrefix + ageMarkers + remainingContent
}
