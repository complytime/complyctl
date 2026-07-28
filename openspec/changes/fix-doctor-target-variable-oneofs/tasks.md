## 1. Proto and Generated Code

- [x] 1.1 Add `optional_target_variable_groups` field 7 to `DescribeResponse` in `api/plugin/plugin.proto`
- [x] 1.2 Regenerate protobuf Go code (`make proto`)

## 2. Provider SDK Domain Types

- [x] 2.1 Add `OptionalTargetVariableGroups []string` to `pkg/provider/client.go` `DescribeResponse` struct and wire in `Describe()` method
- [x] 2.2 Map the new field in `pkg/provider/server.go` gRPC adapter

## 3. Doctor Validation

- [x] 3.1 Add `OptionalTargetVariableGroups []string` to `internal/doctor/doctor.go` `ProviderHealth` struct
- [x] 3.2 Wire the new field from provider discovery in `CheckProviders()`
- [x] 3.3 Update `CheckVariables()` to validate optional groups as at-least-one-present per target
- [x] 3.4 Update verbose mode detail output for optional groups

## 4. OPA Provider Update

- [x] 4.1 Update OPA provider `Describe()` in `complytime-providers/cmd/opa-provider/server/server.go` to move `url` and `input_path` to `OptionalTargetVariableGroups`

## 5. Tests

- [x] 5.1 Add test: target with one member of optional group passes
- [x] 5.2 Add test: target with no members of optional group fails
- [x] 5.3 Add test: target with both members of optional group passes
- [x] 5.4 Add test: unmapped target with optional groups (nil resolver)
- [x] 5.5 Add test: verbose mode shows optional group details
- [x] 5.6 Run `make test-unit` and verify all pass

## 6. Vendor

- [x] 6.1 Run `make vendor` to update vendored dependencies
