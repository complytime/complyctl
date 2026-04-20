# AI Tooling

This repository uses AI tools to help contributors code, review PRs, and manage feature specifications. [OpenCode](https://opencode.ai) is the standardized agent tool. [OpenSpec](https://opencode.ai) is the recommended spec-driven framework — both OpenSpec and SpecKit are agent-agnostic and work with any supported AI tool.

## Getting Started

### OpenCode (Recommended)

```bash
git clone <repository-url>
cd <repository>
opencode .
```

OpenCode auto-discovers project-specific commands from `.opencode/command/`. Install the OpenSpec or SpecKit plugin to get framework commands (like `/speckit.specify`).

### Other AI Tools

Any AI agent that supports command loading can use this repository's AI tooling:

1. Clone the repository
2. Install OpenSpec or SpecKit for your AI agent
3. Read `.specify/memory/constitution.md` for coding standards
4. Reference `.opencode/command/` for project-specific command definitions

## Commands

### `/review_pr <number>`

Reviews a pull request for alignment, security, and compliance. Designed to be token-efficient and CI-aware.

```
/review_pr 42
```

**How it works:**
1. **Checks CI status** — fetches check suite results and triages failures: distinguishes PR-caused failures from pre-existing issues on the base branch.
2. **Runs local tools** — detects and executes linters, test runners, and formatters available in the project (per `.specify/memory/constitution.md` Coding Standards). Skips checks CI already covered.
3. **AI reviews what tools can't** — focuses on intent alignment, security patterns, and architectural concerns.
4. **Offers fix-branch for pre-existing CI failures** — if a CI failure exists independently of the PR, offers to create a fix branch with a proposed resolution. You review and file the PR when ready.
5. **Offers in-line PR comments** — for HIGH+ findings, prepares in-line comments and shows them to you for confirmation before posting to the PR.

**What it checks:**
- CI: check suite pass/fail with causality triage (PR-caused vs. pre-existing)
- Local tools: lint, formatting, tests, coverage (deterministic, zero AI tokens)
- AI: alignment between PR intent/spec and code, security vulnerabilities, constitution compliance (judgment-based)

**Output:** CI status table + local tool results + structured AI findings with severity levels (CRITICAL / HIGH / MEDIUM / LOW), verdict, optional fix-branch for pre-existing failures, and optional in-line PR comments.

## Creating Commands

Commands are action-oriented prompts that the agent executes when invoked. They go in `.opencode/command/`. Framework commands (speckit.\*, opsx-\*) are managed by the plugin and should not be committed.

### File structure

1. Create `.opencode/command/your-command.md`
2. Add YAML frontmatter:
   ```yaml
   ---
   description: "Brief description of what the command does"
   ---
   ```
3. Write the command instructions in Markdown
4. Submit a PR

### Writing effective commands

- **Single purpose**: One command = one job. Combine concerns by chaining commands, not by overloading one.
- **Set the role first**: Open with a one-sentence persona and goal (e.g., "You are a token-efficient code reviewer."). This anchors the agent's behavior.
- **Define arguments**: List required and optional inputs with examples so the agent knows what to expect from the user.
- **Use numbered steps**: Break the workflow into sequential steps the agent follows. Each step should have a clear action and expected output.
- **Delegate to tools first**: If a check can be done deterministically (lint, test, format), run the tool and use its output — don't spend AI tokens re-analyzing what a tool already covers.
- **Specify the output format**: Define the exact structure of the response (headings, tables, severity levels). This keeps output consistent across runs and reviewers.
- **Reference, don't inline**: Point to `.specify/memory/constitution.md` for standards instead of copying rules into the command. This avoids drift and saves tokens.

See `.opencode/command/review_pr.md` as a reference implementation.

## Creating Skills

Skills provide domain knowledge the agent loads as context when activated. Unlike commands (which define *what to do*), skills define *how to think* about a specific domain. No skills are shipped initially — the structure is ready for contributors to add them.

### File structure

1. Create a directory: `.agents/skills/your-skill-name/`
2. Add a `SKILL.md` with YAML frontmatter:
   ```yaml
   ---
   name: your-skill-name
   description: "What this skill does"
   license: MIT
   compatibility: opencode
   metadata:
     audience: contributors
   ---
   ```
3. Write the skill instructions in Markdown below the frontmatter
4. Submit a PR

### Writing effective skills

- **Domain, not workflow**: A skill teaches the agent *about* a subject (e.g., "OSCAL compliance requirements," "Go error handling patterns"). For step-by-step workflows, create a command instead.
- **Keep it short**: Skills are loaded into context on activation, consuming tokens for the entire session. Target under 200 lines. If it's longer, split into focused sub-skills.
- **Be prescriptive**: State rules as "MUST / MUST NOT / SHOULD" directives the agent can follow mechanically. Avoid vague guidance like "consider security best practices."
- **Include examples**: Show 1-2 concrete before/after examples of correct vs. incorrect patterns. Examples anchor understanding better than abstract rules.
- **Reference external docs**: Link to detailed specifications or standards rather than inlining them. The agent can fetch them when needed instead of carrying them in context permanently.

## Key Files

| File | Purpose |
|------|---------|
| `.specify/memory/constitution.md` | Organizational governance and coding standards |
| `docs/AI_TOOLING.md` | This file — AI tooling documentation |
| `.agents/skills/` | Directory for AI skills — agent-agnostic, auto-discovered by OpenCode |
| `.opencode/command/review_pr.md` | PR review command |
| `specs/` | Feature specifications — SpecKit output |
| `openspec/` | Feature specifications — OpenSpec output |

## Specifications

Features are managed via spec-driven development. SpecKit and OpenSpec use separate output directories with coordinated sequential numbering:

```
specs/                        # SpecKit output
├── 001-first-feature/
│   └── spec.md
├── 002-second-feature/
│   └── spec.md
└── ...

openspec/                     # OpenSpec output
├── 005-next-feature/
│   └── spec.md
└── ...
```

Sequential numbers are coordinated across both directories. The next feature (in either directory) uses the next available number by scanning both. If `specs/004-*` is the highest, the next feature is `005-*` regardless of which framework creates it.

Review both directories for a complete chronological timeline of all features.

Use `/speckit.specify` (or the equivalent OpenSpec command) to create a new feature specification.
