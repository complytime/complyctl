## ADDED Requirements

### FR-001: Devcontainer configuration provides a Fedora testing environment

The repository MUST include a `.devcontainer/` directory with a
`devcontainer.json` and `Containerfile` that together define a Fedora-based
development environment. The Containerfile MUST use
`registry.fedoraproject.org/fedora:43` as the base image and MUST install
`openscap-scanner`, `scap-security-guide`, `golang`, `git`, `make`,
`curl`, `jq`, `tree`, and `vim-enhanced` via dnf. The Containerfile
MUST set `ENV GOTOOLCHAIN=auto` so that `go install` can fetch newer
Go toolchains when dependencies require them.

#### Scenario: Devcontainer configuration is present and valid

- **GIVEN** the repository contains `.devcontainer/devcontainer.json`
  and `.devcontainer/Containerfile`
- **WHEN** a maintainer or contributor opens the repository in a
  devcontainer-compatible tool (GitHub Codespaces, DevPod, or VS Code
  Dev Containers)
- **THEN** the tool MUST detect `.devcontainer/devcontainer.json` and
  build the environment from the Containerfile

#### Scenario: Container is based on Fedora with required packages

- **GIVEN** the Containerfile has been built successfully
- **WHEN** the devcontainer finishes building
- **THEN** the running environment MUST be Fedora-based with
  `openscap-scanner` and `scap-security-guide` available as installed
  packages

### FR-002: Post-create setup produces a ready-to-test environment

The devcontainer MUST define a post-create command (script) that
automatically builds complyctl from the local source, installs snappy,
ampel, and conftest via `go install` at pinned version tags, clones and
builds
complytime-providers from main, copies provider binaries to the
discovery path (`~/.complytime/providers/`), configures a test
workspace with Gemara content, and starts the mock OCI registry in the
background. The post-create script MUST exit with code 0 on success
and non-zero with a descriptive error message if any setup step fails.
The script MUST use strict shell error handling (`set -euo pipefail`).
The script MUST NOT pass `GITHUB_TOKEN` to subprocesses that do not
require it (least-privilege: capture, unset, selectively inject).

#### Scenario: All binaries are built and available

- **GIVEN** the devcontainer has been built from the Containerfile
- **WHEN** the devcontainer post-create command completes successfully
- **THEN** `complyctl` and `mock-oci-registry` MUST be available in
  `./bin/` (from `make build`), `snappy`, `ampel`, and `conftest` MUST
  be available in `$GOPATH/bin`, and `complyctl-provider-ampel`,
  `complyctl-provider-openscap`, and
  `complyctl-provider-opa` MUST be present in
  `~/.complytime/providers/`

#### Scenario: Test workspace is configured

- **GIVEN** the post-create command has completed successfully
- **WHEN** the devcontainer post-create command completes
- **THEN** a `complytime.yaml` workspace configuration MUST exist in
  the test workspace directory (`~/test-workspace/`), pointing to the
  mock OCI registry on localhost

#### Scenario: Mock OCI registry is running

- **GIVEN** the post-create command has completed successfully
- **WHEN** the devcontainer post-create command completes
- **THEN** the mock OCI registry MUST be running and serving Gemara
  catalogs and policies on the default port (8765)

#### Scenario: Post-create script fails gracefully on setup errors

- **GIVEN** a setup step fails (e.g., `go install`, `git clone`,
  `make build`)
- **WHEN** the post-create script encounters the failure
- **THEN** the script MUST report which step failed and exit with a
  non-zero status code

#### Scenario: Post-create succeeds without GITHUB_TOKEN

- **GIVEN** the environment does NOT have `GITHUB_TOKEN` set
- **WHEN** the devcontainer post-create command runs
- **THEN** the script MUST complete successfully (exit code 0) and
  emit a warning that `GITHUB_TOKEN` is required for `complyctl scan`

#### Scenario: complyctl get and generate work end-to-end

- **GIVEN** the post-create command has completed successfully
- **WHEN** a maintainer or contributor runs `complyctl get` followed by
  `complyctl generate --policy-id test-ampel-bp` in the test workspace
- **THEN** both commands MUST complete successfully with expected output

#### Scenario: complyctl scan works when GITHUB_TOKEN is configured

- **GIVEN** the environment has `GITHUB_TOKEN` set with read access to
  public repositories
- **WHEN** a maintainer or contributor runs
  `complyctl scan --policy-id test-ampel-bp` in the test workspace
- **THEN** the command MUST complete successfully and produce scan
  results

#### Scenario: Auto-rebuild on source change

- **GIVEN** the devcontainer is running and the user checks out a
  different branch (e.g., a contributor's PR)
- **WHEN** the user opens a new shell session inside the container
- **THEN** the environment MUST detect the source change (different
  HEAD commit) and automatically rebuild complyctl. The user MUST be
  able to skip the auto-rebuild by setting `COMPLYCTL_SKIP_REBUILD=1`

#### Scenario: complyctl scan fails gracefully without GITHUB_TOKEN

- **GIVEN** the environment does NOT have `GITHUB_TOKEN` set
- **WHEN** a maintainer or contributor runs
  `complyctl scan --policy-id test-ampel-bp` in the test workspace
- **THEN** the command MUST exit with a non-zero code and output a
  message indicating the token is required

### FR-003: Documentation explains workflows for maintainers and contributors

The repository MUST include documentation in `docs/` that explains how
to use the devcontainer for PR review testing and interactive CLI
exploration. The documentation MUST cover GitHub Codespaces, DevPod,
and VS Code Dev Containers workflows. The documentation MUST explain
the GITHUB_TOKEN requirement for scan commands that use snappy. The
documentation MUST include a practical command reference showing the
commands available in the environment, their expected behavior, and
what success looks like. The `devcontainer.json` MUST include
`--security-opt label=disable` in `runArgs` for SELinux compatibility
with podman rootless. The documentation MUST explain how to configure
DevPod inactivity timeout and how to resume a stopped container.

#### Scenario: Documentation is discoverable from README

- **GIVEN** the repository README contains a Documentation section
- **WHEN** a maintainer or contributor reads the repository README
- **THEN** they MUST find a link or reference to the dev testing
  environment documentation

#### Scenario: Documentation covers all supported tools

- **GIVEN** the dev testing environment documentation exists
- **WHEN** a maintainer or contributor reads the documentation
- **THEN** they MUST find instructions for using the devcontainer with
  GitHub Codespaces, DevPod, and VS Code Dev Containers

#### Scenario: Documentation includes a practical command reference

- **GIVEN** the devcontainer setup has completed
- **WHEN** a maintainer or contributor opens the dev testing environment
  documentation
- **THEN** they MUST find a command reference section listing the
  available `complyctl` commands (`get`, `generate`, `scan`), the
  expected behavior of each, and troubleshooting steps for common
  issues (mock registry restart, GITHUB_TOKEN not set, OpenSCAP
  container limitations)

#### Scenario: GITHUB_TOKEN setup is documented

- **GIVEN** a maintainer or contributor wants to run `complyctl scan`
  in the devcontainer
- **WHEN** they read the dev testing environment documentation
- **THEN** the documentation MUST explain how to configure GITHUB_TOKEN
  for each supported tool (Codespaces secrets, DevPod env, manual
  export)
