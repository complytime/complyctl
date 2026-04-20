---
description: "Review a pull request for alignment, security, and constitution compliance"
---

# Review Pull Request

You are a token-efficient code reviewer. The user will provide a PR number. Delegate deterministic checks to local tools and CI results first, then apply AI judgment only where tools cannot reach: intent alignment, security patterns, and architectural concerns.

## Arguments

- **PR number** (required): The pull request number to review (e.g., `42`)

## Execution Steps

### 1. Fetch PR Metadata (Minimal)

Retrieve PR metadata first — avoid loading the full diff until needed:

```bash
gh pr view <PR_NUMBER> --json title,body,files,additions,deletions,baseRefName,headRefName,labels,milestone,commits
```

Record the PR title, description, branch name, base branch, and changed file list. **Do NOT fetch the full diff yet** — later steps determine which files need AI analysis.

### 2. Fetch CI Check Results

Retrieve the CI/CD check suite status for the PR:

```bash
gh pr checks <PR_NUMBER> --json name,state,description,link
```

Categorize each check as:
- **PASS**: Check succeeded
- **FAIL**: Check failed
- **PENDING**: Check still running
- **SKIPPED**: Check was skipped

If checks are still PENDING, inform the user and ask whether to wait or proceed with the available results.

**If all checks pass**: Record this and move to Step 3. No CI triage needed.

**If any checks fail**: Proceed to Step 2a for causality determination.

#### 2a. CI Failure Causality Determination

For each failing check, determine whether the failure is caused by the PR's changes or is a pre-existing issue on the base branch.

**Method**: Check if the same test/check also fails on the base branch:

```bash
# Get the base branch name (from Step 1 metadata, e.g., "main")
BASE_BRANCH="<baseRefName from Step 1>"

# Check the latest CI status on the base branch
gh api repos/{owner}/{repo}/commits/${BASE_BRANCH}/check-runs --jq '.check_runs[] | select(.name == "<FAILING_CHECK_NAME>") | {name, conclusion}'
```

**Classification**:

| Base branch status | PR check status | Classification |
|--------------------|-----------------|----------------|
| Pass | Fail | **PR-caused** — the PR introduced the failure |
| Fail | Fail | **Pre-existing** — failure exists independently of the PR |
| No data | Fail | **Unknown** — treat as PR-caused (conservative) |

Record the classification for each failing check. This feeds into Step 5 (AI review) and Step 8 (fix-branch).

### 3. Run Local Deterministic Tools (Pre-flight)

Run the project's own tools as a rapid pre-flight check. These are authoritative per the constitution's Coding Standards section. **If CI already ran and passed the same checks, skip re-running them locally** to save time.

**Detection**: Check which tools are available by looking for their configuration files:

```bash
test -f Makefile && echo "MAKEFILE=yes"
test -f .golangci.yml && echo "GO_LINT=yes"
test -f ruff.toml -o -f pyproject.toml && echo "PYTHON_LINT=yes"
test -f .yamllint.yml && echo "YAML_LINT=yes"
test -f .pre-commit-config.yaml && echo "PRECOMMIT=yes"
```

**Execution** (run only what is detected, skip if CI already covers it):

| Tool detected | Command to run | What it checks |
|---------------|----------------|----------------|
| Makefile | `make lint` (or `make check`) | Project-defined lint/format/vet |
| `.golangci.yml` | `golangci-lint run ./...` | Go lint rules |
| `ruff.toml` / `pyproject.toml` | `ruff check .` | Python lint rules |
| `.yamllint.yml` | `yamllint .` | YAML lint rules |
| `.pre-commit-config.yaml` | `pre-commit run --all-files` | Pre-commit hooks |
| `go.mod` | `go test ./...` | Go tests |
| `pyproject.toml` / `setup.py` | `pytest` or `python -m pytest` | Python tests |

**Record results**: Capture tool exit codes and output. If tools pass, skip those categories in the AI review entirely. If tools fail, include the failure output as context.

**If no tools are detected**: Note this and proceed to AI-based review for all categories.

### 4. Fetch Diff (Scoped)

Now fetch the diff, being token-conscious:

```bash
gh pr diff <PR_NUMBER>
```

**Large diff handling**:
- If the diff exceeds 500 lines, process file-by-file instead of loading the entire diff
- Skip binary files, lock files (`package-lock.json`, `go.sum`, `bun.lock`), and auto-generated files
- For very large PRs (2000+ lines changed), warn the user and ask whether to review all files or focus on specific ones

### 5. Locate Associated Specification

Search both the `specs/` and `openspec/` directories for a specification that matches this PR:

- Check if the PR branch name matches a spec directory in either location:
  - `specs/<branch-name>/spec.md` (SpecKit output)
  - `openspec/<branch-name>/spec.md` (OpenSpec output)
- Check if the PR description references a spec
- If a spec is found in either directory, read only the **Functional Requirements** and **User Stories** sections (not the entire spec) to minimize token usage
- If no spec is found in either directory, note this and use the PR title and description as the intent source

### 6. AI Review (Judgment-Based Only)

Focus AI analysis exclusively on what deterministic tools and CI cannot check. Skip any category where local tools or CI already passed.

#### 6a. Alignment Check

Compare the PR intent (title + description + linked spec) against the actual code changes:

- **Scope alignment**: Do the changed files match what the spec/description says should change? Flag files modified outside the stated scope.
- **Requirement coverage**: For each requirement in the spec (if found), verify the code changes address it. Flag uncovered requirements.
- **Completeness**: Are there partial implementations that could leave the system in an inconsistent state?
- **Drift detection**: Does the code do anything NOT described in the intent/spec? Flag undocumented behavioral changes.

#### 6b. Security Review

Examine the diff for security vulnerabilities that linters cannot catch:

- **Input sanitization**: Are external inputs (user input, API parameters, file paths, environment variables, command arguments) validated before use in:
  - SQL queries (injection risk)
  - Shell commands (command injection)
  - File paths (path traversal)
  - HTML/template output (XSS)
  - YAML/JSON parsing (deserialization attacks)
- **Unexpected workflows**: Can the code be executed in an unintended order or context?
  - Missing authentication/authorization checks
  - Race conditions or TOCTOU vulnerabilities
  - State machine violations (skipping steps)
  - Error handling that exposes sensitive information
- **Privilege escalation**: Does the code grant permissions or elevate privileges without proper validation?
- **Secrets and credentials**: Are there hardcoded secrets, tokens, or API keys? Are secrets logged or exposed in error messages?
- **Dependency risks**: Are new dependencies well-maintained and from trusted sources?

#### 6c. Constitution Compliance (AI-only items)

Read `.specify/memory/constitution.md`. **Only check items that local tools and CI did NOT already verify.** Typical AI-only checks:

- **Architectural principles**: Does the code follow Single Responsibility and Isolation (Principle II)?
- **Incremental improvement**: Does the PR stay focused on a single concern (Principle III)?
- **Readability**: Is the code self-documenting? Do comments explain the "why" not the "what" (Principle IV)?
- **No reinvention**: Does the code avoid custom implementations when established libraries exist (Principle V)?
- **Composability**: Are components modular and their outputs consumable by other tools (Principle VI)?
- **Testing coverage**: Are tests present for the changes? Do they include positive and negative cases?

**Skip if already covered by local tools or CI**: naming conventions, line length, lint issues, formatting, file headers.

#### 6d. CI Failure Analysis

For each CI failure classified in Step 2a, provide analysis:

**PR-caused failures**: Include as HIGH or CRITICAL findings:
- Which check failed and what the error output says
- Which PR change likely caused the failure (map failing test to changed file/function)
- Suggested fix or direction

**Pre-existing failures**: Report separately with clear labeling:
- Confirm the failure also exists on the base branch
- Brief root cause analysis if determinable from the error output
- Note that this will be addressed in Step 8 (fix-branch offer)

### 7. Output Format

Present findings in this structured format:

```markdown
## PR Review: #<NUMBER> — <TITLE>

### CI Status
| Check | Status | Classification |
|-------|--------|----------------|
| <name> | PASS/FAIL | PR-caused / Pre-existing / N/A |

### Local Tool Results
<Table showing which tools ran, pass/fail status, and summary of failures if any>

### Summary
<1-2 sentence overview of what the PR does and overall assessment>

### Alignment
- <Finding with severity>

### Security
- <Finding with severity>

### Constitution Compliance
- <Finding with severity>

### CI Failures (PR-caused)
- <Finding with severity — only if PR-caused failures exist>

### CI Failures (Pre-existing)
- <Description — only if pre-existing failures exist>
- Note: These failures exist independently of this PR. See fix-branch offer below.

### Verdict
**<APPROVE / REQUEST CHANGES / COMMENT>**

<Brief justification. Pre-existing CI failures do NOT block the PR verdict.>
```

**Severity levels**:
- **CRITICAL**: Must be fixed before merge (security vulnerabilities, data loss risks)
- **HIGH**: Should be fixed before merge (spec violations, missing tests for critical paths, PR-caused CI failures)
- **MEDIUM**: Recommended to fix (code quality, minor compliance issues)
- **LOW**: Optional improvements (style, naming suggestions)

If no issues are found in a category, state "No issues found."

### 8. Offer Fix-Branch for Pre-existing CI Failures

If Step 2a identified any **pre-existing** CI failures, offer to create a fix branch:

```
I identified <N> pre-existing CI failure(s) that are NOT caused by this PR:
- <check name>: <brief description of failure>

These failures also occur on the base branch (<BASE_BRANCH>).

Would you like me to create a fix branch with a proposed resolution?
I will create the branch and commit locally — you can review the changes and file a PR when ready.
```

**If the user agrees**:

1. **Create a fix branch** from the base branch:
   ```bash
   git checkout <BASE_BRANCH>
   git checkout -b fix/ci-<descriptive-name>
   ```
   Branch naming: `fix/ci-<failing-check-or-test-name>` (e.g., `fix/ci-yamllint-line-length`, `fix/ci-test-auth-timeout`)

2. **Analyze and propose the fix**: Use the CI failure output and the failing file(s) to determine the minimal change needed. Keep the scope as small as possible — fix only what is failing.

3. **Commit with Conventional Commits format**:
   ```bash
   git add <changed-files>
   git commit -m "fix: resolve <failing-check> CI failure

   <Brief description of what was wrong and how the fix addresses it.>

   This failure was pre-existing on <BASE_BRANCH> and unrelated to PR #<PR_NUMBER>."
   ```

4. **Report to the user**:
   ```
   Fix branch created: fix/ci-<name>

   Changes:
   - <file>: <what changed>

   The branch is local. To review and push:
     git checkout fix/ci-<name>
     git log -1
     git push -u origin fix/ci-<name>
   ```

5. **Switch back** to the PR branch:
   ```bash
   git checkout <PR_BRANCH>
   ```

**Guardrails**:
- The fix MUST be scoped to the specific failing check — no unrelated changes
- The agent MUST NOT push to the remote or file a PR automatically
- If the fix is non-trivial (requires understanding business logic or architectural decisions), inform the user instead of attempting a fix:
  ```
  The CI failure in <check> appears to require a non-trivial fix involving <description>.
  I recommend investigating this separately rather than proposing an automated fix.
  ```

### 9. Offer In-line PR Comments

After presenting the summary, if there are findings with severity HIGH or above, offer to post them as in-line comments on the PR:

```
I found <N> findings (X CRITICAL, Y HIGH). Would you like me to post in-line comments on the PR so the author can see them in context?

I will prepare the comments and show them to you for approval before posting anything.
```

**If the user agrees**:

1. **Prepare comments**: For each finding that maps to a specific file and line range in the diff, prepare an in-line comment with:
   - The finding description
   - The severity level
   - A concrete suggestion for fixing the issue (if applicable)

2. **Show all comments for human review**: Present each prepared comment in this format:
   ```
   File: <path>
   Line: <line_number>
   Body: <comment text>
   ```

3. **Wait for explicit confirmation**: Ask "Post these comments? (yes/no/edit)"
   - **yes**: Post all comments using the `gh` CLI:
     ```bash
     gh pr review <PR_NUMBER> --comment --body "<summary comment>"
     ```
     For in-line comments, use the GitHub API:
     ```bash
     gh api repos/{owner}/{repo}/pulls/<PR_NUMBER>/reviews \
       --method POST \
       -f body="AI-assisted review" \
       -f event="COMMENT" \
       -f comments[]="<JSON array of inline comments>"
     ```
   - **no**: Skip posting, the summary is sufficient
   - **edit**: Let the user modify comments before posting, then re-confirm

4. **CRITICAL RULE**: NEVER post comments without explicit human confirmation. Always show the exact content that will be posted and wait for approval.
