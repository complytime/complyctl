---
description: "Check a hero constitution's alignment with the Unbound Force org constitution"
agent: constitution-check
---
<!-- scaffolded by uf vdev -->

# Command: /constitution-check

## Description

Compares a hero repository's constitution against the Unbound Force
org constitution and produces a structured alignment report. The
report shows whether each org principle is supported by the hero's
principles, identifies contradictions and gaps, and checks for a
parent constitution reference.

## Usage

```
/constitution-check [hero-constitution-path] [org-constitution-path]
```

### Arguments

- **hero-constitution-path** (optional): Path to the hero constitution
  file. Defaults to `.specify/memory/constitution.md` in the current
  repository.
- **org-constitution-path** (optional): Path to the org constitution
  file. Defaults to `.specify/memory/constitution.md` in the current
  repository if it is the unbound-force meta repo.

### Examples

```
# Check the current repo's constitution against the org constitution
/constitution-check

# Check a specific hero constitution
/constitution-check /path/to/hero/.specify/memory/constitution.md

# Check with explicit org constitution path
/constitution-check /path/to/hero/constitution.md /path/to/org/constitution.md
```

## Instructions

1. Parse `$ARGUMENTS` to extract optional file paths.

2. Locate the **org constitution**:
   - If a second argument is provided, use it as the org constitution
     path.
   - Otherwise, check if the current repository is the unbound-force
     meta repo (look for `unbound-force.md` at the repo root). If so,
     use `.specify/memory/constitution.md` as the org constitution.
   - If the current repo is NOT the meta repo, look for the org
     constitution at `../unbound-force/.specify/memory/constitution.md`
     (sibling directory). If not found, ask the user for the path.

3. Locate the **hero constitution**:
   - If a first argument is provided, use it as the hero constitution
     path.
   - Otherwise, use `.specify/memory/constitution.md` in the current
     repository.
   - If the current repo IS the meta repo and no hero path was
     specified, ask the user which hero to check.

4. Verify both files exist. If either is missing, report the error
   with the expected path and stop.

5. Read both constitution files.

6. Delegate to the `constitution-check` agent with both file contents
   as context. The agent will produce the structured alignment report.

7. Display the report to the user.

## Error Handling

- **File not found**: Report which file is missing and suggest the
  correct path. Do not attempt to create or modify any files.
- **Not a constitution**: If a file does not contain constitution
  markers (e.g., "## Core Principles", "## Governance"), report
  that the file does not appear to be a valid constitution.
- **Same file**: If both paths resolve to the same file, report
  that a hero constitution cannot be checked against itself.
