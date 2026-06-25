# Dev Testing Environment

complyctl includes a Fedora-based devcontainer that provides a
one-command path to interactive CLI testing during PR reviews.
The environment comes pre-built with all binaries, a mock OCI
registry loaded with test content, and a ready-to-use workspace
so reviewers can immediately exercise `complyctl` commands
against realistic policy data.

## Prerequisites

You need one of the following tools installed to open the
devcontainer. If none are installed, pick the one that fits
your workflow:

- **GitHub Codespaces** -- no local setup required; works
  directly from a PR on GitHub. Best for quick PR review
  testing.
- **DevPod** -- open-source, runs locally or on remote
  providers. Install from
  [devpod.sh/docs/getting-started/install](https://devpod.sh/docs/getting-started/install).
  Requires a container runtime (`podman` or `docker`).
- **VS Code** with the
  [Dev Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
  extension. Requires a container runtime (`podman` or
  `docker`).

The devcontainer builds a Fedora image and runs a post-create
script that sets up all binaries and test content. This takes
2-5 minutes on first launch. The `~/test-workspace/` directory
and mock OCI registry are only available **inside** the running
devcontainer, not on your host machine.

## GitHub Codespaces

1. Navigate to the PR you want to test on GitHub.
2. Click **Code** > **Codespaces** > **Create codespace on
   \<branch\>**.
3. Codespaces auto-detects `.devcontainer/devcontainer.json`
   and builds the environment.

**GITHUB_TOKEN**: Set via **Settings > Codespaces > Secrets**
in your GitHub user settings, or configure it at the repository
level under **Settings > Secrets and variables > Codespaces**.

**Note**: Codespace suspend/resume kills background processes.
The mock OCI registry will need to be restarted manually after
resuming -- see the Troubleshooting section below.

## DevPod

### First-time setup

Configure the provider to auto-stop containers after 30
minutes of inactivity. This only needs to be done once and
applies to all workspaces:

```bash
# For podman (default on Fedora/RHEL)
devpod provider set-options podman \
  -o INACTIVITY_TIMEOUT=30m

# For docker
devpod provider set-options docker \
  -o INACTIVITY_TIMEOUT=30m
```

To change the timeout later, re-run with a different value
(e.g., `1h`, `10m`).

### CLI

Create the workspace and connect:

```bash
# From a remote repository (e.g., testing a PR)
devpod up github.com/complytime/complyctl --ide none \
  && devpod ssh complyctl

# From a local branch (e.g., testing your own changes)
devpod up . --ide none && devpod ssh complyctl
```

### Testing a different branch or PR

If you already have a workspace and check out a different
branch (e.g., a contributor's PR), the source code inside
the container updates automatically because DevPod
bind-mounts your local directory.

The environment **auto-rebuilds** `complyctl` when it
detects the source has changed (different commit). Start
the workspace and connect -- the rebuild happens on login:

```bash
git checkout pr-branch
devpod up . --ide none && devpod ssh complyctl
```

To skip the auto-rebuild (e.g., for a docs-only change):

```bash
COMPLYCTL_SKIP_REBUILD=1 devpod ssh complyctl
```

To fully recreate the workspace from scratch:

```bash
devpod up . --ide none --recreate && devpod ssh complyctl
```

### Resuming a stopped container

If the container was stopped (by inactivity timeout or
`devpod stop`), use `devpod up` to restart it. Do **not**
use `devpod ssh` directly on a stopped workspace -- it may
fail to reconnect:

```bash
devpod up . --ide none && devpod ssh complyctl
```

When done, exit the SSH session (`exit`). The container
stops automatically after the inactivity timeout, or stop
it immediately:

```bash
devpod stop complyctl
```

### Desktop

Open DevPod Desktop and add a new workspace from the GitHub
URL `https://github.com/complytime/complyctl`.

**GITHUB_TOKEN**: Export the token before starting DevPod:

```bash
export GITHUB_TOKEN=<your-token>
devpod up github.com/complytime/complyctl --ide none
```

Alternatively, configure the environment variable through
DevPod's environment configuration settings.

## VS Code Dev Containers

1. Clone the repository locally.
2. Open the repository folder in VS Code.
3. When prompted, click **Reopen in Container**.

You can also open the Command Palette (`Ctrl+Shift+P` /
`Cmd+Shift+P`) and select **Dev Containers: Reopen in
Container**.

**GITHUB_TOKEN**: Either export the token before starting
VS Code:

```bash
export GITHUB_TOKEN=<your-token>
code /path/to/complyctl
```

Or set it in the integrated terminal after the container opens:

```bash
export GITHUB_TOKEN=<your-token>
```

## What the Environment Provides

### Binaries

| Binary | Location |
|--------|----------|
| `complyctl` | `./bin/` |
| `mock-oci-registry` | `./bin/` |
| `snappy` | `$GOPATH/bin` |
| `ampel` | `$GOPATH/bin` |
| `conftest` | `$GOPATH/bin` |
| `complyctl-provider-ampel` | `~/.complytime/providers/` |
| `complyctl-provider-openscap` | `~/.complytime/providers/` |
| `complyctl-provider-opa` | `~/.complytime/providers/` |

### Test Content

- A mock OCI registry running on `localhost:8765`, loaded with
  Gemara test catalogs and policies.
- A test workspace at `~/test-workspace/` with a
  `complytime.yaml` pre-configured to point at the mock
  registry.

### System Packages

- `openscap-scanner`
- `scap-security-guide`

**Note**: OpenSCAP has limited functionality in containers.
See [Troubleshooting > OpenSCAP limitations](#openscap-limitations)
for details and the recommended testing path.

## Command Reference

These are the primary commands for testing `complyctl` inside
the devcontainer:

### Ampel Provider (branch protection)

```bash
cd ~/test-workspace

# Fetch policies from the mock registry
complyctl get
# Expected: "Synchronization completed."

# Generate a policy bundle for the ampel provider
complyctl generate --policy-id test-ampel-bp
# Expected: "Generation completed."

# Run a scan (requires GITHUB_TOKEN)
GITHUB_TOKEN=<your-token> complyctl scan \
  --policy-id test-ampel-bp
# Expected: Scan results with requirement status
```

### OPA Provider (container security)

```bash
cd ~/test-workspace

# Fetch policies and complypacks from the mock registry
complyctl get
# Expected: "Synchronization completed."

# Generate for the OPA provider
complyctl generate --policy-id test-opa-k8s
# Expected: "Generation completed."

# Run a scan against the test deployment
complyctl scan --policy-id test-opa-k8s
# Expected: Scan results for container security requirements
```

Note: The OPA provider requires the `complyctl-provider-opa`
binary in `~/.complytime/providers/` (installed by the
post-create script from `complytime-providers`).

For OPA complypacks, `complytime-mapping.json` entries use the
Gemara assessment plan `id` in `requirement_id`. This is the ID
sent to providers during generation. It is different from the
assessment plan's `requirement-id`, which complyctl resolves later
when writing scan results.

## Private Bundles

The devcontainer can serve private Gemara policies through
the mock OCI registry without pushing them to an external
registry or committing them to the repository. Mounted
policies are served alongside the built-in test content,
so the standard `complyctl get` -> `generate` -> `scan`
workflow works for all policies.

### Setup

Place raw Gemara YAML files in a directory and mount it
into the devcontainer at `/bundles/` (or set
`COMPLYCTL_BUNDLES_DIR` to a custom path):

```
/bundles/
└── my-private-policy/
    ├── catalog.yaml
    └── policy.yaml
```

Each subdirectory under `/bundles/` containing both
`catalog.yaml` and `policy.yaml` is automatically
discovered and served by the mock registry during
container setup.

### Mounting bundles with DevPod

Use the `--workspace-env` flag to set the bundles path, and
configure a volume mount in `devcontainer.json`:

```bash
devpod up github.com/complytime/complyctl \
    --ide none \
    --dotfiles none
```

Or add a `mounts` entry to `.devcontainer/devcontainer.json`:

```json
"mounts": [
    "source=/path/to/local/bundles,target=/bundles,type=bind,readonly"
]
```

### Using bundles

After the devcontainer starts, mounted policies are served
by the mock registry. Use the standard workflow:

```bash
cd ~/test-workspace

# Fetch policies from the mock registry (including mounted ones)
complyctl get

# Generate and scan as usual
complyctl generate --policy-id my-private-policy
complyctl scan --policy-id my-private-policy
```

### How it works

The mock OCI registry's `seedFromDirectory()` reads Gemara
catalog and policy YAML files from the mounted bundles
directory and serves them as OCI artifacts, exactly like
the embedded test content. The post-create script adds
policy entries to `complytime.yaml` pointing at the mock
registry (`http://localhost:8765/policies/{name}`), so `complyctl
get` populates the cache through normal code paths.

## Troubleshooting

### Mock registry not running

If `complyctl get` fails to connect, the mock registry may not
be running. Start it manually and verify:

```bash
./bin/mock-oci-registry &
curl -sf http://localhost:8765/v2/
```

A successful response confirms the registry is available.

### GITHUB_TOKEN not set

`complyctl scan` will fail if `GITHUB_TOKEN` is not set.
Export it in your shell:

```bash
export GITHUB_TOKEN=<your-token>
```

### OpenSCAP limitations

OpenSCAP system scans are limited inside containers due to
missing host-level access. Use the Ampel provider with the mock
registry for CLI testing. Full OpenSCAP testing is available via
[complytime-demos](https://github.com/complytime/complytime-demos).

### File ownership changed after using DevPod (podman)

When using DevPod with podman rootless, the container's user
namespace remaps your host UID to a different UID inside the
container. This can change file ownership on the host after
the workspace stops, causing git to refuse operations:

```
fatal: detected dubious ownership in repository at '/path/to/complyctl'
```

Fix by restoring ownership with `podman unshare`:

```bash
podman unshare chown -R 0:0 /path/to/complyctl
```

This is inherent to podman rootless user namespace mapping
and does not affect files inside the running devcontainer.

### After Codespace resume

Background processes are killed when a Codespace is suspended.
After resuming, restart the mock registry manually:

```bash
./bin/mock-oci-registry &
```

## See Also

- [Quick Start](./QUICK_START.md)
- [E2E Testing](../tests/e2e/README.md)
- [Cross-Repo Integration Tests](../tests/cross-repo/)
- [complytime-demos](https://github.com/complytime/complytime-demos)
  -- full OpenSCAP testing in a Fedora VM
