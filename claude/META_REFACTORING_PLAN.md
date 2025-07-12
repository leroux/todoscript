# Meta-Refactoring Plan Generation

## Overview
A gradient descent approach to iteratively generate and improve a REFACTORING_PLAN.md document until it reaches optimal quality for guiding code improvements.

## Why This Meta-Approach?
- **Iterative Refinement**: Plans improve through systematic iteration
- **Quality Convergence**: Stop when plan quality plateaus
- **Measurable Progress**: Track plan quality improvements
- **Human Oversight**: You approve each plan iteration

## Plan Quality Assessment Framework

### High-Impact Plan Qualities (Address First)
- [ ] Plan clearly identifies specific improvement targets
- [ ] Implementation steps are concrete and actionable
- [ ] Quality metrics are well-defined and measurable
- [ ] Stopping criteria are clear and achievable
- [ ] Plan scope is appropriate for codebase size/complexity

### Plan Assessment Questions
1. **Would this plan actually improve code quality?**
2. **Are the steps specific enough to execute without ambiguity?**
3. **Do the quality metrics align with the project's needs?**
4. **Are the stopping criteria realistic and observable?**
5. **Does the plan balance thoroughness with practicality?**

### Plan Quality Metrics
- **Specificity Score**: How concrete are the improvement targets? (1-10)
- **Actionability Score**: How executable are the steps? (1-10)
- **Coverage Score**: How well does it cover key quality dimensions? (1-10)
- **Clarity Score**: How understandable is the plan? (1-10)
- **Completeness Score**: Does it have all necessary components? (1-10)

### Convergence Criteria
- All high-impact qualities achieved
- Quality metrics plateau (< 0.5 point improvement across all metrics)
- Maximum iterations reached (default: 10)
- Human satisfaction with plan quality

## Meta-Improvement Process

### 4-Step Meta-Cycle
1. **Generate**: Create or refine REFACTORING_PLAN.md based on codebase analysis
2. **Assess**: Evaluate plan quality using metrics and assessment questions
3. **Identify**: Find specific weaknesses in current plan iteration
4. **Iterate**: Improve plan based on identified weaknesses

### Plan Generation Instructions
```
PROMPT: Generate a REFACTORING_PLAN.md that:
1. Analyzes the current codebase for improvement opportunities
2. Prioritizes improvements by impact and effort
3. Defines concrete, measurable quality targets
4. Establishes clear stopping criteria
5. Provides step-by-step implementation guidance
6. Includes quality assessment checkpoints

Consider:
- Codebase size and complexity
- Current code quality baseline
- Available development time
- Risk tolerance for changes
- Specific quality pain points
```

### Safety and Tracking
- Git commit each plan iteration with quality scores
- Document reasoning for each plan change
- Track quality metric progression
- Preserve rejected plan iterations for reference

## Plan Improvement Patterns

### Specificity Enhancement
- **Target**: Vague improvement goals
- **Action**: Define concrete, measurable targets
- **Example**: "Improve error handling" → "Standardize error messages to format 'operation failed: %w'"

### Actionability Boost
- **Target**: Abstract improvement steps
- **Action**: Break down into specific actions
- **Example**: "Clean up functions" → "Extract functions > 50 lines, rename unclear functions, add docstrings"

### Quality Metric Refinement
- **Target**: Unmeasurable success criteria
- **Action**: Define quantitative and qualitative measures
- **Example**: "Better code" → "Cyclomatic complexity < 10, function length < 30 lines"

### Scope Optimization
- **Target**: Plans too broad or narrow for codebase
- **Action**: Adjust scope based on codebase analysis
- **Example**: Single-file projects need simpler plans than multi-module systems

### Priority Clarification
- **Target**: Unclear improvement ordering
- **Action**: Rank by impact/effort matrix
- **Example**: High-impact, low-effort improvements first

## Meta-Quality Dimensions

### Plan Structure
- **Goal Definition**: Clear, measurable objectives
- **Step Sequencing**: Logical, dependency-aware ordering
- **Quality Gates**: Checkpoints for progress validation
- **Risk Mitigation**: Backup and rollback strategies

### Plan Content
- **Codebase Awareness**: Specific to actual code issues
- **Tool Integration**: Leverages available development tools
- **Time Estimation**: Realistic effort estimates
- **Success Metrics**: Observable completion criteria

## Ready to Generate?

### Prerequisites
- Identify target codebase for refactoring plan
- Understand current code quality baseline
- Define project-specific quality priorities
- Set meta-iteration limits and convergence thresholds

### Meta-Process Execution
1. **Initial Generation**: Create REFACTORING_PLAN.md v1.0
2. **Quality Assessment**: Score against all metrics
3. **Gap Analysis**: Identify specific improvement areas
4. **Plan Iteration**: Generate improved version
5. **Convergence Check**: Evaluate if stopping criteria met
6. **Repeat or Execute**: Continue meta-cycle or run final plan

### Final Plan Validation
- Plan quality scores all > 8.0
- Human review and approval
- Alignment with project goals
- Realistic implementation timeline
- Clear success measures defined