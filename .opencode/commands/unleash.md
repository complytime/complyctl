---
description: >
  Run the full Speckit or OpenSpec pipeline autonomously:
  clarify ambiguities with Dewey, generate plan and tasks,
  review specs, implement with parallel Swarm workers,
  review code, store learnings, and present demo
  instructions. Exits to the human only when it genuinely
  needs human judgment.
---
<!-- scaffolded by uf vdev -->

# Command: /unleash

## Description

Autonomous pipeline execution for both Speckit (strategic)
and OpenSpec (tactical) changes. Takes a spec from draft
to demo-ready code in a single command. Orchestrates 8
steps: clarify, plan, tasks, spec review, implement, code
review, retrospective, and demo. Exits gracefully when
human judgment is needed and resumes from where it left
off on re-run.

## Usage

```
/unleash
```

## Instructions

### 0. Startup Cleanup

Before any pipeline logic, clean up stale worktrees from
previous interrupted runs:

1. Call `swarm_worktree_list` with the project path.
2. If any worktrees exist, call `swarm_worktree_cleanup`
   with `cleanup_all: true` to remove them.
3. This cleanup does NOT affect resumability -- spec.md,
   plan.md, tasks.md, and task checkboxes live in the main
   working directory, not in worktrees.

If `swarm_worktree_list` is not available (Replicator
not installed), skip this step silently.

### 1. Branch Safety Gate

Get the current branch:

```bash
git rev-parse --abbrev-ref HEAD
```

- If on `main`: **STOP** with error:
  > "Cannot run /unleash on main. Must be on a Speckit
  > (`NNN-*`) or OpenSpec (`opsx/*`) feature branch."

- If on `opsx/*`: **OpenSpec mode detected.**
  Extract the change name from the branch:
  `opsx/<name>` → `<name>`.
  Set `FEATURE_DIR = openspec/changes/<name>/`.
  Set `WORKFLOW_TIER = openspec`.

  Check if `FEATURE_DIR/tasks.md` exists. If not:
  **STOP** with error:
  > "No tasks.md found for change `<name>`. Run
  > `/opsx-propose` first."

  Announce: "Detected OpenSpec change: `<name>`"

- If the branch matches `NNN-*` (digits followed by a
  dash): **Speckit mode detected.**

  Validate that spec.md exists by running from the repo
  root:

  ```bash
  .specify/scripts/bash/check-prerequisites.sh --json --require-spec
  ```

  If the script fails or no spec.md is found: **STOP**
  with error:
  > "No spec.md found in the feature directory. Run
  > `/speckit.specify` first."

  Extract the `FEATURE_DIR` from the JSON output. This
  is the working directory for all subsequent steps.
  Set `WORKFLOW_TIER = speckit`.

- If the branch does not match `NNN-*` or `opsx/*`:
  **STOP** with error:
  > "Unrecognized branch pattern. /unleash requires a
  > Speckit feature branch (`NNN-*`) or OpenSpec branch
  > (`opsx/*`). Run `/speckit.specify` or
  > `/opsx-propose` to create one."

### 2. Resumability Detection

Probe filesystem state to determine which steps are
already complete. Check in order:

**If `WORKFLOW_TIER = openspec`**: checks 1-3 are always
"done" — these artifacts were created by `/opsx-propose`.
Skip directly to check 4.

1. **Clarify done?** *(Speckit only)* Read spec.md in
   the feature directory. Check for
   `[NEEDS CLARIFICATION]` markers.
   - If NO markers exist AND (a `## Clarifications`
     section exists in spec.md OR plan.md exists in the
     feature directory): clarify is done.
   - Otherwise: clarify is needed.

2. **Plan done?** *(Speckit only)* Check if `plan.md`
   exists in the feature directory.

3. **Tasks done?** *(Speckit only)* Check if `tasks.md`
   exists in the feature directory.

4. **Spec review done?** Read `FEATURE_DIR/tasks.md`
   and check for the `<!-- spec-review: passed -->` HTML
   comment marker.

5. **Implementation done?** Read `FEATURE_DIR/tasks.md`
   and check if all task checkboxes are `[x]` (no
   `- [ ]` lines remain in the task phases).

6. **Code review done?** Read `FEATURE_DIR/tasks.md`
   and check for the `<!-- code-review: passed -->` HTML
   comment marker.

Display the detection results. For OpenSpec mode:

```
Detected: OpenSpec mode — artifacts from /opsx-propose
  clarify ✓ (skipped)  plan ✓ (skipped)  tasks ✓ (skipped)
  spec-review [status]  implement [status]  code-review [status]
```

For Speckit mode:

```
Detected: clarify ✓ plan ✓ tasks ✓ spec-review ✗
Resuming at step 4/8: Reviewing specs...
```

Skip all steps that are detected as complete. Resume
from the first incomplete step.

If ALL steps are complete (all tasks done, tests pass):
skip directly to step 7 (retrospective) since it is
idempotent.

### 3. Step 1 -- Clarify

**If `WORKFLOW_TIER = openspec`**: skip this step.
Announce: "OpenSpec mode — clarify handled by
/opsx-propose."

Scan spec.md in the feature directory for
`[NEEDS CLARIFICATION]` markers.

**If no markers exist**: announce "No clarification
needed" and proceed to step 2.

**If markers exist**: for each marker:

1. Extract the question text from the marker (e.g.,
   `[NEEDS CLARIFICATION: What auth provider?]`).
2. Read 3-5 surrounding lines for context.
3. Formulate a targeted Dewey semantic search query by
   combining the question's topic keywords with the
   project domain (from the spec title and description).
   Do NOT use the raw `[NEEDS CLARIFICATION]` text
   verbatim as the query.

4. **If Dewey is available** (`dewey_semantic_search`
   tool exists): call `dewey_semantic_search` with the
   formulated query.
   - Evaluate the results using your judgment. If the
     results sufficiently answer the question:
     - Remove the `[NEEDS CLARIFICATION: ...]` marker
       from the spec text.
     - Write the answer inline where the marker was.
     - Add an entry to the `## Clarifications` section
       at the bottom of spec.md with the format:
       ```
       - Q: [original question] -> A: [answer]
         (Dewey-resolved from [page name or block UUID])
       ```
     - This is a silent auto-resolution -- do NOT ask
       the human for confirmation.
   - If the results are empty, off-topic, or do not
     sufficiently answer the question: add the question
     to the unanswerable list.

5. **If Dewey is NOT available**: add ALL questions to
   the unanswerable list. Note:
   > "Dewey not available -- all clarification questions
   > require human input."

**After processing all markers**:

- If the unanswerable list is empty: all questions were
  auto-resolved. Announce how many were resolved and
  proceed to step 2.
- If the unanswerable list is NOT empty: **EXIT** with
  all unanswerable questions presented at once:

  ```
  ## /unleash paused at: clarify

  **Reason**: N question(s) require human input

  Q1: [question text]
  Context: [surrounding spec lines]

  Q2: [question text]
  Context: [surrounding spec lines]

  ### What to do next
  Answer these questions in the spec, then re-run
  `/unleash`.

  ### Then resume
  Run `/unleash` to continue from the plan step.
  ```

### 4. Step 2 -- Plan

**If `WORKFLOW_TIER = openspec`**: skip this step.
Announce: "OpenSpec mode — plan handled by
/opsx-propose."

Generate the implementation plan by delegating to the
`cobalt-crush-dev` agent.

1. Read the full contents of
   `.opencode/commands/speckit.plan.md`.
2. Delegate to the `cobalt-crush-dev` agent via the Task
   tool, passing the plan command's instructions as the
   agent's prompt. Include the feature directory path so
   the agent knows where to write `plan.md`.
3. After the agent completes, verify that `plan.md` was
   created in the feature directory.
4. If `plan.md` was NOT created: **STOP** with error:
   > "Plan generation failed -- plan.md was not created.
   > Check the agent output for errors."

### 5. Step 3 -- Tasks

**If `WORKFLOW_TIER = openspec`**: skip this step.
Announce: "OpenSpec mode — tasks handled by
/opsx-propose."

Generate the task list by delegating to the
`cobalt-crush-dev` agent.

1. Read the full contents of
   `.opencode/commands/speckit.tasks.md`.
2. Delegate to the `cobalt-crush-dev` agent via the Task
   tool, passing the tasks command's instructions as the
   agent's prompt. Include the feature directory path.
3. After the agent completes, verify that `tasks.md` was
   created in the feature directory.
4. If `tasks.md` was NOT created: **STOP** with error:
   > "Task generation failed -- tasks.md was not created.
   > Check the agent output for errors."

### 6. Step 4 -- Spec Review

Review the spec artifacts using the review council in
spec review mode. This step subsumes `/speckit.analyze`
and `/speckit.checklist` -- the review council's 5
Divisor agents provide equivalent coverage (consistency
analysis + quality validation) in a single pass.

1. Read the full contents of
   `.opencode/commands/review-council.md`.
2. Delegate to the `cobalt-crush-dev` agent via the Task
   tool with the review council's instructions, adding
   the explicit mode override: "Run in **Spec Review
   Mode** -- review the spec artifacts in `FEATURE_DIR`,
   not code." The review council auto-detects the
   workflow tier from the branch name (`opsx/*` vs
   `NNN-*`).
3. Collect the review results.

**Processing results**:

- If all reviewers APPROVE: write the spec review marker
  to `FEATURE_DIR/tasks.md`:
  ```
  <!-- spec-review: passed -->
  ```
  Append this marker at the very end of
  `FEATURE_DIR/tasks.md`. Proceed to step 5.

- If only LOW and MEDIUM findings exist: auto-fix them
  (the review council handles this per its hybrid fix
  policy). After fixes, re-run the review. If all
  APPROVE after fixes, write the marker and proceed.

- If HIGH or CRITICAL findings remain after auto-fixing
  LOW/MEDIUM: **EXIT** with the findings:

  ```
  ## /unleash paused at: spec review

  **Reason**: HIGH/CRITICAL findings in spec artifacts

  ### Findings

  [list each HIGH/CRITICAL finding with context]

  ### What to do next
  Run `/speckit.clarify` to address the findings, then
  re-run `/unleash`.

  ### Then resume
  Run `/unleash` to continue from spec review.
  ```

### 7. Step 5 -- Implement

Parse `FEATURE_DIR/tasks.md` for phases and execute
tasks.

**Derive build/test commands**: Read all files in
`.github/workflows/` to identify the exact CI commands
(build, test, vet, lint). Do NOT hardcode language-
specific commands -- the workflow files are the source
of truth. This is the same pattern used by
`/review-council` Phase 1a.

**For each phase in tasks.md**:

1. Separate tasks into two groups:
   - **Sequential** (no `[P]` marker): run first, in
     order
   - **Parallel** (`[P]` marker): run after sequential
     tasks complete

2. **Sequential execution**: for each non-`[P]` task,
   delegate to the `cobalt-crush-dev` agent via the Task
   tool with the task description as the prompt. After
   the agent completes, mark the task `[x]` in
   `tasks.md` immediately.

3. **Parallel execution**: check if
   `swarm_worktree_create` is available.

   **If Swarm worktrees are available**:

   a. Get the current commit hash:
      ```bash
      git rev-parse HEAD
      ```

   b. Limit concurrent workers to 4. If more than 4
      `[P]` tasks exist in the phase, batch them: run
      the first 4, wait for all to complete, then run
      the next batch.

   c. For each `[P]` task in the current batch:
      - Call `swarm_worktree_create` with the project
        path, task ID, and current commit hash to create
        a dedicated worktree.
      - Call `swarm_spawn_subtask` with the task
        description, files from the task, and the
        worktree path.

   d. Wait for all workers in the batch to complete.

   e. **If any worker fails**: stop spawning new workers
      (do not start the next batch). Wait for any
      already-running workers to complete or fail. Then
      call `swarm_worktree_cleanup` with `cleanup_all:
      true` to remove all worktrees. **EXIT** with error
      context:

      ```
      ## /unleash paused at: implement (Phase N)

      **Reason**: Parallel worker failed

      **Failed task**: [task ID and description]
      **Error**: [error details from worker]

      ### What to do next
      Fix the issue described above, then re-run
      `/unleash`.

      ### Then resume
      Run `/unleash` to continue from the failed phase.
      ```

   f. After all workers in a batch complete successfully,
      merge each worktree back:
      - Call `swarm_worktree_merge` for each worktree.
        This uses cherry-pick to apply the worker's
        commits to the main branch.
      - After each merge, check for conflict markers
        (`<<<<<<<`, `=======`, `>>>>>>>`) in the
        affected files.
      - If NO conflict markers remain: merge succeeded.
        Call `swarm_worktree_cleanup` for that worktree.
      - If conflict markers remain: auto-resolution
        failed. Call `swarm_worktree_cleanup` with
        `cleanup_all: true`. **EXIT** with conflict
        details:

        ```
        ## /unleash paused at: implement (Phase N)

        **Reason**: Worktree merge conflict

        **Conflicting files**:
        [list files with conflict markers]

        ### What to do next
        Resolve the merge conflicts manually, then
        re-run `/unleash`.

        ### Then resume
        Run `/unleash` to continue from the current
        phase.
        ```

   g. Mark each successfully merged `[P]` task as `[x]`
      in `tasks.md`.

   **If Swarm worktrees are NOT available**: fall back to
   sequential execution for `[P]` tasks. Announce:
   > "Swarm worktrees not available -- executing parallel
   > tasks sequentially. Install Replicator for
   > parallel execution."

   Execute each `[P]` task sequentially via the
   `cobalt-crush-dev` agent, same as non-`[P]` tasks.

   **SwarmMail file reservations**: if `swarmmail_reserve`
   is available, reserve file paths for each parallel
   worker before spawning. If SwarmMail is NOT available,
   proceed without file locks -- worktree isolation
   provides sufficient protection for parallel workers.

4. **Phase checkpoint**: after all tasks in the phase
   are complete (both sequential and parallel), run the
   CI build and test commands derived from
   `.github/workflows/`.
   - If all pass: proceed to the next phase.
   - If any fail: **EXIT** with the failure details:

     ```
     ## /unleash paused at: implement (Phase N checkpoint)

     **Reason**: Build or test failure after Phase N

     **Failed command**: [the command that failed]
     **Output**: [error output]

     ### What to do next
     Fix the build/test failure, then re-run `/unleash`.

     ### Then resume
     Run `/unleash` to continue from the next phase.
     ```

### 8. Step 6 -- Code Review

Review the implementation using the review council in
code review mode. This includes the Phase 1a CI hard
gate, Phase 1b Gaze quality analysis (if available), and
Divisor agent reviews.

1. Read the full contents of
   `.opencode/commands/review-council.md`.
2. Delegate to the `cobalt-crush-dev` agent via the Task
   tool with the review council's instructions, adding
   the explicit mode override: "Run in **Code Review
   Mode** -- review the implementation code in
   `FEATURE_DIR`."
3. Collect the review results.

**If Gaze is not installed**: the review council will
skip Phase 1b with an informational note. This is
non-blocking -- code review proceeds without Gaze
quality data.

**Processing results**:

- If all reviewers APPROVE (and CI passes):
  Write the code review marker to
  `FEATURE_DIR/tasks.md`:
  ```
  <!-- code-review: passed -->
  ```
  Append this marker at the very end of
  `FEATURE_DIR/tasks.md` (after the spec-review marker
  if present). Proceed to step 7.

- If findings exist: attempt to fix them. For each
  iteration (up to 3 total):
  a. Address the findings by making code fixes.
  b. Re-run the review council in code review mode.
  c. If all APPROVE: proceed to step 7.

- If 3 iterations are exhausted with remaining findings:
  **EXIT** with the persistent issues:

  ```
  ## /unleash paused at: code review

  **Reason**: 3 review iterations exhausted

  ### Persistent findings

  [list each finding that persists across iterations,
   noting which were fixed vs. which remain]

  ### Circular dependencies (if any)

  [if fixing one reviewer's finding causes another
   reviewer's check to fail, describe the cycle]

  ### What to do next
  Fix the outstanding findings manually, then re-run
  `/unleash`.

  ### Then resume
  Run `/unleash` to continue from code review.
  ```

### 9. Step 7 -- Retrospective

Analyze the session and store learnings in semantic
memory.

1. Review the session: tasks completed, review findings
   encountered, fixes applied, patterns discovered.

2. Compose at least one learning as a natural language
   paragraph (Dewey's semantic search works best on
   narrative text, not bullet lists). Categorize each
   learning as one of:
   - **Patterns**: coding/design patterns that worked
     well
   - **Gotchas**: unexpected issues or edge cases
   - **Review Insights**: what the review council found
     and how it was fixed
   - **File-Specific**: learnings about specific files
     that future workers should know

3. **If Dewey is available** (`dewey_store_learning` tool
   exists): store each learning via `dewey_store_learning`
   with tags including:
   - The branch name (e.g., `018-unleash-command`)
   - The current date (e.g., `2026-03-29`)
   - The learning category (e.g., `pattern`, `gotcha`,
     `review-insight`, `file-specific`)

4. **If Dewey is NOT available**: skip the storage
   step with an informational note:
   > "Dewey not available -- retrospective learnings
   > not stored. Install Dewey for semantic memory."

   Display the learnings in the output so they are not
   lost.

### 10. Step 8 -- Demo

Present structured demo instructions to the developer.

1. **What Was Built**:
   - **If `WORKFLOW_TIER = speckit`**: read the spec's
     user story titles and descriptions from
     `FEATURE_DIR/spec.md`. Summarize what was
     implemented.
   - **If `WORKFLOW_TIER = openspec`**: read the change
     proposal from `FEATURE_DIR/proposal.md`. Summarize
     what was implemented.

2. **How to Verify**:
   - **If `WORKFLOW_TIER = speckit`**: read
     `FEATURE_DIR/quickstart.md` if it exists. If not,
     generate verification steps from the acceptance
     scenarios in `FEATURE_DIR/spec.md`.
   - **If `WORKFLOW_TIER = openspec`**: generate
     verification steps from the acceptance scenarios
     in `FEATURE_DIR/tasks.md`. OpenSpec changes do not
     have a `quickstart.md`.

3. **Key Files Changed**: run:
   ```bash
   git diff --name-only main...HEAD
   ```
   List the changed files grouped by directory.

4. **Test Results**: summarize the test output from the
   most recent build/test checkpoint.

5. **Next Steps**: always present these options:
   - `/finale` to commit, push, create PR, and return
     to main
   - `/speckit.clarify` to refine the spec and re-run
     `/unleash`

Format the output as:

```
## What Was Built

[summary from spec user stories]

## How to Verify

[verification commands from quickstart.md or acceptance
 scenarios]

## Key Files Changed

[grouped file list from git diff]

## Test Results

[pass/fail summary with counts]

## Next Steps

- Run `/finale` to create PR and watch CI
- Run `/speckit.clarify` to refine and iterate
```

## Guardrails

- **NEVER run on `main`** -- the command is for Speckit
  (`NNN-*`) and OpenSpec (`opsx/*`) feature branches
- **NEVER skip spec review exit on HIGH/CRITICAL** --
  these findings block implementation to prevent wasted
  effort on a flawed spec
- **NEVER merge worktrees with unresolved semantic
  conflicts** -- conflict markers in files after
  cherry-pick mean the merge failed and requires human
  resolution
- **ALWAYS present exit messages with actionable next
  steps** -- every exit point must tell the developer
  what happened, what to do, and how to resume
- **ALWAYS store at least one learning in retrospective**
  -- the retrospective is the swarm's memory; skipping
  it means the same mistakes will be repeated
- **ALWAYS clean up worktrees** -- stale worktrees waste
  disk space and can cause confusion on re-runs
- **NEVER create WorkflowInstance objects** -- `/unleash`
  operates at the Speckit pipeline level, not the hero
  lifecycle workflow level (Specs 008/012/016)
- **NEVER hardcode build/test commands** -- derive them
  from `.github/workflows/` (CI files are source of
  truth)
