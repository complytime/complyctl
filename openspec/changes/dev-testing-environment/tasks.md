## 1. Containerfile

- [x] 1.1 Create `.devcontainer/Containerfile` using
  `registry.fedoraproject.org/fedora:43` as base image with dnf install of
  `openscap-scanner`, `scap-security-guide`, `golang`, `git`, `make`,
  `curl`, `jq`, `tree`, and `vim-enhanced`

## 2. Post-create setup script

- [x] 2.1 Create `.devcontainer/scripts/post-create.sh` with `set -euo
  pipefail` and executable permissions (`chmod +x`). The script builds
  complyctl and mock-oci-registry from local source (`make build`).
  Each step MUST report what it is doing and exit with a descriptive
  error on failure
- [x] 2.2 Add `go install` of snappy
  (`github.com/carabiner-dev/snappy@v0.2.4`), ampel
  (`github.com/carabiner-dev/ampel/cmd/ampel@v1.2.1`), and conftest
  (`github.com/open-policy-agent/conftest@v0.68.2`) to the script.
  Versions MUST be pinned (not `@latest`). Document pinned versions in
  a comment block in the script for easy updates
- [x] 2.3 Add clone of `complytime-providers@main`, build providers, and
  copy all provider binaries (`complyctl-provider-ampel`,
  `complyctl-provider-openscap`, `complyctl-provider-opa`) to
  `~/.complytime/providers/`.
  Log the cloned commit SHA for auditability. If the clone fails
  (network issue, broken main), the script MUST exit with a clear
  error identifying the failure as an upstream dependency issue
- [x] 2.4 Add workspace setup: copy `tests/cross-repo/testdata/complytime.yaml`
  and granular policies to the test workspace directory (`~/test-workspace/`)
- [x] 2.5 Add mock OCI registry startup in background with readiness check
  (mirror the registry startup and `curl` readiness-poll pattern from
  `tests/cross-repo/cross_repo_integration_test.sh`)
- [x] 2.6 Add `GITHUB_TOKEN` least-privilege handling: capture the token
  into an internal variable, unset `GITHUB_TOKEN` from the environment
  so that `go install`, `git clone`, and `make build` do not inherit it.
  Only pass the token to `complyctl` subprocesses that need it (mirror
  the pattern from `cross_repo_integration_test.sh`). If the token is
  not set, emit a warning that `complyctl scan` requires it, but do
  not fail the script

- [x] 2.7 Add auto-rebuild hook to `.bashrc`: compare HEAD against
  `./bin/.build-commit` on shell login, rebuild if changed. Skip with
  `COMPLYCTL_SKIP_REBUILD=1`

## 3. Devcontainer configuration

- [x] 3.1 Create `.devcontainer/devcontainer.json` referencing the
  Containerfile and setting `postCreateCommand` to run the post-create
  script

## 4. Documentation

- [x] 4.1 Create `docs/TESTING_ENVIRONMENT.md` with sections covering:
  how to open from a PR in GitHub Codespaces, how to use with DevPod
  (CLI and desktop), how to use with VS Code Dev Containers extension,
  GITHUB_TOKEN configuration for each tool, what the environment
  provides (binaries, test content, mock registry), a practical command
  reference listing the available `complyctl` commands (`get`, `generate`,
  `scan`) with expected behavior and what success looks like, and
  troubleshooting steps for common issues (mock registry restart,
  GITHUB_TOKEN not set, OpenSCAP container limitations, Codespace
  suspend/resume killing background processes)
- [x] 4.2 Update `README.md` to add a link to the dev testing environment
  documentation in the existing docs section
- [x] 4.3 Update `AGENTS.md` project structure to include `.devcontainer/`
  with its sub-entries (Containerfile, devcontainer.json,
  scripts/post-create.sh)

## 5. CI Smoke Test

- [x] 5.1 Add a `make test-devcontainer` target that builds the
  Containerfile (`podman build .devcontainer/`) to verify the image
  definition is valid. This provides automated regression protection
  for the Containerfile without requiring the full post-create setup
  in CI

## 6. Verification (manual)

- [ ] 6.1 Verify the Containerfile builds: `podman build .devcontainer/`
  exits 0
- [ ] 6.2 Verify the post-create script completes: all binaries on PATH
  (`command -v complyctl snappy ampel`), mock registry responds at
  `localhost:8765/v2/`, `complytime.yaml` exists in `~/test-workspace/`
- [ ] 6.3 Verify CLI pipeline: `complyctl get` outputs
  `Synchronization completed.`, `complyctl generate --policy-id
  test-ampel-bp` outputs `Generation completed.`, `complyctl scan
  --policy-id test-ampel-bp` (with GITHUB_TOKEN set) produces scan
  results
<!-- spec-review: passed -->
<!-- code-review: passed -->
