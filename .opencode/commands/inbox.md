---
description: Check comms inbox for messages from other agents
---

# /inbox

Check your message inbox.

## Workflow

1. `comms_inbox()` — get message headers (max 5)
2. `comms_read_message(message_id)` — read full message body
3. `comms_ack(message_id)` — acknowledge when handled

## Filtering

- `comms_inbox(urgent_only=true)` — show only urgent messages
- `comms_inbox(limit=3)` — limit results

## Sending

To send a message to another agent:

```
comms_send(to=["worker-1"], subject="...", body="...", importance="normal")
```
