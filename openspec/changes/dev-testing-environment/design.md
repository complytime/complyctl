## Context

Maintainers reviewing PRs that affect CLI UX (command output, formatting,
error messages) need to manually test `complyctl` commands in a realistic
environment. Today, this requires cloning three repositories (complyctl,
complytime-providers, complytime-demos), setting up Vagrant with libvirt,
running Ansible playbooks, and SSH-ing into a Fedora VM. This friction
discourages thorough manual testing.

The cross-repo integration test infrastructure already provides a proven
automated setup: building both repos, installing snappy/ampel, starting
a mock OCI registry with Gemara test content, and running `complyctl get`,
`generate`, and `scan` commands. This design adapts that same setup for
interactive human use via the devcontainer standard.

## Goals / Non-Goals

**Goals:**

- Provide a one-command path from "reviewing a PR" to "testing CLI commands
  in a Fedora environment."
- Support GitHub Codespaces, DevPod, and VS Code Dev Containers through
  the open devcontainer.json standard.
- Reuse existing test infrastructure (mock OCI registry, Gemara test
  fixtures, workspace configuration) rather than creating new test content.
- Document the workflow clearly for maintainers and contributors who may
  not be familiar with devcontainers.

**Non-Goals:**

- Full system-level OpenSCAP scanning (container constraints are accepted).
- Replacing the complytime-demos Vagrant setup (it remains available for
  comprehensive testing).
- Publishing or maintaining a pre-built container image in a registry.
  Both repos build from a Containerfile at startup time to avoid registry
  maintenance overhead.
- Dedicated onboarding workflows for external contributors (the
  environment and documentation are accessible to contributors, but the
  primary testing path and examples target maintainer PR review
  scenarios).

## Decisions

### D1: Use the devcontainer.json open standard

**Decision**: Use the devcontainer.json specification (containers.dev) rather
than a custom Makefile target, Vagrant, or proprietary tooling.

**Alternatives considered**:

- *Makefile target with raw podman/docker*: Simpler but no IDE integration,
  no Codespaces support, no standard ecosystem.
- *Vagrant + libvirt*: Real VM but heavy setup, already exists in
  complytime-demos, doesn't integrate with GitHub PRs.
- *Packit + Testing Farm only*: Good for automated RPM testing but designed
  for automated test runs, not interactive sessions.

**Rationale**: devcontainer.json is vendor-neutral and supported by GitHub
Codespaces (one-click from PR), DevPod (open-source local/cloud), and
VS Code Dev Containers. One configuration file, multiple runtimes.
Maintainers choose their preferred tool.

### D2: Fedora base image from Fedora's public registry

**Decision**: Use `registry.fedoraproject.org/fedora:43` as the base image.

**Rationale**: OpenSCAP provider requires `openscap-scanner` and
`scap-security-guide`, which are Fedora/RHEL packages. Fedora's official
container registry is free, public, and doesn't require authentication.

### D3: No custom container registry

**Decision**: Build the Containerfile at devcontainer startup time. Do not
publish or maintain a pre-built image.

**Alternatives considered**:

- *Publish to ghcr.io*: Faster startup in complytime-providers (pull vs
  build), but adds a CI workflow, image versioning, and another thing to
  maintain. The Containerfile is small (Fedora + dnf install), so build
  time is acceptable.

**Rationale**: Avoids the complexity of maintaining a container image
pipeline. The Containerfile only installs a few system packages; the
heavier work (Go builds, tool installs) happens in the postCreateCommand
regardless.

### D4: Reuse cross-repo test infrastructure for workspace setup

**Decision**: The devcontainer post-create script reuses the same test
fixtures and mock OCI registry from `tests/cross-repo/testdata/` and
`cmd/mock-oci-registry/`.

**Rationale**: The cross-repo integration test already proves this setup
works. The mock registry embeds Gemara catalogs and policies, provides
an OCI-compliant endpoint, and requires no external services. Reusing it
means zero new test content to create or maintain.

### D5: Install snappy and ampel via go install

**Decision**: Install external tools using `go install` rather than
downloading pre-built binaries or using GitHub Actions.

**Alternatives considered**:

- *carabiner-dev/actions/install*: Only works in GitHub Actions, not in
  a local devcontainer.
- *Download release binaries*: Platform-specific, requires curl/wget and
  architecture detection.

**Rationale**: `go install` works identically in CI and local containers.
Go is already installed in the container. The install paths are documented
in the ampel-provider README and the quickstart spec.

### D6: Pin tool versions, clone providers from main

**Decision**: Install snappy and ampel at pinned version tags (e.g.,
`@v0.2.4`, `@v1.2.1`) rather than `@latest`. Clone
`complytime-providers` from `main` (unversioned) and build from source.

**Alternatives considered**:

- *`@latest` for all tools*: Simpler but produces non-reproducible
  environments. Two developers on different days get different versions.
  The constitution prohibits floating tags for container images, and the
  same principle applies to tool installs.
- *Download release binaries for providers*: Providers have not been
  released to a package registry yet. Building from source is the only
  available path.
- *Pin providers to a commit SHA*: Would provide reproducibility but
  creates a maintenance burden. The providers repo is actively developed
  and the devcontainer should track `main` for forward compatibility.

**Rationale**: Pinned versions for snappy/ampel provide reproducibility
and match CI's approach (the cross-repo workflow uses pinned action
SHAs). Cloning providers from `main` is an accepted trade-off: providers
are same-org, under maintainer trust, and the devcontainer's value is
testing the latest code. The cloned commit SHA is logged for
auditability.

### D7: Canonical source in complyctl, thin consumer in complytime-providers

**Decision**: The full devcontainer configuration (Containerfile,
post-create script, documentation) lives in complyctl. complytime-providers
adds its own `.devcontainer/` with a similar Containerfile and a mirrored
post-create script that builds providers locally and clones complyctl
from main.

**Rationale**: Mirrors the established pattern for cross-repo integration
tests: the canonical script lives in complyctl, complytime-providers
consumes it. complyctl is the user-facing CLI and the natural home for
the testing environment definition.

## Risks / Trade-offs

- **[Container limitations for OpenSCAP]** → Accepted. OpenSCAP system
  scans are limited in containers. The devcontainer targets CLI UX testing
  using the Ampel provider with the mock registry, which works fully in
  a container. Full OpenSCAP testing remains available via complytime-demos.

- **[Containerfile duplication across repos]** → Minimal. The Containerfile
  is ~10 lines of dnf install. Each repo's post-create logic is different
  (which binary to build from local source vs. clone from main), so some
  divergence is unavoidable and acceptable.

- **[Build time at startup]** → Building Go binaries and installing tools
  via `go install` adds 2-5 minutes to devcontainer startup on a 4-core
  machine. The devcontainer.json SHOULD specify `hostRequirements` with
  a minimum of 4 cores and 16GB RAM. The default 2-core Codespace may
  be slow or hit memory limits during Go builds. Codespaces prebuild
  could reduce startup time in the future if needed.

- **[GITHUB_TOKEN requirement]** → snappy requires a GitHub token to query
  the GitHub API for branch protection rules. Codespaces can store this as
  a secret; DevPod and local VS Code require manual export. Documentation
  must make this clear.

- **[mock-oci-registry as background process]** → The post-create script
  starts the mock registry in the background. If it crashes, `complyctl get`
  will fail with a connection error. The script includes a readiness check.
  Restarting it manually is documented. Note: Codespace suspend/resume
  kills background processes. After resuming, the mock registry must be
  restarted manually. Documentation must cover this.

- **[complytime-providers@main clone without commit pinning]** →
  Accepted trade-off. The devcontainer clones `main` of a same-org repo
  under maintainer trust. Unlike CI (which uses `actions/checkout` with
  SHA pinning), the devcontainer prioritizes testing the latest provider
  code. The post-create script logs the cloned commit SHA for
  auditability. If `main` is broken, the post-create script fails with
  a clear error identifying the upstream dependency issue. This differs
  from CI's pinned approach because the devcontainer's purpose is
  interactive testing of the latest code, not reproducible builds.

- **[GITHUB_TOKEN least-privilege]** → The post-create script follows
  the same least-privilege pattern as `cross_repo_integration_test.sh`:
  captures the token, unsets it from the environment, and only passes
  it to `complyctl` subprocesses that require it. This prevents `go
  install`, `git clone`, and `make build` from inheriting the token
  unnecessarily.

- **[Fedora base image tag pinning]** → The Containerfile uses
  `fedora:43` (a specific version tag, not `latest`). While digest
  pinning would provide full reproducibility, it creates a maintenance
  burden of tracking Fedora image updates. Tag-level pinning is an
  intentional trade-off for a dev-only environment that benefits from
  receiving security updates automatically.
