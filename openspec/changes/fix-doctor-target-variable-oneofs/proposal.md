## Why

`complyctl doctor` and `complyctl scan` have contradictory requirements for
target variables. The OPA provider declares both `url` and `input_path` in
`RequiredTargetVariables`, so doctor demands both be present. However,
`complyctl scan` (via the OPA provider's `validateTargetVariables()`) rejects
configs where both are set, requiring exactly one. Users cannot satisfy both
commands simultaneously. Fixes #759.

## What Changes

- Add a new `repeated string optional_target_variable_groups` field (field 7)
  to the `DescribeResponse` proto message. Each entry is a pipe-delimited
  group (e.g., `"url|input_path"`) meaning "at least one of these must be
  present in each target's variables."
- Extend `pkg/provider` domain types (`DescribeResponse`) and gRPC
  translation layers to carry the new field.
- Extend `internal/doctor.ProviderHealth` with
  `OptionalTargetVariableGroups []string` and update `CheckVariables()` to
  validate one-of groups: at least one member of each group must be present
  per target instead of requiring all members.
- Update the OPA provider's `Describe()` to move `url` and `input_path`
  from `RequiredTargetVariables` to an optional group `"url|input_path"`.

## Capabilities

### New Capabilities

- `variable-oneofs`: Express mutually-exclusive (one-of) target variable
  groups in the provider Describe protocol so doctor validates them as
  "at least one required" rather than "all required."

### Modified Capabilities

## Impact

- `api/plugin/plugin.proto` — new field 7 on `DescribeResponse`
- `api/plugin/plugin.pb.go` — regenerated protobuf code
- `pkg/provider/client.go` — `DescribeResponse` struct gains field
- `pkg/provider/server.go` — gRPC adapter maps new field
- `internal/doctor/doctor.go` — `ProviderHealth` struct + `CheckVariables()`
- `internal/doctor/doctor_test.go` — new test cases for one-of groups
- `complytime-providers/cmd/opa-provider/server/server.go` — `Describe()`
  updated to use optional groups
