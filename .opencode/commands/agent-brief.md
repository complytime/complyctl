---
description: >
  Create, validate, and improve AGENTS.md -- the project briefing
  for AI coding agents. Auto-detects mode: creates from scratch
  when no AGENTS.md exists, audits and suggests improvements when
  one is present. Also ensures cross-tool bridge files (CLAUDE.md,
  .cursorrules) are properly configured.
---
<!-- scaffolded by uf vdev -->

# Command: /agent-brief

## Description

Manage the AGENTS.md lifecycle: create, audit, and improve the
project briefing that AI coding agents read at session start.

AGENTS.md is the API contract between humans and AI agents. It
tells agents how to build, test, and lint the project, what
conventions to follow, and what constraints to respect. Without
a good AGENTS.md, every agent session starts from cold context.

**Modes**:
- No AGENTS.md → **Create mode** (analyze project, generate file)
- AGENTS.md exists → **Audit mode** (score, report, suggest)
- `/agent-brief create` → Force create mode (overwrite)
- `/agent-brief audit` → Force audit mode (read-only)

## Instructions

### Step 1: Mode Detection

1. Check if AGENTS.md exists at the repository root.

2. Parse the user's argument (if any):
   - `create` → force create mode
   - `audit` → force audit mode
   - No argument → auto-detect based on file existence

3. Route to the appropriate mode:
   - **Create mode**: No AGENTS.md exists, or user passed `create`
   - **Audit mode**: AGENTS.md exists, or user passed `audit`

4. Announce the selected mode:
   - Create: `"No AGENTS.md found. Analyzing project to generate one..."`
   - Audit: `"Found AGENTS.md (N lines). Running quality audit..."`
   - Force create: `"Force-creating AGENTS.md. Existing file will be replaced after your review."`

### Step 2: Project Analysis

Analyze the project to understand its characteristics. Read the
following files (skip any that do not exist):

**Language & Dependencies**:
1. `go.mod` → Go project (extract module name, Go version, key deps)
2. `package.json` → Node/TypeScript project (extract name, scripts, key deps)
3. `Cargo.toml` → Rust project (extract name, edition, key deps)
4. `pyproject.toml` → Python project (extract name, version, key deps)
5. `tsconfig.json` → TypeScript confirmation

**Build System** (read only in Create mode or when refreshing
Build & Test section in Audit mode):
6. `Makefile` or `justfile` → extract build/test/lint targets
7. `.github/workflows/` → read CI workflow files to find exact
   build, test, vet, and lint commands (these are the source of
   truth for build commands)

**Linter Configuration**:
8. `.golangci.yml` → Go linter rules
9. `ruff.toml` or `pyproject.toml [tool.ruff]` → Python linter
10. `.eslintrc*` or `eslint.config.*` → JavaScript/TypeScript linter

**Project Context**:
11. `README.md` → project description (first paragraph or heading)
12. `LICENSE` → license type
13. `.git/config` → remote URL for project/org name
14. Top-level directory listing → project structure

**Governance & Context Detection**:
15. `.specify/memory/constitution.md` → constitution exists
    (triggers Behavioral Rules section)
16. `specs/` → check for `NNN-*/` subdirectories (Speckit)
17. `openspec/config.yaml` or `openspec/` → OpenSpec configured
    (triggers Specification Workflow section)
18. `.opencode/uf/packs/` → convention packs deployed
    (respect existing Convention Packs section from Go binary)
19. `opencode.json` → check for Dewey MCP server
    (triggers Knowledge Retrieval section)

Record what was detected. This informs both create and audit
modes.

### Step 3: Create Mode

If in create mode, generate AGENTS.md using the project analysis.

The target AGENTS.md has 10 sections. Sections 1-5 and 10 are
LLM-generated from project data. Sections 6-8 are verbatim
templates inserted conditionally. Section 9 is detected and
respected from the Go binary.

#### Section 1: Project Overview (LLM-generated)

Generate 2-5 lines from README first paragraph, plus a bullet
list of key attributes:

- What the project is (from README)
- **Type**: project type (CLI, library, web app, API, monorepo)
- Key domain context (heroes, tooling, etc.)
- **License**: license type
- **Mission**: one-line mission statement if available

#### Section 2: Build & Test Commands (LLM-generated)

- Extract exact commands from Makefile targets or CI workflows
- Include the flags that matter (e.g., `-race -count=1` for Go)
- Use fenced code blocks (```) for all commands
- Group as: Build, Test, Lint (and any other common targets)
- If CI workflows define additional structure (workflow names,
  purposes), include a CI Workflow Structure sub-table

#### Section 3: Project Structure (LLM-generated)

- Generate a directory tree showing major directories
- Focus on top-level and one level deep
- Annotate each directory with its purpose
- Use the `text` code fence format

#### Section 4: Coding Conventions (LLM-generated)

- Derive from linter config if present
- Include language-specific defaults:
  - Go: gofmt, goimports, error wrapping, import grouping
  - TypeScript: prettier, ESLint rules, naming conventions
  - Python: ruff/black, type hints, docstring style
  - Rust: clippy, formatting, error handling
- Include naming conventions, comment style, error handling
- Include spec writing conventions: RFC 2119 language
  (MUST/SHOULD/MAY), Given/When/Then scenarios, FR-NNN
  numbering, line length < 72

#### Section 5: Testing Conventions (LLM-generated)

- Framework and version
- Test naming pattern (e.g., `TestXxx_Description`)
- Assertion style (stdlib vs. assertion library)
- Isolation strategy (e.g., `t.TempDir()` for filesystem)
- Special requirements (e.g., drift detection for embedded
  assets)

#### Section 6: Behavioral Rules (verbatim template, conditional)

**Condition**: Insert ONLY when `.specify/memory/constitution.md`
exists. If no constitution is detected, omit this section entirely.

Insert this verbatim:

```markdown
## Behavioral Rules

These rules are non-negotiable. Violations are CRITICAL severity.

- **Gatekeeping**: MUST NOT modify quality/governance gates
  (coverage thresholds, CRAP scores, severity definitions,
  CI flags, agent settings, constitution MUST rules, review
  limits, workflow markers). Stop and report instead.
- **Phase boundaries**: MUST NOT cross workflow phase boundaries.
  Spec phases: spec artifacts only. Implement: source code.
  Review: fixes only. Violation = process error, stop immediately.
- **CI parity**: MUST replicate CI checks locally before marking
  tasks complete. Derive commands from `.github/workflows/`.
- **Review council**: MUST run `/review-council` before PR
  submission. Resolve all REQUEST CHANGES. No code changes
  between APPROVE and PR. Exempt: constitution amendments,
  docs-only, emergency hotfixes.
- **Branch protection**: MUST NOT commit directly to `main`.
  All changes via feature branches and PRs.
- **Documentation gate**: Before marking a task complete,
  assess documentation impact: `CHANGELOG.md` for change
  entries, `AGENTS.md` for structural updates (project
  structure, conventions, build commands), `README.md` for
  description changes.
- **Website gate**: MUST file `unbound-force/website` issue
  for user-facing changes before PR merge. Exempt: internal
  refactoring, test-only, CI-only, spec artifacts.
- **Zero-waste**: No orphaned specs, unused standards, or
  aspirational documents that do not map to actionable work.

### PR Review Commands

| Command | When | Scope |
|---------|------|-------|
| `/review-council` | Pre-PR (local) | 5+ Divisor agents |
| `/review-pr [N]` | Post-PR (GitHub) | Single agent, CI analysis |
```

#### Section 7: Specification Workflow (verbatim template, conditional)

**Condition**: Insert ONLY when `specs/` directory has numbered
subdirectories (`NNN-*/`) OR `openspec/` directory exists. If
neither is detected, omit this section entirely.

Insert this verbatim:

```markdown
## Specification Workflow

All non-trivial changes MUST be preceded by a spec workflow.

| Tier | Tool | When | Artifacts |
|------|------|------|-----------|
| Strategic | Speckit | >= 3 stories, cross-repo | `specs/NNN-*/` |
| Tactical | OpenSpec | < 3 stories, single-repo | `openspec/changes/*/` |

Pipeline: `constitution → specify → clarify → plan → tasks →
analyze → checklist → implement`

**Ordering**: Constitution before specs. Spec before plan. Plan
before tasks. Tasks before implementation. Spec artifacts MUST
be committed/pushed before implementation begins.

**Branches**: Speckit: `NNN-<name>`. OpenSpec: `opsx/<name>`.

**Task bookkeeping**: Mark checkboxes `[x]` immediately on
completion. `[P]` marks parallel-eligible tasks.

**When in doubt**: Start with OpenSpec. Escalate to Speckit if
scope grows beyond 3 stories or crosses repo boundaries.

**What requires a spec**: New features, refactoring that changes
signatures, test additions across multiple functions, agent
changes, CI changes, data model changes.

**Exempt**: Constitution amendments, typo fixes, emergency
hotfixes (retroactively documented).
```

#### Section 8: Knowledge Retrieval (verbatim template, conditional)

**Condition**: Insert ONLY when `opencode.json` contains a Dewey
MCP server configuration. If Dewey is not configured, omit this
section entirely.

Insert this verbatim:

```markdown
## Knowledge Retrieval

Prefer Dewey MCP tools over grep/glob/read for cross-repo
context and architectural patterns.

| Intent | Tool |
|--------|------|
| Conceptual | `dewey_semantic_search` |
| Keyword | `dewey_search` |
| Navigation | `dewey_traverse`, `dewey_get_page` |
| Discovery | `dewey_find_connections`, `dewey_similar` |

**Fallback**: Use Read/Grep/Glob when Dewey is unavailable,
for exact string matching, known file paths, or non-Markdown
content (Go source, JSON, YAML).
```

#### Section 9: Convention Packs (detected, not generated)

**Condition**: If `.opencode/uf/packs/` directory exists and
contains `.md` files, check if AGENTS.md already has a
`## Convention Packs` section (written by the Go binary's
`ensureAGENTSmdPackSection`).

- If the section already exists → keep it as-is, do not
  regenerate.
- If the directory exists but no section → generate a
  section listing the pack files found, using the same format
  as the Go binary:

```markdown
## Convention Packs

This repository uses convention packs scaffolded by
unbound-force. Agents MUST read the applicable pack(s)
before writing or reviewing code.

- `.opencode/uf/packs/<file1>.md`
- `.opencode/uf/packs/<file2>.md`
...
```

- If no `.opencode/uf/packs/` directory exists → omit section.

#### Section 10: Architecture (LLM-generated)

- Describe the dominant design patterns in the project
- Key architectural patterns (e.g., Options/Result structs,
  embed.FS scaffold, Cobra CLI delegation)
- Keep concise: 5-10 lines max

#### 3a: CHANGELOG.md Handling

If `CHANGELOG.md` does not exist at the repository root,
create it with just a heading:

```markdown
# Changelog
```

Do NOT add entries -- just the heading. Entries are added
by the Scribe and `update-agent-context.sh`.

#### 3b: Present and Write

1. Show the complete generated AGENTS.md to the user.
2. Ask: "Does this look good? I can write it now, or you can
   suggest changes first."
3. On confirmation, write the file to `AGENTS.md` at repo root.
4. If CHANGELOG.md was created, mention it in the summary.
5. Proceed to Step 5 (Bridge Files).

### Step 4: Audit Mode

If in audit mode, read the existing AGENTS.md and evaluate it
against the flat section taxonomy.

#### 4a: Section Detection

Scan the file for section headers matching these patterns. A
section is "found" if any of its patterns match a `##` header
line (case-insensitive):

| # | Section | Detection Patterns | Conditional |
|---|---------|--------------------|-|
| 1 | Project Overview | `overview`, `about` | No |
| 2 | Build & Test Commands | `build`, `development` | No |
| 3 | Project Structure | `structure`, `layout`, `directory` | No |
| 4 | Coding Conventions | `convention`, `coding standard`, `style guide`, `coding convention` | No |
| 5 | Testing Conventions | `test` | No |
| 6 | Behavioral Rules | `behavioral`, `rule`, `constraint` | Constitution |
| 7 | Specification Workflow | `specification`, `spec framework`, `speckit`, `openspec`, `spec workflow` | specs/openspec |
| 8 | Knowledge Retrieval | `knowledge`, `retrieval`, `dewey` | Dewey MCP |
| 9 | Convention Packs | `convention pack` | Packs dir |
| 10 | Architecture | `architect`, `pattern`, `design` | No |

Record which sections are found and which are missing.
For conditional sections, only flag as missing if their
trigger condition is met (e.g., don't flag missing
Behavioral Rules if no constitution exists).

#### 4b: Selective Refresh

Re-derive these sections from the current filesystem and
compare against what AGENTS.md contains:

**Build & Test Commands**: Read Makefile and CI workflow files.
Compare extracted commands/targets against what the Build
section lists. Flag specific deltas:
- Missing Makefile targets not in AGENTS.md
- CI workflow names/files that changed
- Command flags that differ

**Project Structure**: List the actual top-level directories.
If the section contains a directory tree (lines with `├`, `└`,
`│`, or indented paths ending with `/`), verify each listed
directory exists. Flag:
- Directories in AGENTS.md that no longer exist
- New directories not listed in AGENTS.md

#### 4c: Quality Metrics

1. **Line count**: Count total lines. Flag if >300.
2. **Build code blocks**: Check if the Build section contains
   at least one fenced code block (triple backtick). Flag if
   the section exists but has no code blocks.
3. **Constitution reference** (only when
   `.specify/memory/constitution.md` exists): Check if AGENTS.md
   has a Behavioral Rules section. Flag if absent.
4. **Spec framework reference** (only when `specs/` has numbered
   subdirs or `openspec/` exists): Check if AGENTS.md describes
   the spec framework. Flag if absent.
5. **Branch protection**: Check if AGENTS.md contains explicit
   instructions prohibiting direct commits to `main`. Look for
   co-occurrence of "main" with "MUST NOT"/"never"/"prohibited"
   in Behavioral Rules or similar section. Flag if absent.
6. **Governance rule completeness** (only when Behavioral Rules
   section exists): Verify all 8 rules are present: Gatekeeping,
   Phase boundaries, CI parity, Review council, Branch protection,
   Documentation gate, Website gate, Zero-waste. Flag any missing.

#### 4d: Scoring

Calculate the overall effectiveness label. Count the 5 core
sections (1-5) as essential. Count conditional sections (6-8)
only when their triggers are detected. Section 9 (Convention
Packs) and 10 (Architecture) are recommended.

| Label | Criteria |
|-------|----------|
| Excellent | All essential + all triggered conditional + all recommended |
| Strong | All essential + all triggered conditional |
| Adequate | 4-5/5 essential |
| Weak | 2-3/5 essential |
| Missing | 0-1/5 essential |

#### 4e: Generate Report

Produce a structured report:

```
## /agent-brief: Audit Report

### Section Coverage

| # | Section | Status | Notes |
|---|---------|:------:|-------|
| 1 | Project Overview | ✅ | [notes] |
| 2 | Build & Test | ✅ | [notes] |
| 3 | Project Structure | ✅/⚠ | [stale dirs if any] |
| 4 | Coding Conventions | ✅ | [notes] |
| 5 | Testing Conventions | ✅ | [notes] |
| 6 | Behavioral Rules | ✅/⊘ | [conditional] |
| 7 | Specification Workflow | ✅/⊘ | [conditional] |
| 8 | Knowledge Retrieval | ✅/⊘ | [conditional] |
| 9 | Convention Packs | ✅/⊘ | [detected] |
| 10 | Architecture | ✅ | [notes] |

### Selective Refresh

[List specific deltas found in Build & Test and Project
Structure sections, or "No staleness detected."]

### Quality Metrics

| Metric | Value | Status |
|--------|-------|--------|
| Total lines | N | ✅/⚠ |
| Essential sections | N/5 | ✅/❌ |
| Conditional sections | N/N | ✅/⚠ |
| Build code blocks | N | ✅/⚠ |
| Governance rules | N/8 | ✅/⚠ |
| Bridge: CLAUDE.md | present/missing | ✅/⚠ |
| Bridge: .cursorrules | present/missing | ✅/⚠ |

### Improvement Suggestions

[numbered list of specific, actionable suggestions with
generated content for missing sections]

### Overall Score: [Label] ([N/N] essential, [N/N] conditional, [N/N] recommended)
```

#### 4f: Offer Improvements

If improvements were suggested:
1. Ask: "Would you like me to apply these improvements?"
2. On confirmation, apply the changes:
   - Insert missing sections at appropriate locations
   - Update stale directory references
   - Do NOT modify existing project-specific sections
3. After applying, re-run the audit to show updated score.

### Step 5: Bridge File Verification

After creating or improving AGENTS.md, verify cross-tool bridge
files exist. Bridge file creation is owned by `uf init`
(`ensureCLAUDEmd()` and `ensureCursorrules()`). This command
only checks their status and suggests running `uf init` if
they are missing or misconfigured.

**CLAUDE.md**:
1. Check if CLAUDE.md exists at repo root.
2. If it exists, check if it contains `@AGENTS.md`.
3. If missing or lacking the import:
   - Report: `"⚠ CLAUDE.md: missing or does not import AGENTS.md"`
   - Suggest: `"Run: uf init to create bridge files"`
4. If already configured:
   - Report: `"⊘ CLAUDE.md: already imports AGENTS.md"`

**.cursorrules**:
1. Check if .cursorrules exists at repo root.
2. If it exists, check if it references AGENTS.md.
3. If missing or lacking the reference:
   - Report: `"⚠ .cursorrules: missing or does not reference AGENTS.md"`
   - Suggest: `"Run: uf init to create bridge files"`
4. If already configured:
   - Report: `"⊘ .cursorrules: already references AGENTS.md"`

**Note**: `uf init` is the canonical owner of bridge file
creation. It generates CLAUDE.md with `@AGENTS.md` plus
convention pack `@` imports, and .cursorrules with AGENTS.md
reading instructions. Do NOT create bridge files with a
different marker -- defer to `uf init`.

### Step 6: Summary Report

Display a final summary:

**Create mode**:
```
## /agent-brief: Complete

### Created
  ✅ AGENTS.md: generated (N lines)
  ✅ CHANGELOG.md: created (if newly created)
  [bridge file statuses]

### Next Steps
  Review the Architecture section and add project-specific
  patterns. Then run `uf init` to deploy convention packs
  and agents.
```

**Audit mode**:
```
## /agent-brief: Audit Complete

### Score: [Label]
  [section coverage summary]
  [selective refresh results]
  [quality metrics summary]
  [improvements applied or suggested]
```

## Guardrails

- **NEVER modify files outside AGENTS.md, CLAUDE.md,
  .cursorrules, and CHANGELOG.md** -- this command manages
  agent context files only.
- **NEVER modify CHANGELOG.md content beyond initial
  creation** -- only create the file with a `# Changelog`
  heading if it does not exist. Do not add, edit, or remove
  entries.
- **NEVER implement code, modify source files, update tests,
  or change configuration** -- this command produces
  documentation artifacts.
- **ALWAYS present generated content to the user before
  writing** -- never auto-write without confirmation.
- **ALWAYS respect existing project-specific sections** -- when
  improving, insert missing sections but do not rewrite or
  remove sections the user has customized.
- **NEVER remove content** -- only add missing sections or
  update stale references. If a section should be condensed,
  suggest it but do not apply without confirmation.
- **Use actual project data** -- in create mode, fill sections
  from real files (README, Makefile, go.mod, CI config). Do
  not use placeholder text or generic examples.
- **Respect Convention Packs ownership** -- the Go binary's
  `ensureAGENTSmdPackSection` owns this section. Detect and
  preserve it; do not regenerate if it already exists.
