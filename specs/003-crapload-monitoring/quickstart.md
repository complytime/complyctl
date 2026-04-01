# Quickstart: CRAP Load Monitoring

## Prerequisites

- Go 1.24+ (matches project requirement)
- `jq` (for baseline comparison in `make crapload-check`)
- `bc` (for numeric comparison in `make crapload-check`)
- Access to the repository

## Local Setup

### 1. Install Gaze

```bash
go install github.com/unbound-force/gaze/cmd/gaze@latest
```

Verify installation:

```bash
gaze --version
```

### 2. Run CRAP Analysis Locally

Single command via Makefile:

```bash
make crapload
```

This will:
1. Install Gaze if not already present
2. Run unit tests with coverage profiling (`coverage.out`)
3. Run `gaze crap` using the coverage profile against all packages
4. Display a human-readable summary of CRAP and GazeCRAP scores

### 3. Generate a Baseline

To capture the current codebase scores as the quality floor:

```bash
make crapload-baseline
```

This generates `.gaze/baseline.json` with per-function scores.

### 4. Compare Against Baseline

To check if your local changes would pass the CI gate:

```bash
make crapload-check
```

This compares current scores against the committed baseline and reports regressions.

## Interpreting Results

| Score Range | Risk Level | Action |
|-------------|------------|--------|
| 0-14        | Safe       | No action needed |
| 15-29       | Warning    | Consider simplifying or adding tests |
| 30+         | Dangerous  | Must reduce complexity or improve coverage |

## Common Workflows

### Before opening a PR

```bash
make crapload-check
```

If the check passes, your PR should pass the CI CRAP gate.

### Investigating a CI failure

1. Read the PR comment to identify which functions regressed
2. Run `gaze crap --format=text ./path/to/package` locally to see detailed scores
3. Either reduce complexity (refactor) or improve test coverage for the flagged functions
4. Re-run `make crapload-check` to verify the fix

### Updating the baseline after improvements

After merging PRs that improve CRAP scores, a maintainer can regenerate the baseline:

```bash
make crapload-baseline
```

Commit the updated `.gaze/baseline.json` in a separate PR.
