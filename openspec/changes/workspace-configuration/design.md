## Context

Currently, `NewWorkspace()` in `internal/complytime/workspace.go` hardcodes `WorkspaceConfigFile` (which resolves to `"complytime.yaml"`) relative to the current working directory. All workspace-relative paths use hardcoded `"."` or `".complytime"` references. The log file writer in `cmd/complyctl/cli/root.go` uses `complytime.WorkspaceDir` without workspace resolution.

This means users must `cd` into the project directory before running any complyctl command. Additionally, the config file at the repository root contributes to visual clutter alongside `.git/`, `.github/`, `.gitignore`, `README.md`, and other metadata files.

Issue #433 requests a `--workspace` flag and `COMPLYTIME_WORKSPACE` env var to allow running commands from any directory. Issue #527 requests moving config files to `.complytime/` to reduce clutter. This change addresses both issues simultaneously.

## Goals / Non-Goals

**Goals:**
- Add `--workspace` / `-w` flag to all commands for specifying workspace directory
- Add `COMPLYTIME_WORKSPACE` environment variable with precedence: flag > env > cwd
- Move `complytime.yaml` to `.complytime/complytime.yaml` with backward compatibility
- Update all workspace-relative path construction to use resolved workspace
- Provide clear error messages for invalid workspace paths
- Provide deprecation warning with migration instructions for legacy config location

**Non-Goals:**
- Changing the workspace config file format or validation logic
- Adding a migration command (users can manually move the file)
- Supporting multiple workspace roots or workspace discovery via upward directory traversal
- Locking or coordination for concurrent workspace access (not a supported use case)

## Decisions

### D1: Workspace flag points to repository root, not `.complytime/`

The `--workspace` flag points to the project root (parent of `.complytime/`), not the `.complytime/` directory itself.

Rationale: This matches how git works (`--git-dir` points to the repo root, not `.git/`) and makes the flag more intuitive. Users think in terms of "my project directory," not "my compliance metadata directory." The `.complytime/` subdirectory is an implementation detail.

Example:
```bash
complyctl scan --workspace ~/projects/myapp
# Looks for ~/projects/myapp/.complytime/complytime.yaml
```

### D2: Precedence order is flag > env var > current directory

Workspace directory resolution follows standard precedence:
1. `--workspace` flag value (if non-empty)
2. `COMPLYTIME_WORKSPACE` environment variable (if set and non-empty)
3. Current working directory (`"."`)

Rationale: This matches common tool patterns (e.g., `DOCKER_HOST`, `KUBECONFIG`). Flags override environment variables override defaults. Clear, predictable precedence hierarchy.

### D3: Support both config locations with new location preferred

When loading config, check locations in order:
1. `.complytime/complytime.yaml` (new location)
2. `complytime.yaml` (legacy location, with deprecation warning)
3. Error if neither exists

When both locations exist, use `.complytime/complytime.yaml` without warning.

Rationale: Smooth migration path. Users can adopt the new location at their own pace. When they do migrate, the warning stops immediately. No hard cutover that breaks existing workflows.

### D4: Deprecation warning is informative, not disruptive

When legacy location is detected, print to stderr:
```text
WARNING: complytime.yaml found at repository root (legacy location).
Please move it to .complytime/complytime.yaml for better organization.
Run: mkdir -p .complytime && mv complytime.yaml .complytime/complytime.yaml
```

The warning is printed once per command invocation, before any other output. It does not block execution or change behavior.

Rationale: Warnings guide users toward best practices without forcing immediate action. Including the exact migration command reduces friction. Printing to stderr keeps stdout clean for pipelines.

### D5: Path expansion and validation happen early

The workspace resolution function:
1. Expands `~/` prefix using existing `ExpandPath()` helper
2. Resolves relative paths to absolute using `filepath.Abs()`
3. Validates that the path exists and is a directory
4. Returns absolute path or error

This happens in `opts.ResolveWorkspace()` called early in each command's `RunE` function, before any workspace operations.

Rationale: Fail fast with clear errors. Absolute paths simplify debugging (no ambiguity about what directory is being used). Validation prevents confusing downstream errors when paths are invalid.

### D6: `NewWorkspace()` signature changes to accept base directory

Current signature:
```go
func NewWorkspace() *Workspace
```

New signature:
```go
func NewWorkspace(baseDir string) *Workspace
```

The constructor uses `DetectConfigPath(baseDir)` to find the config file (new or legacy location). The `Workspace` struct stores both `baseDir` (for path construction) and `configPath` (for load/save operations).

Rationale: Dependency injection makes testing straightforward. The baseDir parameter is explicit — no hidden coupling to current working directory. Struct stores both values for later reference.

### D7: Log file writer initialization deferred until first write

The `lazyLogWriter` already defers file creation until the first write. Modify it to accept a workspace directory value set in `PersistentPreRun` after workspace resolution.

Current flow:
- `init()` creates `lazyLogWriter{}`
- First write creates log file at `".complytime/complyctl.log"`

New flow:
- `init()` creates `lazyLogWriter{}`
- `PersistentPreRun` sets workspace directory on the writer
- First write creates log file at `"<baseDir>/.complytime/complyctl.log"`

Rationale: Avoids moving logger initialization out of `init()`. Minimal disruption to existing logger setup. The lazy writer pattern already handles deferred initialization.

### D8: Constants for all path components

Use existing constants from `internal/complytime/consts.go`:
- `WorkspaceDir = ".complytime"`
- `WorkspaceConfigFile = "complytime.yaml"`
- `LogFileName = "complyctl.log"`
- `ScanOutputDir = "scan"`
- `StateFileName = "state.json"`

Add new constant:
- `WorkspaceEnvVar = "COMPLYTIME_WORKSPACE"`

All path construction uses `filepath.Join()` with these constants. No hardcoded path strings.

Rationale: Single source of truth for path components. Easy to change in one place if needed. Consistent with existing codebase patterns.

## Implementation Details

### Workspace Resolution Flow

```go
// In cmd/complyctl/cli/options.go
func (o *Common) ResolveWorkspace() (string, error) {
    return complytime.ResolveWorkspaceDir(o.Workspace)
}

// In internal/complytime/workspace.go
func ResolveWorkspaceDir(flagValue string) (string, error) {
    var raw string
    if flagValue != "" {
        raw = flagValue
    } else if envValue := os.Getenv(WorkspaceEnvVar); envValue != "" {
        raw = envValue
    } else {
        raw = "."
    }
    
    expanded := ExpandPath(raw) // Handle ~/
    absPath, err := filepath.Abs(expanded)
    if err != nil {
        return "", fmt.Errorf("failed to resolve workspace path: %w", err)
    }
    
    stat, err := os.Stat(absPath)
    if err != nil {
        return "", fmt.Errorf("workspace directory does not exist: %s", absPath)
    }
    if !stat.IsDir() {
        return "", fmt.Errorf("workspace path is not a directory: %s", absPath)
    }
    
    return absPath, nil
}
```

### Config Detection Flow

```go
// In internal/complytime/workspace.go
func DetectConfigPath(baseDir string) (configPath string, isLegacy bool, err error) {
    // Check new location first
    newPath := filepath.Join(baseDir, WorkspaceDir, WorkspaceConfigFile)
    if _, err := os.Stat(newPath); err == nil {
        return newPath, false, nil
    }
    
    // Fall back to legacy location
    legacyPath := filepath.Join(baseDir, WorkspaceConfigFile)
    if _, err := os.Stat(legacyPath); err == nil {
        return legacyPath, true, nil
    }
    
    // Neither exists
    return "", false, fmt.Errorf("config file not found in %s (checked %s and %s)",
        baseDir, newPath, legacyPath)
}
```

### Workspace Constructor

```go
// In internal/complytime/workspace.go
type Workspace struct {
    baseDir    string  // NEW: workspace root directory
    configPath string
    config     *WorkspaceConfig
}

func NewWorkspace(baseDir string) *Workspace {
    configPath, isLegacy, err := DetectConfigPath(baseDir)
    if err != nil {
        // Detection error — let Load() handle it
        configPath = filepath.Join(baseDir, WorkspaceDir, WorkspaceConfigFile)
    }
    
    if isLegacy {
        printDeprecationWarning()
    }
    
    return &Workspace{
        baseDir:    baseDir,
        configPath: configPath,
    }
}

func (w *Workspace) BaseDir() string {
    return w.baseDir
}

func printDeprecationWarning() {
    fmt.Fprintf(os.Stderr, `WARNING: complytime.yaml found at repository root (legacy location).
Please move it to .complytime/complytime.yaml for better organization.
Run: mkdir -p .complytime && mv complytime.yaml .complytime/complytime.yaml

`)
}
```

### Command Integration Pattern

```go
// In cmd/complyctl/cli/scan.go (and other commands)
func (o *scanOptions) run(ctx context.Context) error {
    baseDir, err := o.ResolveWorkspace()
    if err != nil {
        return err
    }
    
    ws := complytime.NewWorkspace(baseDir)
    if err := ws.LoadAndValidate(); err != nil {
        return fmt.Errorf("failed to load workspace config: %w", err)
    }
    
    // Use baseDir for all workspace-relative paths
    outDir := filepath.Join(baseDir, complytime.WorkspaceDir, complytime.ScanOutputDir)
    // ...
}
```

### Log Writer Update

```go
// In cmd/complyctl/cli/root.go
type lazyLogWriter struct {
    once      sync.Once
    file      *os.File
    baseDir   string  // NEW: set in PersistentPreRun
}

func (w *lazyLogWriter) SetWorkspace(baseDir string) {
    w.baseDir = baseDir
}

func (w *lazyLogWriter) Write(p []byte) (int, error) {
    w.once.Do(func() {
        dir := filepath.Join(w.baseDir, complytime.WorkspaceDir)
        if err := os.MkdirAll(dir, 0750); err != nil {
            return
        }
        logPath := filepath.Join(dir, complytime.LogFileName)
        f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
        if err != nil {
            return
        }
        w.file = f
    })
    if w.file == nil {
        return len(p), nil
    }
    return w.file.Write(p)
}

// In New()
cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
    enableDebug(&opts)
    baseDir, _ := opts.ResolveWorkspace()
    if baseDir != "" {
        lw.SetWorkspace(baseDir)
    } else {
        lw.SetWorkspace(".")  // Fallback to cwd
    }
}
```

## Error Scenarios

| Scenario | Error Message | Exit Code |
| ---------- | -------------- | ----------- |
| `--workspace /nonexistent` | `workspace directory does not exist: /nonexistent` | 1 |
| `--workspace /etc/passwd` (file) | `workspace path is not a directory: /etc/passwd` | 1 |
| Neither config location exists | `config file not found in /path (checked .complytime/complytime.yaml and complytime.yaml)` | 1 |
| Invalid env var path | `failed to resolve workspace path: <error>` | 1 |

## Testing Strategy

### Unit Tests (`internal/complytime/workspace_test.go`)

**Workspace Resolution:**
- Flag precedence over env var
- Env var fallback when flag empty
- Default to cwd when neither set
- Tilde expansion (`~/project`)
- Relative to absolute (`./foo`)
- Error for nonexistent path
- Error for file (not directory)

**Config Detection:**
- Finds `.complytime/complytime.yaml`
- Falls back to root `complytime.yaml`
- New location wins when both exist
- Error when neither exists
- `isLegacy` flag correct

**Workspace Constructor:**
- Accepts baseDir parameter
- `BaseDir()` returns correct value
- `Path()` returns detected config path
- Deprecation warning printed for legacy

All tests use `t.TempDir()` for isolation. Tests create fixtures with config files in different locations.

### Integration Tests (`tests/integration_test.sh`)

- `--workspace` flag with scan command
- `COMPLYTIME_WORKSPACE` env var
- Flag overrides env var
- Relative workspace path works
- Tilde expansion works
- Legacy config warning appears
- New location used when both exist
- Scan output written to correct directory
- Log file written to correct directory
- Error for invalid workspace path

## Migration Path (for users)

### New Projects
Run `complyctl init` — it will create `.complytime/complytime.yaml` automatically.

### Existing Projects
Option 1: Continue using root location (not recommended)
- No action needed
- Deprecation warning shown on every command

Option 2: Migrate to new location (recommended)
```bash
mkdir -p .complytime
mv complytime.yaml .complytime/complytime.yaml
git add .complytime/complytime.yaml
git rm complytime.yaml
git commit -m "chore: migrate complytime.yaml to .complytime/ directory"
```

### CI/CD Workflows
Set workspace via environment variable:
```bash
export COMPLYTIME_WORKSPACE=/workspace/project
complyctl scan
```

Or use flag:
```bash
complyctl scan --workspace /workspace/project
```

## Risks / Trade-offs

- **[Signature change]** `NewWorkspace()` signature changes from no parameters to one parameter. All call sites must be updated. → Mitigation: Compiler enforces this. Easy to find and fix.
- **[Legacy support complexity]** Supporting both config locations adds detection logic. → Acceptable: The logic is localized to `DetectConfigPath()` and well-tested. Clear migration path reduces long-term support burden.
- **[Deprecation timeline unclear]** No hard removal date for legacy location. → Acceptable: Wait for user adoption data before forcing migration. No urgency to remove.
- **[Logger initialization complexity]** Log writer needs workspace directory but logger initializes early. → Mitigation: Lazy file creation pattern already exists; add workspace setter called in `PersistentPreRun`.
- **[Environment variable not in config]** `COMPLYTIME_WORKSPACE` is env-only, not settable in `complytime.yaml`. → Intentional: Per-invocation toggle, not workspace-level setting. Consistent with how CI/CD controls feature gates.
