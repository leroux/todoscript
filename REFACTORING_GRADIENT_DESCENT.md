# Iterative Code Improvement Plan

## Overview
A practical approach to continuously improve code quality through small, measurable changes until no obvious improvements remain.

## Why This Approach?
- **Small Changes**: Easier to review and less risky
- **Measurable**: Can see concrete improvements
- **Iterative**: Stop when improvements become marginal
- **Human-Controlled**: You approve every change

## Quality Assessment Framework

### High-Impact Areas (Address First)
- [ ] Error handling follows consistent patterns
- [ ] Functions have clear, single purposes
- [ ] Code is logically organized and grouped
- [ ] No obvious duplication without good reason
- [ ] Complex logic is properly documented

### Assessment Questions
1. **Can a new developer understand this code quickly?**
2. **Are error messages helpful for debugging?**
3. **Would changing one thing require changes in multiple places?**
4. **Is the code's intent clear from reading it?**
5. **Are there any "WTF" moments when reading?**

### Stopping Criteria
- All high-impact checklist items completed
- No obvious improvements that would significantly help maintenance
- Diminishing returns on time invested

## Improvement Process

### Simple 3-Step Cycle
1. **Identify**: Claude finds the next highest-impact improvement
2. **Propose**: Claude shows you exactly what will change
3. **Implement**: You approve, Claude makes the change

### Safety
- Git commit before each change
- Test compilation after each change
- You can reject any proposal

### Stopping Criteria
- All high-impact checklist items completed
- No obvious improvements that would significantly help maintenance
- Diminishing returns on time invested

## Common Refactoring Patterns

### Error Handling Standardization
- **Target**: Inconsistent error messages and patterns
- **Action**: Standardize format and improve consistency
- **Example**: `"failed to X: %w"` → `"X failed: %w"`

### Constants Extraction
- **Target**: Magic numbers and hardcoded strings
- **Action**: Extract into named constants with clear organization
- **Example**: Replace `30` with `httpTimeoutSeconds = 30`

### HTTP Operations Consolidation
- **Target**: Duplicate HTTP request/response handling
- **Action**: Create helper functions for common patterns
- **Example**: `makeAuthenticatedRequest()`, `getTodoistData()`

### Function Naming & Documentation
- **Target**: Unclear function names and missing documentation
- **Action**: Rename for clarity and add comprehensive comments
- **Example**: `extractParenthesesCount` → `parseTaskAgeMarkers`

### Variable Naming Clarity
- **Target**: Confusing or cryptic variable names
- **Action**: Use self-documenting names
- **Example**: `recentTaskMap` → `tasksByContent`

### Return Value Simplification
- **Target**: Complex multi-return functions
- **Action**: Create clear struct-based returns
- **Example**: `(int, string, bool)` → `TaskAgeInfo{...}`

### Parameter Simplification
- **Target**: Long parameter lists
- **Action**: Group related parameters into context structs
- **Example**: `TaskContext{Task, IsRecurring, DaysSinceCompletion, Timezone}`

## Quality Improvement Areas

### Code Structure
- **Error Handling**: Uniform patterns and helpful messages
- **Function Design**: Clear, single-purpose functions with good names
- **Code Organization**: Logical grouping and flow
- **Duplication**: DRY principle without over-abstraction

### Readability
- **Variable Naming**: Self-documenting names
- **Function Documentation**: Clear purpose and usage
- **Return Values**: Structured, meaningful returns
- **Function Signatures**: Manageable parameter lists

## Ready to Start?

### Prerequisites
- Identify target codebase for improvement
- Establish quality priorities for the specific project
- Set up git repository for tracking changes
- Define stopping criteria based on project needs

### Next Steps
1. Analyze current codebase for improvement opportunities
2. Rank opportunities by impact and effort
3. Begin iterative improvement cycle
4. Continue until convergence criteria met