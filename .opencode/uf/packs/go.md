---
pack_id: go
language: Go
version: 1.0.0
---
<!-- scaffolded by uf vdev -->

# Convention Pack: Go

## Coding Style

- **CS-001** [MUST] Format all Go source files with `gofmt`. No manual formatting overrides.
- **CS-002** [MUST] Organize imports with `goimports` in three groups separated by blank lines: standard library, third-party packages, internal packages.
- **CS-003** [MUST] Use PascalCase for exported identifiers and camelCase for unexported identifiers.
- **CS-004** [MUST] Add GoDoc-style comments on all exported functions, methods, and types. The comment MUST start with the identifier name.
- **CS-005** [MUST] Return `error` values from functions that can fail. Never use `panic` for expected error conditions.
- **CS-006** [MUST] Wrap errors with `fmt.Errorf("context: %w", err)` to preserve the error chain. The context MUST describe what operation failed.
- **CS-007** [MUST] Avoid mutable package-level variables. No global mutable state. Prefer functional style and dependency injection.
- **CS-008** [MUST] Use `github.com/charmbracelet/log` for all application logging. Do not use the standard library `log` package or bare `fmt.Println` for operational output.
- **CS-009** [MUST] Use `github.com/spf13/cobra` for CLI command routing and flag parsing.
- **CS-010** [SHOULD] Keep functions focused on a single responsibility. Extract helper functions when a function exceeds ~50 lines.
- **CS-011** [SHOULD] Prefer named return values only when they improve GoDoc clarity, not as a general practice.
- **CS-012** [SHOULD] Use constants or typed enums instead of raw string/int literals for domain values.
- **CS-013** [MUST] Follow standard Go naming idioms: receivers are short (1-2 letters), acronyms are all-caps (`ID`, `HTTP`, `URL`), interfaces ending in `-er` for single-method interfaces.

## Architectural Patterns

- **AP-001** [MUST] Use the `Options`/`Result` struct pattern: functions accept an `Options` struct for configuration and return a `Result` struct for output.
- **AP-002** [MUST] Implement core business logic as a `Run(opts Options) (*Result, error)` function. CLI commands delegate to `Run()` — no business logic in the command layer.
- **AP-003** [MUST] Use the testable CLI pattern: command runner functions accept a params struct that includes `io.Writer` fields for stdout and stderr, enabling unit testing without subprocess execution.
- **AP-004** [MUST] Use `embed.FS` for bundling static assets (templates, scaffolded files). Do not read assets from the runtime filesystem.
- **AP-005** [SHOULD] Implement the file ownership model: classify files as tool-owned (auto-updated on re-run) or user-owned (never overwritten without `--force`).
- **AP-006** [SHOULD] Insert version markers (`<!-- scaffolded by {tool} v{version} -->`) after YAML frontmatter in scaffolded Markdown files for provenance tracking.
- **AP-007** [MUST] Keep package boundaries clean. Business logic lives under `internal/`. CLI commands live under `cmd/`. The `internal/` packages MUST NOT import from `cmd/`.

## Security Checks

- **SC-001** [MUST] Never hardcode secrets, API keys, tokens, or credentials in source code or embedded assets.
- **SC-002** [MUST] Never commit `.env` files, credential JSON files, or private keys to the repository.
- **SC-003** [MUST] Use `filepath.Join` for all filesystem path construction. Never concatenate paths with string operations.
- **SC-004** [MUST] Validate target directories before writing files. Ensure the path is within the expected root and does not escape via `..` traversal.
- **SC-005** [MUST] Set safe file permissions when creating files: `0o644` for regular files, `0o755` for executable scripts and directories.
- **SC-006** [SHOULD] Audit embedded assets (`embed.FS`) for accidental inclusion of sensitive files. The embed directive pattern MUST be as narrow as possible.

## Testing Conventions

- **TC-001** [MUST] Use the standard library `testing` package only. Do not import testify, gomega, or any external assertion library.
- **TC-002** [MUST] Use `t.Errorf` or `t.Fatalf` for assertions directly. No third-party assertion helper functions.
- **TC-003** [MUST] Name tests following the `TestXxx_Description` pattern (e.g., `TestRun_CreatesFiles`, `TestIsToolOwned_ToolFiles`).
- **TC-004** [MUST] Use `t.TempDir()` for all tests that touch the filesystem. No shared mutable state between test cases.
- **TC-005** [MUST] Run tests with `-race -count=1`. All tests MUST pass under the race detector.
- **TC-006** [SHOULD] Use table-driven tests when exercising multiple input/output combinations for the same function.
- **TC-007** [SHOULD] Implement drift detection tests to ensure embedded assets (`go:embed`) match their canonical source files.
- **TC-008** [SHOULD] Name acceptance tests after spec success criteria (e.g., `TestSC001_ComprehensiveDetection`).
- **TC-009** [MUST] Verify specific expected values in assertions — not just `err == nil` or length checks. Assert return values, struct fields, and slice contents.
- **TC-010** [MUST] Ensure tests do not depend on execution order. Each test case MUST be independently runnable.
- **TC-011** [SHOULD] Guard slow tests (subprocess execution, full-module analysis) with `testing.Short()` checks.
- **TC-012** [SHOULD] Place test files alongside their source in the same package directory. Both internal (`_test.go` in same package) and external (`_test` package) test styles are acceptable.

## Documentation Requirements

- **DR-001** [MUST] Write GoDoc comments on every exported function, method, type, and package. Comments MUST be complete sentences starting with the identifier name.
- **DR-002** [MUST] Use RFC 2119 language (MUST, SHOULD, MAY, MUST NOT) for all requirement statements in specifications and governance documents.
- **DR-003** [SHOULD] Write acceptance criteria in Given/When/Then format with specific, verifiable outcomes.
- **DR-004** [SHOULD] Number functional requirements as FR-NNN and success criteria as SC-NNN in specification artifacts.
- **DR-005** [MUST] Use Conventional Commits format for all commit messages: `type: description` (e.g., `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`).

## Custom Rules

<!-- This section is intentionally empty in the canonical pack. Project-specific custom rules belong in go-custom.md -->
