---
description: >
  Invoke the Cobalt-Crush developer persona for implementation tasks.
  With arguments: delegates to the cobalt-crush-dev agent. Without
  arguments: detects active workflow and runs /speckit.implement or
  /opsx-apply.
---
<!-- scaffolded by uf vdev -->

# Command: /cobalt-crush

## Description

Invoke the Cobalt-Crush developer persona to implement code with
clean code principles, convention pack adherence, and feedback loop
integration.

## User Input

```text
$ARGUMENTS
```

## Usage

```
/cobalt-crush [task description or instructions]
/cobalt-crush                    # auto-detect workflow and implement
```

### Examples

```
/cobalt-crush implement T015 from tasks.md
/cobalt-crush fix the failing test in internal/metrics/store_test.go
/cobalt-crush refactor runMetrics to reduce complexity
/cobalt-crush                    # detects speckit or openspec context
```

## Instructions

### When arguments are provided

Delegate to the `cobalt-crush-dev` agent using the Task tool with
`subagent_type: "cobalt-crush-dev"`. Pass the user's arguments as
the task prompt. The agent will:

1. Read AGENTS.md, constitution, convention packs, and active specs
2. Apply its engineering philosophy (clean code, SOLID, TDD awareness)
3. Load convention packs from `.opencode/uf/packs/`
4. Check `.uf/artifacts/` for Gaze/Divisor feedback
5. Execute the requested task following project conventions
6. Document design decisions in code comments

### When no arguments are provided

Detect the active workflow and delegate the corresponding
implementation command to the `cobalt-crush-dev` agent:

1. **Check for a Speckit feature branch**: Run
   `.specify/scripts/bash/check-prerequisites.sh --json --paths-only`
   from the repo root. If it succeeds and returns a `FEATURE_DIR`
   with a `tasks.md`, this is a Speckit strategic workflow.
   Read the full contents of `.opencode/commands/speckit.implement.md`
   and delegate it to the `cobalt-crush-dev` agent via the Task
   tool — pass the command file's instructions as the agent's
   prompt so the agent executes the implementation workflow.

2. **Check for an OpenSpec active change**: Look for directories
   under `openspec/changes/` that contain a `tasks.md` file. If
   one exists, this is an OpenSpec tactical workflow.
   **Validate branch**: Run `git rev-parse --abbrev-ref HEAD`.
   The current branch must be `opsx/<change-name>` where
   `<change-name>` matches the detected change directory name.
   If not on the correct branch, **STOP** with error:
   > "OpenSpec change `<name>` detected but you are on branch
   > `<current-branch>`. Run: `git checkout opsx/<name>`"

   If on the correct branch, read the full contents of
   `.opencode/commands/opsx-apply.md` and delegate it to the
   `cobalt-crush-dev` agent via the Task tool — pass the command
   file's instructions as the agent's prompt so the agent
   executes the apply workflow.

3. **If neither is detected**: Ask the user which workflow to run:

   > No active implementation context detected. Which workflow
   > should I execute?
   >
   > - `/unleash` — Autonomous pipeline (parallel swarm,
   >   recommended for multi-task changes)
   > - `/speckit.implement` — Strategic spec implementation
   >   (requires a feature branch with `specs/NNN-*/tasks.md`)
   > - `/opsx-apply` — Tactical change implementation
   >   (requires an active change in `openspec/changes/`)

## Branch Safety Guardrails

**CRITICAL**: Before switching branches or suggesting a branch
switch for any reason:

1. Run `git status --short` to check for uncommitted changes.
2. If uncommitted changes exist, **STOP** and warn the user.
   Show the list of uncommitted files and ask for confirmation
   before proceeding.
3. Never silently switch branches with a dirty working tree --
   uncommitted changes may follow to the wrong branch or be
   lost entirely.
4. When implementation is complete, all changes MUST be
   committed and pushed on the current branch before suggesting
   any branch switch, PR creation, or archiving.
