## Context

The `DescribeResponse` proto message has a flat `repeated string
required_target_variables` field. Providers list every variable name they
need, and `CheckVariables()` treats the list as "all must be present."
The OPA provider needs exactly one of `url` or `input_path` (mutually
exclusive), but the current protocol cannot express that. Doctor therefore
demands both, while scan rejects both.

## Goals / Non-Goals

**Goals:**

- Allow providers to declare groups of mutually-exclusive target variables
  where at least one member must be present.
- Keep the change backward-compatible: providers that do not set the new
  field behave identically to today.
- Fix the OPA provider so `doctor` and `scan` agree on valid configs.

**Non-Goals:**

- Redesigning the entire variable validation model.
- Adding mutual-exclusivity enforcement at scan time (the provider already
  handles that in its own `validateTargetVariables()`).
- Global variable one-of groups (no use case currently exists).

## Decisions

### D1: Pipe-delimited string groups in proto

Add `repeated string optional_target_variable_groups` (field 7) to
`DescribeResponse`. Each element is a pipe-delimited group, e.g.
`"url|input_path"`. Doctor splits on `|` and checks that at least one
member is present per target.

**Alternative considered**: A nested proto message
`message VariableGroup { repeated string names = 1; }` with
`repeated VariableGroup`. Rejected because it adds message complexity
for a feature that only needs simple string lists, and the pipe-delimiter
convention is already human-readable and unambiguous (variable names
cannot contain `|`).

### D2: Doctor validation semantics

For each optional group, `CheckVariables()` checks whether *at least one*
member of the group is present in the target's `Variables` map. If none
are present, it reports the group as missing (StatusFail). If at least one
is present, the group counts as resolved.

### D3: OPA provider update

Move `url` and `input_path` from `RequiredTargetVariables` to
`OptionalTargetVariableGroups: ["url|input_path"]`. Keep other required
variables (like `scan_path`) in `RequiredTargetVariables`.

## Risks / Trade-offs

- [Proto field addition] Adding field 7 is wire-compatible. Old clients
  ignore it; old providers don't set it. No migration needed.
- [Pipe delimiter] If a variable name ever contains `|`, parsing breaks.
  Mitigation: variable names are short identifiers; `|` is not a valid
  identifier character. No current or foreseeable use case.
