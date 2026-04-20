<!--
Sync Impact Report:
Version: 1.0.2 → 1.1.0 → 1.2.0 (Agent-optimized wording, repo-agnostic refactor, RFC 2119 normalization, org-infra alignment)
Modified Principles:
  - III. Incremental Improvement: Reframed from human-motivational to agent-enforceable MUST rules
  - IV. Code is Written for Humans First → IV. Readability First: Retitled and tightened intro
  - V. Do Not Reinvent the Wheel: Replaced human workflow advice with declarative constraints
  - I, II, VI, VII: Normalized bare imperatives to RFC 2119 (MUST/SHOULD/MAY)
Added Sections:
  - Lint Configuration Awareness (under Guidelines for All Programming Languages)
  - Incrementing This Constitution (under Governance)
Removed Sections: N/A
Other Changes:
  - Removed repository-specific references (complyctl, complyscribe, direct file links)
  - Normalized all directives across Contribution Workflow, Coding Standards, and Governance to RFC 2119
  - Tightened vague language ("sensible defaults", "one sitting", etc.)
  - Added "code agents" to Governance applicability statement
Templates requiring updates:
  ⚠ plan-template.md - Constitution Check gates should reflect retitled Principle IV (Readability First)
  ✅ spec-template.md - No changes needed (generic template)
  ✅ tasks-template.md - No changes needed (generic template)
  ✅ checklist-template.md - No changes needed (generic template)
Follow-up TODOs:
  - plan-template.md: Verify Constitution Check references updated Principle IV title
-->

# ComplyTime Constitution

## Core Principles

### I. Single Source of Truth (Centralized Constants)

Values used in multiple places or that may change over time MUST be centralized. Magic strings (e.g., `"active"`, `"https://api..."`) and magic numbers (e.g., `86400`) MUST NOT appear inline within logic. These values MUST be moved into dedicated files (e.g., `internal/consts/consts.go`, `settings.py`).

**Rationale**: Prevents divergence -- updating a timeout from 30s to 60s in one file ensures every part of the application updates automatically. Avoids "shotgun surgery" -- a single logical change MUST NOT require search-and-replace across multiple files, reducing the risk of missed instances and introduced bugs.

### II. Simplicity & Isolation

Code MUST reduce complexity to improve security and maintainability. Functions MUST follow the Single Responsibility Principle -- small, focused, and independently testable. Prefer isolated, composable parts over monolithic and inflexible approaches.

**Rationale**: Small, isolated components reduce cognitive load, improve testability, and minimize the blast radius of changes.

### III. Incremental Improvement

Improvements to existing code are welcome, but each contribution MUST remain focused on a single concern. Changes unrelated to the current task (refactoring, formatting fixes, better naming) MUST be proposed in a separate commit or Pull Request. Before including incidental improvements, assess their impact on other core principles and justify the scope expansion.

**Rationale**: Keeping aesthetic changes separate from logic fixes ensures that PRs remain atomic and easier to review.

### IV. Readability First

Code is read far more often than it is written. All code MUST prioritize clarity for the reader over brevity or convenience for the writer.

**Implementation**:
- **Explicit Naming**: Variable and function names MUST clearly describe their intent (e.g., use `days_until_expiration` instead of `d`).
- **Avoid "Clever" Code**: MUST NOT use complex one-liners or obscure language features that require deep mental parsing. If the implementation is hard to explain, it is a bad implementation.
- **Self-Documenting**: The code structure itself MUST explain the logic. Comments MUST explain the *why* (business logic/intent), not the *what* (syntax).

**Rationale**: Readable code reduces onboarding time, prevents bugs from misunderstanding, and enables faster debugging.

### V. Do Not Reinvent the Wheel

Leverage existing solutions and validate their quality.

**Implementation**:
- **Prefer Established Libraries**: MUST NOT introduce custom implementations when a well-established, actively maintained library or cloud-native solution exists for the same purpose.
- **Vet Dependencies**: New dependencies MUST be actively maintained (recent release activity) and have a clear governance model. SHOULD NOT adopt abandoned or single-maintainer projects for critical functionality.
- **No Hard Forks**: MUST NOT create hard forks or permanent local workarounds for upstream libraries. If upstream changes are needed, they SHOULD be contributed back in a separate effort.

**Rationale**: Using well-maintained libraries reduces maintenance burden. Contributing back improves the ecosystem and reduces the technical debt of maintaining internal patches.

### VI. Composability (The Unix Philosophy)

Programs and functions MUST do one thing and do it well. Programs and functions MUST be designed to work together. All tools MUST be modular. Output from one tool MUST be easily consumable as input for another (e.g., standard JSON/YAML streams).

**Rationale**: Modular tools enable composition, reuse, and integration with external systems. Standard formats ensure interoperability.

### VII. Convention Over Configuration

Decrease the number of decisions a developer or user needs to make. Provide defaults aligned with the most common use case. Users SHOULD only need to specify configuration when deviating from the established standard.

**Rationale**: Reduces cognitive load, accelerates onboarding, and prevents configuration errors through well-chosen defaults.

## Repository Structure & Standards

Every repository under the ComplyTime organization MUST contain the following standard files in the root directory to ensure a consistent developer experience:

| File | Description | Standard |
|:---|:----|:----|
| `README.md` | Project overview, installation, and usage. | Markdown |
| `LICENSE` | Legal terms of use. | **Apache License 2.0** |
| `CONTRIBUTING.md` | Guidelines for contributors. | Link to org-wide guide or repo-specific details. |
| `CODE_OF_CONDUCT.md` | Community standards. | Standard Contributor Covenant |
| `SECURITY.md` | Security policy. | Vulnerability reporting instructions |
| `.github/` | GitHub configuration. | Issue templates, PR templates, workflows. |

These files MUST link to org-wide definitions whenever available and MAY be incremented with repository-specific content.

## Contribution Workflow

### Branching Strategy

- **Main Branch**: `main` is the stable production branch.
- **Feature Branches**: All changes MUST be developed on branches created from `main`.

### Pull Requests (PRs)

- **Atomic Changes**: PRs MUST address a single concern and be small enough for focused review. Large, multi-concern PRs SHOULD be split into separate submissions.
- **Review Requirement**: All PRs REQUIRE review from at least two Maintainers.
- **CI/CD Gates**:
  - **Standard**: All PRs MUST generally pass automated checks (linting, testing, build) before merging.
  - **Exceptions**: Checks MAY occasionally fail due to external issues or transient flakes. In these rare instances, maintainers MAY agree on exceptions to merge specific PRs despite a failing status.
- **Pull Request Title Format**: `<type>: <description>` (e.g., `feat: implement oscal validation logic`)

### Commit Messages

All commit messages MUST follow the **Conventional Commits** [specification](https://www.conventionalcommits.org/).

### Commit Trailers

- **Signed-off-by**: All commits MUST include a `Signed-off-by` trailer with the author's name and email (use `git commit -s`). This certifies the contributor's right to submit the code under the project's license ([Developer Certificate of Origin](https://developercertificate.org/)).
- **Assisted-by**: Commits that were authored or substantially assisted by an AI tool MUST include an `Assisted-by` trailer identifying the tool and model (e.g., `Assisted-by: OpenCode (claude-opus-4-6)`). This ensures transparency and traceability of AI-assisted contributions.

## Infrastructure Standards Centralization

Workflows, configurations, and templates SHOULD be centralized in the [org-infra](https://github.com/complytime/org-infra) repository, which serves as the canonical source for organization-wide CI/CD standards. Repository-specific overrides MAY exist but MUST NOT conflict with centralized definitions. Changes to shared infrastructure MUST be proposed in org-infra first.

## Coding Standards

### Guidelines for All Programming Languages

- **Empty Line at End of File**: All files MUST end with a single empty line. This ensures clean version control diffs and adheres to POSIX standards.
- **Pre-commit Hooks**: Repositories SHOULD configure pre-commit and pre-push hooks via [pre-commit](https://pre-commit.com/).
- **Makefile**: Repositories MUST use a Makefile to centralize code-specific commands.
- **Testing**: All code MUST have tests. Test functions MUST use descriptive names and include edge cases. Inputs from external sources MUST be tested. Each test scenario MUST include at least one positive and one negative test case to verify that errors and exceptions are properly handled.
- **Line Length**: Lines in source code MUST be limited to 99 characters unless exceeding the limit demonstrably improves readability. YAML files are exempt and follow the `.yamllint.yml` configuration instead.
- **Lint**: Code MUST have zero lint issues according to the lint configuration defined in the repository. No trailing spaces.
- **Lint Configuration Awareness**: Before making code changes, agents MUST read the repository's lint and formatter configuration files to understand the enforced rules. All generated or modified code MUST conform to these configurations. If no lint configuration is present, agents SHOULD follow the language-specific defaults defined in this constitution. The ComplyTime organization standardizes on the following configuration files:
  - `.golangci.yml` -- Go linting rules
  - `ruff.toml` or `pyproject.toml` `[tool.ruff]` -- Python linting rules
  - `.mega-linter.yml` -- Multi-language linting in CI
  - `.pre-commit-config.yaml` -- Pre-commit and pre-push hook definitions
  - `.yamllint.yml` -- YAML linting rules

**Note**: The following language-specific sections apply to repositories containing source code in the respective language. Repositories that host configuration files as sync templates (e.g., `.golangci.yml` in org-infra) are not subject to these sections for their own code.

### Go

#### General Guidelines

- **File Naming**: File names MUST use lowercase letters and underscores (e.g., `my_file.go`).
- **Package Names**: Package names MUST be short, concise, and lowercase. MUST NOT use underscores or mixed caps.
- **Error Handling**: Errors MUST always be checked and handled appropriately. Errors SHOULD be returned to the caller when the current function cannot resolve them.

#### Licensing and File Headers

```go
// SPDX-License-Identifier: Apache-2.0
```

#### Code Formatting

Formatting SHOULD be aligned with native go format tools, [`goimports`](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) and [`go fmt`](https://go.dev/blog/gofmt).

#### Additional Guidelines

Repositories SHOULD define Go-specific lint rules (e.g., via `.golangci.yml`) and run them in CI/CD. These checks SHOULD also be run locally before submitting a PR.

### Python

#### General Guidelines

- **Type Hinting**: All Python code MUST use type hints to improve readability and tooling support.

#### Licensing and File Headers

```python
# SPDX-License-Identifier: Apache-2.0
```

#### Code Formatting

- **Style**: Code MUST be formatted with `black` and `isort`, or equivalently with `ruff format` and `ruff check --select I`.
- **Lint**: Code MUST pass `ruff` linting.
- **Static type check**: Code SHOULD pass `mypy` static type checking. Repositories with minimal scripting (e.g., a single utility script) MAY omit `mypy` if type hints are present.
- **Non-Python files**: SHOULD use [Megalinter](https://github.com/oxsecurity/megalinter) or equivalent to lint non-Python files in a CI task. Repository-specific configuration details belong in each repository's own constitution increment.

### YAML / GitHub Actions Workflows

#### Naming Conventions

- **Reusable workflows**: MUST be prefixed with `reusable_` and have a clear, descriptive name reflecting their function (e.g., `reusable_vuln_scan.yml`).
- **Consumer workflows**: Workflows that call reusable workflows MUST be prefixed with `ci_` (e.g., `ci_security.yml`).

#### Security

- Workflows MUST follow the **Principle of Least Privilege**. Write permissions MUST be avoided; when necessary, they MUST be defined in the minimal possible scope.
- Prefer defining explicit `permissions` per Job over workflow-level permissions.
- Secrets MUST NOT be hardcoded in workflow files. Use GitHub Secrets or environment-based injection.

#### Design

- Reusable workflows MUST be generic enough to be consumed by any repository within the organization.
- Workflow inputs MUST have descriptive `description` fields.
- Required inputs MUST be marked with `required: true`.
- Optional inputs SHOULD provide sensible `default` values.

#### Formatting

- YAML files MUST be linted with `yamllint` using the repository's `.yamllint.yml` configuration.
- YAML line length is governed by the `.yamllint.yml` configuration and is exempt from the 99-character general rule.
- Workflow files MUST include a header comment block describing the workflow's purpose.

### Containers

#### Image Building

- Container images MUST be built using the org-infra reusable workflow (`reusable_publish_ghcr.yml`) or an equivalent workflow that produces the same supply chain artifacts.
- Containerfiles MUST use a specific base image tag or digest. MUST NOT use `latest` or floating tags.
- Multi-stage builds SHOULD be used to minimize final image size and attack surface.

#### Supply Chain Security

- All container images MUST include the following attestations:
  - **SLSA provenance**: Generated during the build process.
  - **SBOM**: Software Bill of Materials attached to the image.
  - **Vulnerability scan**: Scan results attested to the image.
- All attestations MUST be signed using Sigstore keyless signing with Rekor transparency log entries.
- Signature verification MUST validate certificate identity and OIDC issuer.

#### Registry Standards

- Primary registry: GitHub Container Registry (`ghcr.io`).
- Promotion to secondary registries (e.g., Quay.io) MUST verify source image signatures before copying.
- OCI labels (`org.opencontainers.image.*`) SHOULD be applied for description, vendor, source, and version metadata.

## Governance

This constitution supersedes all other practices and serves as the central source of truth for engineering standards, contribution workflows, and architectural principles for the ComplyTime organization. All contributors, maintainers, and code agents MUST adhere to these guidelines to ensure consistency, quality, and compliance.

### Amendment Procedure

- Amendments REQUIRE documentation of the rationale and impact assessment.
- Amendments MUST be reviewed by at least two Maintainers.
- Version MUST increment according to semantic versioning:
  - **MAJOR**: Backward incompatible governance/principle removals or redefinitions.
  - **MINOR**: New principle/section added or materially expanded guidance.
  - **PATCH**: Clarifications, wording, typo fixes, non-semantic refinements.

### Compliance Review

- All PRs/reviews MUST verify compliance with this constitution.
- Complexity MUST be justified when deviating from principles.
- This constitution MUST be used as the authoritative reference for runtime development guidance and decision-making.

### Incrementing This Constitution

This constitution is the org-wide shared reference for all ComplyTime repositories, maintained canonically in the `org-infra` repository.

**For org-infra maintainers**: Changes to this file follow the Amendment Procedure above.

**For all other repositories**: Individual repositories MUST NOT modify the org-wide constitution file. Instead, each repository SHOULD create its own `.specify/memory/constitution.md` that:

1. References this org-wide constitution as its base authority.
2. Adds repository-specific standards (e.g., additional lint rules, architecture constraints, technology-specific conventions) that do not conflict with the principles defined here.
3. MAY tighten org-wide SHOULD directives to MUST for that repository's context, but MUST NOT relax org-wide MUST directives.

Repository-level constitutions are incremental -- they extend, not replace, this document. In case of conflict, this org-wide constitution takes precedence.

**Version**: 1.2.0 | **Ratified**: 2026-02-25 | **Last Amended**: 2026-03-18
