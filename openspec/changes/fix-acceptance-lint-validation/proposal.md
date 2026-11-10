# Proposal

## Why

Local validation can report failures on an unchanged `main` checkout that
the corresponding CI checks neither run nor scope the same way. Contributors
cannot reliably distinguish baseline debt from regressions, and the four
acceptance-tagged security-lint findings lack an explicit disposition.

## What Changes

- Define supported local validation commands with documented tool versions,
  build tags, and file selection that match their CI counterparts.
- Add a CI-parity style and lint validation target that excludes vendored and
  generated style inputs without weakening security scanners.
- Add a separately named acceptance security-lint target that preserves gosec
  coverage and requires narrow fixes or documented dispositions for its
  baseline findings.
- Document the distinction between CI-parity validation, broader pre-commit
  checks, and acceptance-tagged security analysis.
- Add regression coverage for validation target scope and exclusions.

## Capabilities

### New Capabilities

- `ci-parity-local-validation`: Provides reproducible, documented local
  validation profiles that align tool versions, flags, and style-input scope
  with their intended CI checks while retaining separate security analysis.

### Modified Capabilities

- None.

## Impact

- Affected areas: `Makefile`, lint and pre-commit configuration, CI workflow
  integration, acceptance test lint handling, and contributor documentation.
- No public CLI API or runtime behavior changes.
- Existing security checks, including gosec and broader MegaLinter security
  scanners, remain in force; this change MUST NOT use blanket suppressions.
