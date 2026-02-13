# Quickstart: AMPEL Branch Protection Scanning

## Prerequisites

1. complyctl installed and configured
2. AMPEL tools installed and on PATH:
   - `ampel` - policy verification engine
   - `snappy` - branch protection data collector

3. Access to target GitHub/GitLab repositories

## Setup

### 1. Install the plugin

Copy the `ampel-plugin` binary and manifest to the complyctl
plugins directory:

```bash
cp ampel-plugin ~/.local/share/complytime/plugins/
cp c2p-ampel-manifest.json ~/.local/share/complytime/plugins/
```

### 2. Create an assessment plan

```bash
complyctl plan <framework-id>
```

This creates `assessment-plan.json` in your workspace with
branch protection controls.

### 3. Configure target repositories

Create `ampel-targets.yaml` in your workspace under
`ampel/ampel-targets.yaml`:

```yaml
repositories:
  - url: https://github.com/myorg/myrepo
    branches:
      - main
  - url: https://gitlab.com/myorg/another-repo
    branches:
      - main
      - develop
```

### 4. Generate AMPEL policies

```bash
complyctl generate
```

This translates the OSCAL assessment plan into AMPEL policy
files at `{workspace}/ampel/policy/`.

### 5. Scan repositories

```bash
complyctl scan
```

This scans each configured repository for branch protection
compliance and produces:
- Per-repository result files in `{workspace}/ampel/results/`
- Consolidated `assessment-results.json` in the workspace

### 6. View results

```bash
complyctl scan --format markdown
```

## Workspace Structure After Scan

```text
~/.local/share/complytime/
├── assessment-plan.json
├── assessment-results.json
└── ampel/
    ├── ampel-targets.yaml
    ├── policy/
    │   └── branch-protection-policy.json
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
