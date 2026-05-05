---
description: Global coding rules and tool usage discipline
tags: [always-on, coding, quality]
---

# Always-On Guidance

Rules that apply to every coding session.

## Tool Usage Discipline

- Read files before editing — never guess at content
- Use `org_*` tools for work item management
- Use `comms_*` tools for agent messaging and file reservations
- Use `forge_*` tools for multi-agent coordination
- Use `hivemind_*` tools for learning storage and retrieval
- Check `hivemind_find` before solving problems from scratch

## Code Quality

- Functions do one thing well
- Names reveal intent — no abbreviations
- Comments explain *why*, not *what*
- No dead code or unused imports
- Error messages include context

## Testing

- Write tests for all new code
- Use `db.OpenMemory()` for database tests
- Use `t.TempDir()` for filesystem tests
- Standard library `testing` package only — no testify
- Test names: `TestXxx_Description`

## Error Handling

- Return errors, don't panic
- Wrap errors with context: `fmt.Errorf("operation: %w", err)`
- Handle all error paths — no ignored returns
- Use `errors.Is` for sentinel error checks

## Git Discipline

- Conventional commits: `type: description`
- Never force push to main
- Commit early, commit often
