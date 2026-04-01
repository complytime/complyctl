# Data Model: CRAP Load Monitoring

## Entities

### Baseline Configuration

The baseline is a snapshot of CRAP and GazeCRAP scores for all functions in the codebase at a specific point in time. It serves as the quality floor for PR enforcement. Stored as the direct output of `gaze crap --format=json` with paths normalised to repository-relative.

**Attributes**:
- `scores[]` — list of Function Score records (per-function entries)
- `summary` — aggregate statistics (total functions, averages, CRAPload counts, worst offenders)

### Function Score

A single function's CRAP metrics at baseline time.

**Attributes**:
- Function name
- File path and line number
- Package
- Cyclomatic complexity
- Line coverage percentage
- Contract coverage percentage (if available)
- CRAP score (computed)
- GazeCRAP score (computed, if contract coverage available)

### PR Analysis Result

The output of running CRAP analysis on a PR's changes, compared against the baseline.

**Attributes**:
- PR number
- Commit SHA
- Analysis timestamp
- List of function results (Function Score records)
- List of regressions (functions exceeding baseline)
- List of improvements (functions with lower scores than baseline)
- List of new functions (no baseline entry)
- Overall status (pass/fail)

### Metrics Entry

A timestamped record of codebase-wide CRAP health, appended to `metrics.json` on the `gh-pages` branch whenever Go code is merged to main.

**Attributes**:
- Timestamp (ISO 8601)
- Commit SHA
- Git ref (branch name)
- Total functions analysed
- Average complexity
- Average line coverage
- Average contract coverage
- Average CRAP score
- Average GazeCRAP score
- CRAPload count (functions at or above threshold)
- GazeCRAPload count

## Relationships

```text
Baseline Configuration
  └── contains many → Function Score records

PR Analysis Result
  ├── compares against → Baseline Configuration
  └── contains many → Function Score records (with regression/improvement flags)

Metrics Entry
  └── aggregates → Function Score records (full codebase snapshot)
```

## Storage

- **Baseline**: Committed JSON file at `.gaze/baseline.json` in the repository root.
- **PR Analysis Result**: Transient — exists only as CI output (PR comment + workflow logs). Not persisted beyond CI artifact retention.
- **Metrics Entry**: Appended to `metrics.json` on the `gh-pages` branch. Persists indefinitely as git history.
