# Homebrew Publishing Specification

## Purpose

Publish Homebrew formula updates through authenticated feature
branches and pull requests without bypassing tap protection.

## Requirements

### Requirement: FR-001 Authenticated formula publishing

The publisher MUST make its generated repository-scoped App
credential available to both API commands and Git's credential
helper during formula publication. It MUST NOT persist the
credential in remote URLs, Git configuration, or logs.

#### Scenario: Publishing with a generated App credential

- **GIVEN** a successful release and an authorized App credential
- **WHEN** a new formula is published
- **THEN** the feature-branch push authenticates with that credential
- **AND** the publisher opens a formula PR against tap main

#### Scenario: Missing publishing credential

- **GIVEN** no usable publishing credential or cached credentials
- **WHEN** the publisher attempts the feature-branch push
- **THEN** publication fails noninteractively
- **AND** the publisher does not report successful PR creation

### Requirement: FR-002 Preserve protected publishing behavior

The publisher MUST retain repository-scoped App permissions and
the existing PR-based flow. An existing open formula PR for the
version MUST remain a successful no-op. Publishing failure MUST
NOT modify an existing release tag or its published artifacts.

#### Scenario: Formula PR already exists

- **GIVEN** an open formula PR for the requested version
- **WHEN** publication runs again
- **THEN** it reports the existing PR and exits successfully
- **AND** it does not push another commit or create another PR

#### Scenario: Push fails after release creation

- **GIVEN** an existing successful release
- **WHEN** the formula branch push fails
- **THEN** the publishing step reports failure
- **AND** the existing release tag and artifacts remain unchanged
