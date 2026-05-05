---
pack_id: default
language: Any
version: 1.0.0
---
<!-- scaffolded by uf vdev -->

# Convention Pack: Default (Language-Agnostic)

This is the language-agnostic default convention pack for
The Divisor PR reviewer framework. It contains universal
software engineering rules that apply regardless of
programming language, framework, or runtime. It serves as
the fallback pack when no language-specific pack (e.g.,
`go.md`, `typescript.md`) is available.

Language-specific packs SHOULD extend these rules with
targeted checks. When a language-specific pack is active,
personas load both packs -- the language pack takes
precedence on overlapping concerns.

---

## Coding Style

- **CS-001** [MUST] Code MUST be formatted consistently
  using the project's declared standard formatter. If no
  formatter is declared, files MUST use consistent
  indentation (all spaces or all tabs, not mixed) within
  each file.

- **CS-002** [MUST] Code MUST NOT contain dead code --
  unreachable branches, commented-out blocks, or unused
  imports, variables, functions, or types. Dead code
  obscures intent and inflates maintenance cost.

- **CS-003** [MUST] Identifiers (variables, functions,
  types, constants) MUST use meaningful, descriptive
  names that convey purpose. Single-letter names are
  acceptable only for conventional loop indices (`i`,
  `j`, `k`) and very short lambdas.

- **CS-004** [MUST] Code MUST follow the DRY principle.
  Identical or nearly identical logic appearing in more
  than two locations MUST be extracted into a shared
  function, method, or module.

- **CS-005** [SHOULD] Import and dependency declarations
  SHOULD be organized into logical groups (e.g., standard
  library, third-party, internal) separated by blank
  lines, following the project's established ordering
  convention.

- **CS-006** [MUST] Error handling MUST be consistent
  across the codebase. Errors MUST NOT be silently
  swallowed. Every error MUST be either handled (logged,
  returned, recovered) or explicitly documented as
  intentionally ignored with a justifying comment.

- **CS-007** [SHOULD] Magic numbers and hardcoded string
  literals SHOULD be extracted into named constants or
  configuration values. Exceptions: `0`, `1`, `-1`,
  empty string, and boolean literals used in obvious
  contexts.

- **CS-008** [SHOULD] Functions and methods SHOULD be
  small and focused -- each doing one thing well.
  Functions exceeding ~50 lines of logic (excluding
  comments and blank lines) SHOULD be evaluated for
  decomposition.

---

## Architectural Patterns

- **AP-001** [MUST] Each module, class, or package MUST
  have a single, well-defined responsibility (Single
  Responsibility Principle). A unit of code that handles
  both business logic and I/O coordination is a
  violation.

- **AP-002** [SHOULD] Dependencies SHOULD be injected
  rather than hard-instantiated. Functions and
  constructors SHOULD accept interfaces or abstractions
  rather than concrete implementations, enabling testing
  and substitution.

- **AP-003** [MUST] Separation of concerns MUST be
  maintained across architectural layers. Presentation
  logic MUST NOT contain business rules. Data access
  logic MUST NOT contain rendering or CLI output.

- **AP-004** [SHOULD] Interfaces SHOULD be narrow and
  client-specific rather than broad and general-purpose
  (Interface Segregation Principle). Consumers SHOULD
  NOT be forced to depend on methods they do not use.

- **AP-005** [MUST] Circular dependencies between
  packages, modules, or layers MUST NOT exist. If
  module A imports module B, module B MUST NOT import
  module A (directly or transitively).

---

## Security Checks

- **SC-001** [MUST] Code MUST NOT contain hardcoded
  secrets, credentials, API keys, tokens, passwords, or
  private keys. Secrets MUST be loaded from environment
  variables, secret managers, or encrypted configuration
  files.

- **SC-002** [MUST] All external input (user input, API
  payloads, file contents, environment variables used as
  data) MUST be validated and sanitized before use.
  Validation MUST reject unexpected types, lengths, and
  formats.

- **SC-003** [MUST] File system operations MUST validate
  paths to prevent directory traversal attacks. Paths
  constructed from external input MUST be canonicalized
  and constrained to expected directories.

- **SC-004** [MUST] Database queries constructed with
  external input MUST use parameterized queries or
  prepared statements. String concatenation or
  interpolation for query construction MUST NOT be used.

- **SC-005** [SHOULD] Dependencies SHOULD be reviewed for
  known vulnerabilities before adoption and periodically
  thereafter. Projects SHOULD use automated dependency
  scanning (e.g., Dependabot, Snyk, `govulncheck`,
  `npm audit`) and address critical/high findings
  before merge.

---

## Testing Conventions

- **TC-001** [MUST] New functionality MUST be accompanied
  by tests that exercise the primary success path and at
  least one failure/edge case path.

- **TC-002** [MUST] Tests MUST be isolated -- each test
  MUST be independently runnable without depending on
  the execution order of other tests or on shared
  mutable state (global variables, external databases,
  temporary files from other tests).

- **TC-003** [MUST] Test assertions MUST be meaningful
  and specific. Tests that assert only "no error" or
  "not nil" without verifying the actual value or
  behavior MUST be improved to check expected outcomes.

- **TC-004** [MUST] Tests MUST NOT depend on execution
  order. Each test MUST set up its own preconditions and
  clean up its own resources. Test suites MUST pass when
  run in any order or in isolation.

- **TC-005** [SHOULD] Error paths, boundary conditions,
  and documented edge cases SHOULD have dedicated test
  cases. "Happy path only" coverage is insufficient for
  production code.

- **TC-006** [MUST] Bug fixes MUST include a regression
  test that reproduces the original failure and verifies
  the fix. The test MUST fail without the fix and pass
  with it.

- **TC-007** [SHOULD] Test names SHOULD clearly describe
  the scenario being tested, including the condition and
  expected outcome (e.g., `TestParse_EmptyInput_ReturnsError`
  rather than `TestParse2`).

- **TC-008** [SHOULD] Tests SHOULD avoid testing
  implementation details. Prefer testing observable
  behavior (inputs and outputs, side effects) over
  internal state or private method calls.

---

## Documentation Requirements

- **DR-001** [MUST] Public APIs (exported functions,
  classes, methods, types, and constants) MUST have
  documentation comments that describe purpose,
  parameters, return values, and notable error
  conditions.

- **DR-002** [SHOULD] Configuration options, environment
  variables, and feature flags SHOULD be documented in
  the project README or a dedicated configuration
  reference, including defaults, valid ranges, and
  examples.

- **DR-003** [SHOULD] User-visible changes (new features,
  breaking changes, deprecations, bug fixes) SHOULD be
  recorded in a changelog or release notes, following
  the project's established format.

- **DR-004** [MUST] Commit messages MUST be meaningful
  and describe the intent of the change, not just what
  files were modified. If the project uses Conventional
  Commits, the message MUST conform to the format
  (e.g., `feat:`, `fix:`, `docs:`).

---

## Custom Rules

<!-- This section is intentionally empty in the canonical
     pack. Project-specific custom rules belong in
     default-custom.md alongside this file. Custom rules
     use the CR-NNN identifier prefix. -->
