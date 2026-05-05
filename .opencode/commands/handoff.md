---
description: End a session cleanly - release reservations, sync state, generate handoff note
---

# /handoff

Wrap up a session cleanly.

## Workflow

1. Summarize completed work and open blockers
2. `comms_release_all()` — free all file reservations
3. `org_update()` / `org_close()` — update cell statuses
4. `org_sync()` — persist state to git
5. `org_session_end(handoff_notes="...")` — save handoff for next session

## Handoff Note Template

Include in your handoff notes:

- **Completed**: What tasks were finished
- **In Progress**: What was started but not finished
- **Blocked**: What is waiting on external input
- **Next Steps**: What the next agent should do first
- **Gotchas**: Any surprises or edge cases discovered
