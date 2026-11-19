# Go Project Style Guide

This style guide outlines the best practices to ensure consistency and readability across the codebase.

## ComplyTime Organization Style Guide

Refer to [Style Guide](https://github.com/complytime/community/blob/main/STYLE_GUIDE.md), this is the universal style guide that all projects under the ComplyTime Organization should follow.

## Project Style Guide

### Code Formatting

- **Braces**: Place opening braces on the same line as the statement (e.g., `if`, `for`, `func`).

### Additional Guidelines

- Other [Go checks](https://github.com/complytime/complyctl/blob/main/.golangci.yml) are present in CI/CD and therefore it may be useful to also run them locally before submitting a PR.
- The pre-commit and pre-push hooks can be configured by installing [pre-commit](https://pre-commit.com/) and running `make dev-setup`
- Complyctl leverages the [charmbracelet/log](https://github.com/charmbracelet/log) library for logging all command and provider activity. By default, this output is printed to stdout.

### Verify Hook Scope

Run the scope regression suite to check that project hooks preserve upstream
content while still detecting first-party defects. It invokes real pinned hooks
in temporary normal Git repositories, not stubs, and isolates `PRE_COMMIT_HOME`
for each test session. Missing prerequisites, setup failures, and timeouts fail
the suite rather than count as skips or successful verification.

Prerequisites are Git, Make, Python **3.10 or newer** (both `python3` for pytest
and `python` for the local SPDX hook), pytest installed for `python3`, and
pre-commit **3.2.0 or newer** for the configured stage names. Pytest is a
development/test prerequisite, not a complyctl runtime dependency. Use Go
**1.26.10**, as declared in `go.mod`, and its bundled `gofmt`; recheck `go.mod`
when updating the toolchain. Record the resolved versions from the repository
root before verification:

```bash
git --version
python3 --version
python --version
python3 -m pytest --version
pre-commit --version
export PATH="$(go env GOROOT)/bin:$PATH"
go version
go version "$(go env GOROOT)/bin/gofmt"
```

The pinned hook repositories and their environments must initialize successfully.
Initial setup can require network access to fetch the revisions in
`.pre-commit-config.yaml` and install their dependencies. Hook runs below initialize
those environments on demand; the regression suite uses its own isolated cache.
From the repository root, run:

```bash
make test-pre-commit-scope
```

#### Scope and Stages

Only `trailing-whitespace`, `end-of-file-fixer`, `go-fmt`, and `check-spdx`
exclude the repository-root `vendor/` tree. The whitespace/EOF hooks also
preserve exactly these three root-relative scaffolded command paths:

- `.opencode/commands/uf.review-council.md`
- `.opencode/commands/uf.review-pr.md`
- `.opencode/commands/uf.address-feedback.md`

These are whitespace/EOF-only exceptions, not formatting repairs. The `go-fmt`
and SPDX exclusions remain vendor-only. All other commands, path lookalikes,
nested copies, and archived documents remain eligible for whitespace/EOF checks;
there is no directory-wide command or archive exemption. Vendor-like maintained
paths such as `internal/vendor_helpers.go` remain checked. SPDX still checks
first-party Go, Python, and shell headers; direct calls to the SPDX script do
not apply the hook's vendor exclusion. Unrelated controls, including `check-yaml`,
`check-added-large-files`, `go-unit-tests`, and security scanners, retain their
existing scope and rules.

Verify whitespace, EOF, and `go-fmt` at **pre-push**, and SPDX at **pre-commit**.
The pinned whitespace/EOF manifests additionally enable **pre-commit** and
**manual**; these stages remain enabled despite the repository's pre-push
default. A wrong-stage rejection or a "no files to check" skip does not prove
that a hook executed. Include eligible first-party controls when testing scope.

#### Staged and All-Files Commands

Run these mutating checks only in a disposable **normal Git checkout**, from
its root, with the candidate configuration, SPDX script, and intended inputs
copied in. Include intended uncommitted changes explicitly: a clone of HEAD
alone will not contain them. Do not install or run these hooks in the actual
developer checkout during verification. Use an isolated cache for the commands:

```bash
export PRE_COMMIT_HOME="$(mktemp -d)"
pre-commit validate-config

# Staged selection: prepare eligible staged inputs in the disposable checkout.
pre-commit run trailing-whitespace --hook-stage pre-push --verbose
pre-commit run end-of-file-fixer --hook-stage pre-push --verbose
pre-commit run go-fmt --hook-stage pre-push --verbose
pre-commit run check-spdx --hook-stage pre-commit --verbose

# All tracked files, including defects outside the staged selection.
pre-commit run trailing-whitespace --hook-stage pre-push --all-files --verbose
pre-commit run end-of-file-fixer --hook-stage pre-push --all-files --verbose
pre-commit run go-fmt --hook-stage pre-push --all-files --verbose
pre-commit run check-spdx --hook-stage pre-commit --all-files --verbose

# Preserve and exercise the additional whitespace/EOF manifest stages.
for stage in pre-commit manual; do
    for hook in trailing-whitespace end-of-file-fixer; do
        pre-commit run "$hook" --hook-stage "$stage" --verbose
        pre-commit run "$hook" --hook-stage "$stage" --all-files --verbose
    done
done
```

Without `--all-files` or `--files`, pre-commit selects staged paths and
temporarily stashes unstaged edits. A passing staged run says nothing about
unknown defects outside that selection. `--files` explicitly selects named
paths; it is **not** staged-only verification. `--all-files` selects tracked
files and checks their working-tree content, not untracked files. To demonstrate
the difference, keep a defect in a committed file and stage only a separate
compliant change in the disposable fixture: staged selection should pass,
while all-files selection should detect the defect. Use fresh fixtures per
hook/stage so earlier repairs cannot mask findings.

Repairs return nonzero and modify files. Inspect exit statuses and changed paths,
verify vendor and exempt command bytes remain unchanged, and restage intended
repairs **only in the disposable checkout** before checking second-run success.
Record unresolved first-party findings rather than broadening exclusions.
These per-hook commands do not run unrelated Go tests; full-stage repository
verification must separately cover both pre-commit and pre-push. An unqualified
all-files run alone does not cover both. See the
[hook-scope design](../openspec/changes/fix-pre-commit-vendor-scope/design.md)
for fixture coverage and baseline-finding procedures.
