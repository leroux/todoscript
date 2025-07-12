# Post-Claude Cleanup Checklist

This document outlines cleanup tasks to perform after Claude coding sessions.

## Code Quality
- [ ] Run linter and fix any issues
- [ ] Run type checker and resolve type errors
- [ ] Ensure all tests pass
- [ ] Remove any debugging code or console.logs
- [ ] Review and clean up any temporary/experimental code
- [ ] Remove obvious/redundant comments (e.g., "// we already know 24 is hours in day")
- [ ] Clean up legacy comments that no longer apply
- [ ] Remove TODO comments that have been addressed
- [ ] Review inline comments for clarity and necessity

## Documentation
- [ ] Update relevant documentation for new features
- [ ] Add or update code comments where necessary
- [ ] Update CHANGELOG if applicable

## Dependencies
- [ ] Review any new dependencies added
- [ ] Remove unused dependencies
- [ ] Update package versions if needed

## Security
- [ ] Ensure no secrets or sensitive data are committed
- [ ] Review any new external integrations
- [ ] Check for security vulnerabilities in new code

## Git Hygiene
- [ ] Review commit messages for clarity
- [ ] Squash commits if needed for cleaner history
- [ ] Ensure proper branch naming conventions

## Performance
- [ ] Profile any performance-critical changes
- [ ] Check for memory leaks in new code
- [ ] Optimize any inefficient implementations

## Cleanup
- [ ] Remove any files from scratch directory if no longer needed
- [ ] Clean up any temporary files or artifacts
- [ ] Ensure working directory is clean