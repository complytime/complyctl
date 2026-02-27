# Quickstart: AMPEL Branch Protection Scanning

## Prerequisites

1. complyctl installed and configured
2. AMPEL tools installed via `go install` and on PATH:
   ```bash
   go install github.com/carabiner-dev/snappy@latest
   go install github.com/carabiner-dev/ampel/cmd/ampel@latest
   export PATH=$PATH:$HOME/go/bin
   ```
3. A GitHub personal access token with read access to target
   repositories:
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

## Setup

### 1. Install the plugin

Copy the `ampel-plugin` binary and manifest to the complyctl
plugins directory:

```bash
cp ampel-plugin ~/.local/share/complytime/plugins/
cp c2p-ampel-manifest.json ~/.local/share/complytime/plugins/
```

### 2. Prepare granular AMPEL policies

Place granular AMPEL policy files (one JSON file per control)
in the policy directory. Sample policies are available in the
[complytime-demos](https://github.com/complytime/complytime-demos)
repository under `base_ansible_env/files/ampel-policies/`.

```bash
cp ampel-policies/*.json ~/.local/share/complytime/ampel-policies/
```

### 3. Create an assessment plan

```bash
complyctl plan <framework-id>
```

This creates `assessment-plan.json` in your workspace with
branch protection controls.

### 4. Configure target repositories

Create `ampel-targets.yaml` in your workspace under
`ampel/ampel-targets.yaml`:

```yaml
repositories:
  - url: https://github.com/myorg/myrepo
    branches:
      - main
    specs:
      - builtin:github/branch-rules.yaml
  - url: https://github.com/myorg/another-repo
    branches:
      - main
      - develop
    specs:
      - builtin:github/branch-rules.yaml
```

Each repository requires a `specs` list. Use `builtin:` prefix
for embedded spec files or absolute paths for custom specs.

### 5. Generate AMPEL policies

```bash
complyctl generate
```

This matches OSCAL assessment plan rules against the granular
AMPEL policies and merges the matching policies into a combined
bundle at `{workspace}/ampel/policy/complytime-ampel-policy.json`.

### 6. Scan repositories

```bash
complyctl scan
```

This scans each configured repository for branch protection
compliance and produces:
- Per-repository result files in `{workspace}/ampel/results/`
- Consolidated `assessment-results.json` in the workspace

## Workspace Structure After Scan

```text
~/.local/share/complytime/
├── assessment-plan.json
├── assessment-results.json
├── ampel-policies/
│   ├── SC-CODE-01.01-require-pull-request.json
│   ├── SC-CODE-02.01-minimum-approvals.json
│   └── ...
└── ampel/
    ├── ampel-targets.yaml
    ├── policy/
    │   └── complytime-ampel-policy.json
    └── results/
        ├── myorg-myrepo-main.json
        └── myorg-another-repo-main.json
```

## Custom Policy Location

To use an existing AMPEL policy directory, configure the plugin
manifest override:

```bash
mkdir -p /etc/complytime/config.d/
cat > /etc/complytime/config.d/c2p-ampel-manifest.json << 'EOF'
{
  "configuration": [
    {
      "name": "policy_dir",
      "default": "/path/to/my/ampel/policies"
    }
  ]
}
EOF
```

## Verify Tool Installation

If the plugin reports missing tools, verify they are on PATH:

```bash
which ampel snappy
```
