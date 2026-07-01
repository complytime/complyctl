## 1. Add reserved statements to proto

- [ ] 1.1 In `api/plugin/plugin.proto`, add `reserved 6;` inside `DescribeResponse`
- [ ] 1.2 Add `reserved "supports_export";` inside `DescribeResponse`
- [ ] 1.3 Run `make proto` to regenerate Go bindings
- [ ] 1.4 Run `buf lint` and confirm it passes

## 2. Remove stale collector config

- [ ] 2.1 Remove `collector:` and `endpoint: localhost:4317` from `.complytime/complytime.yaml`
- [ ] 2.2 Verify `complyctl scan` still works (no parse error on missing key)

## 3. Regenerate CRAP baseline

- [ ] 3.1 Run `make crapload-baseline`
- [ ] 3.2 Verify `.gaze/baseline.json` has no entries for deleted export functions
- [ ] 3.3 Run `make crapload-check` and confirm no regressions

## 4. Final verification

- [ ] 4.1 Run `make sanity` (vendor + format + vet + git diff check)
- [ ] 4.2 Run `make test-unit`
- [ ] 4.3 Confirm no unintended file changes
