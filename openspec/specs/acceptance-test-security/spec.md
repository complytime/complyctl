# Acceptance Test Security Specification

## Purpose

Keeps acceptance-test registry checks compatible with trusted test
infrastructure while restricting generated configuration to its owner.

## Requirements

### Requirement: FR-001 Preserve registry preflight behavior

Acceptance preflight MUST retain access to the runner-configured
registry, including private Compose endpoints, and its seeded-policy
check. Resolving the two reported G704 findings MUST NOT change its
timeout or existing failure behavior.

#### Scenario: Configured test registry is available

- **GIVEN** the runner configures a reachable private test registry
  and a seeded lifecycle policy
- **WHEN** acceptance preflight runs
- **THEN** the registry and seeded-policy requests succeed
  and the test suite proceeds

#### Scenario: Registry or seeded policy is unavailable

- **GIVEN** the configured registry is unreachable or the
  seeded-policy check fails
- **WHEN** acceptance preflight runs
- **THEN** it retains the existing fatal diagnostic and nonzero exit

### Requirement: FR-002 Restrict generated configuration permissions

The lifecycle and verification configuration helpers MUST create
their workspace configuration files with owner-only read/write
permissions. The test process and its CLI child MUST retain access
to the generated configuration.

#### Scenario: Both helpers create configuration

- **GIVEN** a fresh test workspace for each helper
- **WHEN** lifecycle or verification configuration is written
- **THEN** the file grants no group or other permissions and remains
  readable by the test process and its CLI child

### Requirement: FR-003 Resolve the four acceptance lint findings

The fix MUST resolve the two G704 preflight findings and two G306
configuration-write findings with existing lint rules enabled. Any
G704 exception MUST be specific to the reported request and explain
its runner-controlled test inputs. No package-wide Go lint exclusion
or change to Go scanner configuration is permitted. The separate
`yaml-lint-scope` capability defines the permitted YAML exclusions.

#### Scenario: Repeat acceptance-tagged lint

- **GIVEN** recorded tool versions and comparable before/after
  acceptance-tagged lint runs
- **WHEN** the fixed test sources are linted
- **THEN** those four findings are resolved without changing
  Go scanner configuration or suppressing unrelated Go findings
