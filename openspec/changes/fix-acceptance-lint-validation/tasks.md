# Tasks

## 1. Define Validation Profiles

- [ ] 1.1 Inventory the CI lint, YAML, pre-commit, and acceptance security commands and record their tool versions, flags, build tags, and input scope; verify the inventory against `.github/workflows/` and current tool configuration.
- [ ] 1.2 Add named Make targets for CI-parity style/lint validation and acceptance-tagged security lint; verify each target prints or executes its documented tool, flags, tags, and file scope.
- [ ] 1.3 Add automated coverage for validation target scope, including vendored and generated style inputs; verify the tests fail when intended exclusions or acceptance security lint execution are removed.
- [ ] 1.4 Document the validation profiles and their intended CI relationship in contributor documentation; verify every documented command runs as written in a clean checkout.

## 2. Resolve Acceptance Security Baseline

- [ ] 2.1 Evaluate the two G704 acceptance registry requests against the Compose test-infrastructure trust model and implement narrow fixes or reviewed dispositions; verify acceptance-tagged gosec reports no unresolved G704 baseline finding.
- [ ] 2.2 Resolve the two G306 acceptance test-configuration permission findings without weakening unrelated file handling; verify acceptance-tagged gosec reports no unresolved G306 baseline finding and the acceptance tests retain their required configuration access.
- [ ] 2.3 Add or update acceptance tests for any changed registry request or test-configuration behavior; verify the affected acceptance test suite passes.
- [ ] 2.4 Document any retained security disposition and its trust boundary; verify the documentation identifies the affected validation profile and does not introduce a blanket suppression.

## 3. Verify Parity and Regression Protection

- [ ] 3.1 Run the CI-parity validation profile on the baseline scope; verify it excludes only documented vendored and generated style inputs and reports no unexpected baseline failures.
- [ ] 3.2 Run the acceptance security-lint profile and the affected acceptance tests; verify security analysis remains enabled and all resolved findings stay fixed.
- [ ] 3.3 Run repository lint, formatting, and relevant CI-equivalent checks; verify the change introduces no lint, formatting, or validation regression.

## Workflow follow-up

- Run the required review workflow after implementation and before creating a pull request.
- Archive the change after the project's review requirements are satisfied.
