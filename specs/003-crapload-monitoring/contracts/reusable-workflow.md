# Contract: Reusable CRAP Load Analysis Workflow

## Interface Type

GitHub Actions reusable workflow (`workflow_call`)

## Caller Contract

### Inputs

| Input | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `go-version-file` | string | no | `'./go.mod'` | Path to go.mod for Go version detection |
| `gaze-version` | string | no | `'latest'` | Gaze version tag to install via `go install` |
| `baseline-file` | string | no | `'.gaze/baseline.json'` | Path to committed baseline thresholds file |
| `packages` | string | no | `'./...'` | Go packages to analyse |
| `coverprofile` | string | no | `'coverage.out'` | Path to coverage profile |
| `new-function-threshold` | number | no | `30` | CRAP score ceiling for new functions with no baseline entry |

> **Note:** Per-function CRAP/GazeCRAP thresholds (both default to 15) are
> managed by gaze internally. The workflow uses `gaze report --format=json`
> which runs CRAP, Quality, Classification, and Docscan analysis in a single
> invocation.

### Required Permissions

| Permission | Level | Purpose |
|------------|-------|---------|
| `contents` | `read` | Checkout code and read baseline file |

The reusable workflow requires only `contents: read`. PR comment posting is the caller's responsibility, keeping the reusable workflow minimal and avoiding the need to pass `pull-requests: write` tokens across repository boundaries.

### Outputs

| Output | Type | Description |
|--------|------|-------------|
| `status` | string | `pass` or `fail` |
| `crapload-count` | number | Number of functions at or above CRAP threshold |
| `gaze-crapload-count` | number | Number of functions at or above GazeCRAP threshold |
| `regressions-count` | number | Number of functions that regressed vs baseline |
| `improvements-count` | number | Number of functions that improved vs baseline |

### Artifacts

| Artifact | Contents | Always uploaded |
|----------|----------|-----------------|
| `crapload-analysis` | `/tmp/crapload-comment-body.md` (markdown comment body) | Yes |
| `crapload-analysis-detailed` | `/tmp/gaze-report.json`, `/tmp/crapload-current.json` | Only when analysis runs (skipped if no Go changes) |

## Behaviour Contract

### Preconditions

- Calling repository MUST be a Go project with a valid `go.mod`
- Go test suite MUST be runnable via `go test ./...`

### Postconditions

- The markdown comment body is uploaded as the `crapload-analysis` artifact
- Exit code is 0 if all thresholds are met, non-zero otherwise
- Coverage profile is generated if not already present

### Invariants

- The workflow MUST NOT modify the repository contents (read-only analysis)
- The workflow MUST NOT push commits or create branches
- The workflow MUST NOT post PR comments (caller's responsibility)

## Usage Example

```yaml
jobs:
  crapload:
    uses: complytime/complyctl/.github/workflows/reusable_crapload_analysis.yml@main
    permissions:
      contents: read
    with:
      new-function-threshold: 25

  post-comment:
    needs: crapload
    runs-on: ubuntu-latest
    permissions:
      pull-requests: write
    steps:
      - uses: actions/download-artifact@v4
        with:
          name: crapload-analysis
          path: artifact
      - uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const body = fs.readFileSync('artifact/tmp/crapload-comment-body.md', 'utf8');
            // Post or update PR comment...
```
