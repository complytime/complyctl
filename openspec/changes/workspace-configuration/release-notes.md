## Configurable Workspace Directory

complyctl now supports running commands from any directory using the `--workspace` flag or `COMPLYTIME_WORKSPACE` environment variable. Additionally, the configuration file has moved from the repository root to `.complytime/complytime.yaml` for better organization.

### What's New

**Workspace Flag and Environment Variable**
- Run complyctl commands from any directory:
  ```bash
  complyctl scan --workspace ~/projects/myapp
  complyctl list --workspace ../other-project
  ```
- Set workspace via environment variable:
  ```bash
  export COMPLYTIME_WORKSPACE=/workspace/project
  complyctl scan
  ```
- Precedence: `--workspace` flag > `COMPLYTIME_WORKSPACE` env var > current directory

**Config File Location**
- Configuration file moves from repository root to `.complytime/complytime.yaml`
- Reduces repository clutter by consolidating all complyctl artifacts in `.complytime/`
- Backward compatible: complyctl checks `.complytime/complytime.yaml` first, falls back to root `complytime.yaml` with a deprecation warning

### Migration

**For New Projects**
No action needed. `complyctl init` creates `.complytime/complytime.yaml` automatically.

**For Existing Projects**
Move your config file to the new location:
```bash
mkdir -p .complytime
mv complytime.yaml .complytime/complytime.yaml
git add .complytime/complytime.yaml
git rm complytime.yaml
git commit -m "chore: migrate complytime.yaml to .complytime/ directory"
```

Until you migrate, commands will continue to work but will show a deprecation warning:
```text
WARNING: complytime.yaml found at repository root (legacy location).
Please move it to .complytime/complytime.yaml for better organization.
Run: mkdir -p .complytime && mv complytime.yaml .complytime/complytime.yaml
```

**For CI/CD Workflows**
Set the workspace directory to avoid `cd` commands:
```bash
# GitHub Actions
- name: Run compliance scan
  env:
    COMPLYTIME_WORKSPACE: ${{ github.workspace }}
  run: complyctl scan

# GitLab CI
script:
  - export COMPLYTIME_WORKSPACE=${CI_PROJECT_DIR}
  - complyctl scan

# Jenkins
sh 'COMPLYTIME_WORKSPACE=${WORKSPACE} complyctl scan'
```

### Breaking Changes

None. The change is fully backward compatible. Existing workflows continue to work without modification.
