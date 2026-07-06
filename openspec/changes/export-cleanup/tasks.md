## 1. Add reserved statements to proto

- [x] 1.1 In `api/plugin/plugin.proto`, add `reserved 6;` inside `DescribeResponse`
- [x] 1.2 Add `reserved "supports_export";` inside `DescribeResponse`
- [x] 1.3 Run `make proto` to regenerate Go bindings
- [x] 1.4 Run `buf lint` and confirm it passes

## 2. Remove stale collector config

- [x] 2.1 Remove `collector:` and `endpoint: localhost:4317` from `.complytime/complytime.yaml`
- [x] 2.2 Verify `complyctl scan` still works (no parse error on missing key)

## 3. Regenerate CRAP baseline

- [x] 3.1 Run `make crapload-baseline`
- [x] 3.2 Verify `.gaze/baseline.json` has no entries for deleted export functions
- [x] 3.3 Run `make crapload-check` and confirm no regressions

## 4. Final verification

- [x] 4.1 Run `make sanity` (vendor + format + vet + git diff check)
- [x] 4.2 Run `make test-unit`
- [x] 4.3 Confirm no unintended file changes
