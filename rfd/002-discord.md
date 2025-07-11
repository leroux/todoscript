# RFD 002: Discord Integration for Todoscript

## Summary

This document proposes a Discord integration for the Todoscript application that would enable two key features:
1. Morning task notifications - A daily digest of pending tasks sent to a Discord channel
2. Nagging reminders - Recurring notifications for overdue tasks with a #nagme tag

## Background & Motivation

Todoscript currently tracks task staleness by automatically adding visual markers (parentheses) to tasks that have been sitting in Todoist. This provides visual pressure to complete or remove stale tasks. However, users must actively open Todoist to see these indicators. 

Adding Discord notifications would:
1. Proactively notify users of pending tasks at the start of each day
2. Provide persistent reminders for critical overdue tasks
3. Increase task visibility and accountability
4. Leverage Discord as a platform many users already have open throughout the day

## User Requirements

### Morning Task Notifications
- Send a daily summary of pending tasks to a configured Discord channel
- Include task content, age indicators (parentheses count), and due dates if available
- Configurable timing (default: start of workday, e.g., 9:00 AM local time)
- Group tasks by project/priority for better organization
- Include summary statistics (total pending tasks, oldest task, etc.)

### Task Nagging
- Allow individual tasks to be tagged with #nagme to enable recurring notifications
- Send reminder notifications at configurable intervals (default: every 30 minutes after due date)
- Include task content, age, and time overdue
- Support configurable quiet hours to avoid disturbing users during off-hours
- Provide a mechanism to temporarily snooze nagging for specific tasks

## Technical Design

### Components

#### 1. Discord Integration Module
- Discord webhook client for sending messages
- Message formatting and rendering
- Rate limiting and error handling

#### 2. Task Scheduling Engine
- Schedule management for both morning summaries and recurring nags
- Time zone aware scheduling
- Configurable intervals and timing

#### 3. Task Selection & Filtering
- Filter tasks eligible for morning summaries
- Identify and track tasks with #nagme tag
- Apply user configuration preferences

#### 4. Configuration Management
- Discord webhook URL and channel configuration
- Notification timing and frequency settings
- User preferences and quiet hours

### System Architecture

The Discord integration will operate as part of the existing Todoscript application with these additions:

1. **Configuration**
   ```
   DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..."
   MORNING_NOTIFICATION_TIME="09:00"
   TIMEZONE="America/Los_Angeles"
   NAG_INTERVAL_MINUTES=30
   QUIET_HOURS_START="22:00"
   QUIET_HOURS_END="07:00"
   ```

2. **Discord Client**
   - HTTP client for sending webhook requests
   - Message templating for consistent formatting
   - Error handling and retries

3. **Scheduler**
   - Background process to track scheduled notifications
   - Time-based triggers for morning summaries and nags
   - Task tracking to manage nagging state

4. **Core Integration Points**
   - Hook into existing task processing flow
   - Share authentication and API client with main application
   - Leverage existing task metadata for consistency

### Implementation Plan

#### Phase 1: Core Discord Integration
- Set up Discord webhook communication
- Implement basic message formatting
- Create configuration structure
- Add simple notification capability

#### Phase 2: Morning Task Summary
- Implement daily task aggregation
- Design summary message format
- Add scheduled delivery
- Support task grouping and statistics

#### Phase 3: Task Nagging
- Implement #nagme tag detection
- Add recurring notification scheduling
- Create nagging state management
- Support quiet hours and snoozing

## API Requirements

### Discord Webhook API
- `POST` requests to Discord webhook URL
- Support for embedded messages with formatting
- Rate limit compliance (5 requests per 2 seconds per webhook)

### Todoist API Extensions
- Additional metadata to track nagging state
- Tag detection for #nagme tasks
- No changes to existing API usage patterns

## Security Considerations

1. **Webhook URL Protection**
   - Store webhook URL securely
   - Support rotation of webhook URLs
   - No sensitive task data in plain notifications

2. **Information Disclosure**
   - Consider privacy implications of tasks in Discord channels
   - Option to obscure sensitive task details
   - Clear documentation on data sharing

## Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `DISCORD_WEBHOOK_URL` | Discord webhook URL | Required |
| `DISCORD_ENABLED` | Enable/disable Discord integration | `false` |
| `MORNING_NOTIFICATION_ENABLED` | Enable daily morning summaries | `true` when Discord enabled |
| `MORNING_NOTIFICATION_TIME` | Time for daily summary (HH:MM) | `09:00` |
| `NAG_ENABLED` | Enable nagging for #nagme tasks | `true` when Discord enabled |
| `NAG_INTERVAL_MINUTES` | Minutes between nag reminders | `30` |
| `QUIET_HOURS_ENABLED` | Enable quiet hours | `true` |
| `QUIET_HOURS_START` | Start of quiet hours (HH:MM) | `22:00` |
| `QUIET_HOURS_END` | End of quiet hours (HH:MM) | `07:00` |
| `MAX_TASKS_PER_SUMMARY` | Maximum tasks in morning summary | `25` |
| `TASK_SUMMARY_GROUPING` | How to group tasks in summary | `project` |

## Operational Considerations

1. **Execution Frequency**
   - The application must run at higher frequency (e.g., every 15 minutes) to support nagging
   - Alternative: Split into two processes (daily aging + high-frequency notifications)

2. **Rate Limiting**
   - Respect Discord's rate limits
   - Implement exponential backoff for retries
   - Queue notifications during high volume

3. **Error Handling**
   - Graceful recovery from Discord API failures
   - Logging for notification delivery issues
   - Retry mechanism for transient errors

## Future Extensions

1. **Interactive Controls**
   - Add Discord buttons to complete or snooze tasks
   - Support for task comments via Discord
   - Real-time task status updates

2. **Rich Formatting**
   - Custom embeds with task metadata
   - Color coding based on priority or age
   - Progress bars and visual indicators

3. **Advanced Scheduling**
   - User-defined notification schedules
   - Custom nagging rules beyond fixed intervals
   - Calendar-aware notifications (e.g., don't nag during meetings)

## Open Questions

1. Should we support multiple Discord channels for different task categories?
2. How should we handle tasks that are completed between scheduled notifications?
3. What's the appropriate balance between helpful reminders and notification fatigue?
4. Should we add authentication for Discord interactions beyond webhook security?

## Success Metrics

1. **Engagement Metrics**
   - Reduction in task staleness after implementing Discord notifications
   - Completion rate of tasks with #nagme tag
   - User interaction with notifications

2. **System Health**
   - Notification delivery success rate
   - System resource usage with increased execution frequency
   - API rate limit compliance

## References

1. Discord Webhook API Documentation: https://discord.com/developers/docs/resources/webhook
2. Todoist REST API Documentation: https://developer.todoist.com/rest/v2
3. Existing Todoscript architecture and design patterns