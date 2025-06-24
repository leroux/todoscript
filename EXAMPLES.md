# Todoscript Examples

This document provides practical examples of how todoscript's staleness tracking feature works in real-world scenarios.

## Task Staleness Tracking Examples

Todoscript uses the number of parentheses in a task name to visually indicate how stale a task is. Tasks tagged with `@auto` will automatically have their parentheses count updated based on the rules below.

### Non-Recurring Task Example

For non-recurring tasks, the parentheses count increases daily (when the script runs) to indicate growing staleness:

**Task Setup:**
- Create a task in Todoist: `25) Submit quarterly tax report @auto`
- Make sure it's not set as recurring

**Daily Progression:**

| Day | Task Content | Action | Explanation |
|-----|-------------|--------|-------------|
| 0 | `25) Submit quarterly tax report` | Initial state | Task is created |
| 1 | `25)) Submit quarterly tax report` | Script runs | One parenthesis added to show 1 day of staleness |
| 2 | `25))) Submit quarterly tax report` | Script runs | Another parenthesis added (2 days stale) |
| 3 | `25)))) Submit quarterly tax report` | Script runs | Another parenthesis added (3 days stale) |
| 4 | `25))))) Submit quarterly tax report` | Script runs | Another parenthesis added (4 days stale) |
| 5 | Task completed | User completes | User marks task as complete, removing it from active tasks |

The increasing number of parentheses provides a visual reminder of how long the task has been waiting for completion.

### Recurring Task Example

For recurring tasks, the parentheses count increases daily but resets when the task is completed:

**Task Setup:**
- Create a recurring task: `10) Daily standup meeting @auto`
- Set it as recurring daily in Todoist

**Daily Progression:**

| Day | Task Content | Action | Explanation |
|-----|-------------|--------|-------------|
| Mon | `10) Daily standup meeting` | Initial state | Recurring task is created |
| Tue | `10)) Daily standup meeting` | Script runs | Task wasn't completed Monday, one parenthesis added |
| Wed | `10))) Daily standup meeting` | Script runs | Task wasn't completed Tuesday either, another parenthesis added |
| Thu | `10) Daily standup meeting` | User completes + Script runs | User completes the task on Wednesday, Todoist creates a new instance, and when the script runs it detects completion via the Activity Log API and resets to 1 parenthesis |
| Fri | `10)) Daily standup meeting` | Script runs | Task wasn't completed Thursday, one parenthesis added |

The reset to a single parenthesis after completion makes it easy to spot which recurring tasks have been recently completed and which are becoming stale.

## Notes About Todoscript Behavior

1. **24-Hour Update Limit**: Tasks will only have their parentheses incremented once per 24 hours.
2. **Reset Priority**: The reset action (when a recurring task is completed) happens regardless of when the task was last updated.
3. **Metadata Storage**: Todoscript stores last update information in the task description.
4. **Task Filtering**: Only tasks tagged with `@auto` are processed.
5. **Dry Run Mode**: You can preview changes using `DRY_RUN=true ./todoscript` before applying them.

## Advanced: Manual Testing of Reset Functionality

If you'd like to test the reset functionality on a recurring task:

1. Create a recurring task with the pattern: `01) Test recurring task @auto`
2. Run todoscript once: `./todoscript`
   - The task should now show: `01)) Test recurring task`
3. Complete the task in Todoist
4. Run todoscript again
   - The task should reset to: `01) Test recurring task`