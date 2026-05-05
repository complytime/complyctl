---
description: System design principles for clean architecture
tags: [design, architecture, principles]
---

# System Design

Core design principles for the replicator codebase.

## Deep Modules

Modules should have simple interfaces and complex implementations.
A deep module hides complexity behind a clean API.

## Fight Complexity

- Reduce the number of concepts a developer must hold in mind
- Make the common case simple, the edge case possible
- Prefer explicit over implicit behavior

## SOLID Principles

- **Single Responsibility**: Each package owns one domain concept
- **Open/Closed**: Extend via interfaces, not modification
- **Liskov Substitution**: Subtypes must be substitutable
- **Interface Segregation**: Small, focused interfaces
- **Dependency Inversion**: Depend on abstractions, not concretions

## DRY

Extract only when there are 3+ duplications. Premature abstraction
is worse than duplication.

## Dependency Injection

- Constructor injection: `NewFoo(deps)` or `Options` structs
- No global state or package-level variables
- External dependencies (filesystem, network, time) behind interfaces
- Makes testing straightforward with in-memory implementations
