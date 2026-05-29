## Why

Manual testing of CLI UX changes requires maintainers to set up a complex
multi-repository environment (complyctl + complytime-providers + complytime-demos)
with Vagrant, Ansible, and libvirt. This friction discourages thorough manual
testing during PR reviews, especially for CLI output and user experience changes
that automated tests cannot fully validate. As the project receives more external
contributions, maintainers need a one-command path to a ready-to-test Fedora
environment for any PR.

## What Changes

- Add a `.devcontainer/` configuration to complyctl providing a Fedora-based
  development and testing environment with all dependencies pre-installed.
- The devcontainer uses the open `devcontainer.json` standard, compatible with
  GitHub Codespaces, DevPod, and VS Code Dev Containers.
- A `postCreateCommand` script automates the full setup: builds complyctl and
  mock-oci-registry from the local source, installs snappy and ampel via
  `go install` at pinned version tags, clones and builds
  complytime-providers from `main`, copies provider binaries to the
  discovery path, and configures a test workspace with Gemara content
  via the mock OCI registry.
- Add documentation in `docs/` explaining how maintainers and contributors
  use the devcontainer for PR review testing and interactive CLI exploration,
  covering Codespaces, DevPod, and local VS Code workflows, with a practical
  command reference and troubleshooting section.
- Update `README.md` with a reference to the new documentation.

## Capabilities

### New Capabilities

- `dev-testing-environment`: Devcontainer configuration and setup automation
  providing a one-command Fedora environment for interactive CLI testing
  during PR reviews.

## Impact

- **New files**: `.devcontainer/Containerfile`, `.devcontainer/devcontainer.json`,
  `.devcontainer/scripts/post-create.sh`, `docs/TESTING_ENVIRONMENT.md`
- **Modified files**: `README.md` (add link to new docs)
- **Dependencies**: Uses `registry.fedoraproject.org/fedora:43` as the base
  container image. Installs `openscap-scanner`, `scap-security-guide`, `curl`,
  `jq`, `tree`, and `vim-enhanced` via dnf. Installs `snappy` (v0.2.4),
  `ampel` (v1.2.1), and `conftest` (v0.68.2) via `go install` at pinned
  versions. Clones `complytime-providers` from GitHub at build time.
- **No changes to existing code**: This is purely additive infrastructure.
- **Downstream**: complytime-providers will add its own thin devcontainer
  configuration in a follow-up change, consuming the same pattern.
