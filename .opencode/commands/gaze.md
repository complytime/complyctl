---
description: >
  Run Gaze quality analysis on a Go package. Supports three modes:
  full (default), crap (CRAP scores only), and quality (test quality
  metrics only). Delegates to the gaze-reporter agent.
agent: gaze-reporter
---
<!-- scaffolded by gaze dev -->

# Command: /gaze

## Description

Run Gaze quality analysis and produce a human-readable report.
Delegates to the `gaze-reporter` agent which runs the appropriate
`gaze` CLI commands and interprets the JSON output.

## Usage

```
/gaze [mode] [package-pattern]
```

### Modes

| Mode | Description |
|------|-------------|
| (none) | Full report: CRAP + quality + classification + health assessment |
| `crap` | CRAP scores only |
| `quality` | Test quality metrics only |

### Examples

```
/gaze ./...                     # Full report on entire module
/gaze crap ./internal/store     # CRAP scores for one package
/gaze quality ./pkg/api         # Test quality for one package
/gaze crap                      # CRAP scores for ./... (default)
/gaze                           # Full report for ./... (default)
```

## Instructions

Pass `$ARGUMENTS` to the `gaze-reporter` agent. The agent handles
mode parsing, binary resolution, command execution, and report
formatting.

If no arguments are provided, the agent defaults to full mode with
the package pattern `./...`.
