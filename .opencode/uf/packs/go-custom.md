---
pack_id: go-custom
language: Go
version: 1.0.0
---
<!-- scaffolded by uf vdev -->

# Custom Rules: Go

Project-specific Go conventions that extend the canonical
Go convention pack. Rules in this file are loaded alongside
`go.md` by Cobalt-Crush (during implementation) and
all Divisor persona agents (during review).

Use the `CR-NNN` prefix for all custom rules. Use `[MUST]`,
`[SHOULD]`, or `[MAY]` severity indicators per RFC 2119.

## Testing Conventions (Project Override)

- **CR-001** [MUST] Use stdlib `testing` +
  `github.com/stretchr/testify` (assert/require).
  No other external assertion libraries (gomega, etc.).
  This overrides go.md TC-001.
- **CR-002** [MUST] Use `require` for fatal preconditions
  and `assert` for non-fatal checks. Use `t.Errorf`/`t.Fatalf`
  only when testify assertions are insufficient.
  This overrides go.md TC-002.
