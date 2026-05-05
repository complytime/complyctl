---
description: >
  Finalize a branch: commit, push, create PR, watch CI
  checks, and return to main. The PR stays open for
  review. One command to wrap up any feature or OpenSpec
  branch.
---
<!-- scaffolded by uf vdev -->

# Command: /finale

## User Input

```text
$ARGUMENTS
```

## Description

Automate the end-of-branch workflow. Stages all changes,
generates a conventional commit message, pushes, creates
a PR, watches CI checks, and returns to `main`. The PR
stays open for human review. Works with both Speckit
(`NNN-*`) and OpenSpec (`opsx/*`) branches.

## Usage

```
/finale                    # auto-detect everything
/finale fix the typo       # use as commit message hint
```

## Instructions

### 1. Branch Safety Gate

Get the current branch:

```bash
git rev-parse --abbrev-ref HEAD
```

- If on `main`: **STOP** with error:
  > "Cannot run /finale on main. Switch to a feature
  > branch (e.g., `opsx/*` or `NNN-*`) first."
- Otherwise: proceed. Note the branch name for the
  summary.

### 2. Check for Changes to Commit

Run `git status --short` to inspect the working tree.

**If no changes exist** (clean working tree):
- Check if there are unpushed commits:
  `git log origin/<branch>..HEAD --oneline 2>/dev/null`
- If unpushed commits exist: skip to step 4 (push).
- If no unpushed commits: check if a PR exists (step 5).
  If a PR exists, skip to step 6 (watch checks). If no
  PR and no changes, report "Nothing to finalize" and
  stop.

**If changes exist**:
- **Secrets check**: Scan unstaged/untracked files for
  names that likely contain secrets:
  - `.env`, `.env.*`
  - `credentials.json`, `secrets.json`, `*.key`, `*.pem`
  - Any file matching common secret patterns

  If potential secret files are found:
  > "Warning: the following files may contain secrets
  > and should not be committed:
  >
  > - .env.local
  > - credentials.json
  >
  > Proceed with staging all files? These files will be
  > included in the commit."

  Ask for confirmation. If the user declines, stop and
  let them handle it manually.

- **Stage all changes**: `git add .`

### 3. Generate and Confirm Commit Message

a. Analyze the staged changes:

```bash
git diff --cached --stat
git diff --cached
git log --oneline -5
```

b. Generate a conventional commit message:
- Determine the type: `feat:`, `fix:`, `docs:`,
  `chore:`, `refactor:`, `test:`
- Write a concise summary (1 line) focusing on the
  "why" not the "what"
- Add a body with bullet points if multiple logical
  changes are staged
- If `$ARGUMENTS` is not empty, use it as a hint or
  directly as the summary if it's already well-formed

c. Show the proposed message to the user:

> **Proposed commit message:**
>
> ```
> feat: add /finale slash command for branch finalization
>
> - Create finale.md command definition
> - Add scaffold asset and update file count test
> ```
>
> Approve, edit, or provide your own?

d. Commit with the approved message.

### 4. Push to Remote

```bash
# Check if upstream is set
git rev-parse --abbrev-ref @{upstream} 2>/dev/null
```

- If no upstream: `git push -u origin <branch>`
- If upstream exists: `git push`
- If push fails: report error and **STOP**.

### 5. Create or Find PR

Check if a PR already exists:

```bash
gh pr view --json number,url 2>/dev/null
```

- **If PR exists**: use its number and URL. Skip
  creation.
- **If no PR**: create one:

  a. Generate PR title from commit history:
  ```bash
  git log main..HEAD --oneline
  ```
  Use the most descriptive commit message as the title,
  or synthesize from multiple commits.

  b. Generate PR body: summarize all commits on the
  branch with a `## Summary` section and bullet points.

  c. Create:
  ```bash
  gh pr create --title "<title>" --body "<body>"
  ```

  d. Report the PR URL.

### 6. Watch CI Checks

```bash
gh pr checks <number> --watch
```

- **If checks pass**: proceed to step 7.
- **If checks fail**: report the failure details and
  **STOP**:

  > "CI checks failed on PR #<number>:
  >
  > - Build & Test: FAIL (45s)
  >   https://github.com/.../runs/...
  >
  > Options:
  > 1. Investigate the failure
  > 2. Re-run the checks
  > 3. Stop here and fix manually"

  Ask the user how to proceed.

### 7. Return to Main

Return to main so the developer can start other work:

```bash
git checkout main 2>/dev/null  # may already be on main
git pull
```

Verify:
```bash
git rev-parse --abbrev-ref HEAD
```

Should be `main`.

### 8. Summary

Display a completion report:

```
## Finale Complete

**Branch:** opsx/finale-command (pushed)
**Commit:** feat: add /finale slash command
**PR:** #65 — CI passed, ready for review
**Checks:** passed
**Status:** on main, up to date

Next: Request reviewers on the PR, then merge after
approval with: gh pr merge --rebase --delete-branch
```

## Guardrails

- **NEVER run on `main`** — the command is for feature
  branches only
- **NEVER merge the PR** — /finale creates PRs for
  review, not for immediate merge. Users merge manually
  after reviewer approval.
- **NEVER stage secret files without warning** — always
  prompt
- **NEVER commit without user approval** of the message
- **ALWAYS report the PR URL** so the user can review it
- **If any step fails**, stop immediately with context
  and options — do not attempt to continue or recover
  silently

## Branch Safety

This command inherits the branch safety guardrails from
the OpenSpec and Speckit workflows:

- Checks `git status` before any destructive operation
- All changes are committed before any branch switch
- The remote branch is NOT deleted — it stays open with
  the PR until a reviewer merges
