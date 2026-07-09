# Implementation Tasks

## Phase 1: Core Workspace Resolution

### Task 1.1: Add workspace resolution function
- [ ] Add `WorkspaceEnvVar` constant to `internal/complytime/consts.go`
- [ ] Implement `ResolveWorkspaceDir(flagValue string) (string, error)` in `internal/complytime/workspace.go`
  - [ ] Check precedence: flag > env var > cwd
  - [ ] Expand `~/` prefix using existing `ExpandPath()`
  - [ ] Convert to absolute path via `filepath.Abs()`
  - [ ] Validate path exists and is directory
  - [ ] Return error with clear message for invalid paths

### Task 1.2: Add config detection function
- [ ] Implement `DetectConfigPath(baseDir string) (string, bool, error)` in `internal/complytime/workspace.go`
  - [ ] Check `.complytime/complytime.yaml` first
  - [ ] Fall back to `complytime.yaml` at root
  - [ ] Return `isLegacy=true` for root location
  - [ ] Return error if neither exists

### Task 1.3: Add deprecation warning
- [ ] Implement `printDeprecationWarning()` helper
  - [ ] Print to stderr
  - [ ] Include migration command in warning text
  - [ ] Call from `NewWorkspace()` when legacy location detected

### Task 1.4: Update Workspace struct and constructor
- [ ] Add `baseDir string` field to `Workspace` struct
- [ ] Change `NewWorkspace()` signature to `NewWorkspace(baseDir string) *Workspace`
- [ ] Update constructor to call `DetectConfigPath(baseDir)`
- [ ] Store both `baseDir` and `configPath` in struct
- [ ] Add `BaseDir() string` method to expose workspace root

## Phase 2: CLI Integration

### Task 2.1: Update Common struct and flag binding
- [ ] Add `Workspace string` field to `Common` struct in `cmd/complyctl/cli/options.go`
- [ ] Update `BindFlags()` to register `--workspace` / `-w` flag
- [ ] Add `ResolveWorkspace() (string, error)` method to `Common`

### Task 2.2: Update command files to use resolved workspace
- [ ] Update `init.go`: resolve workspace and pass to `NewWorkspace(baseDir)`
- [ ] Update `list.go`: resolve workspace and pass to `NewWorkspace(baseDir)`
- [ ] Update `scan.go`: resolve workspace and pass to `NewWorkspace(baseDir)`
- [ ] Update `generate.go`: resolve workspace and pass to `NewWorkspace(baseDir)` (if applicable)
- [ ] Update any other commands using `NewWorkspace()`

### Task 2.3: Update scan output path construction
- [ ] Update `processScanOutput()` in `scan.go` to use baseDir parameter
- [ ] Change `outDir := filepath.Join(".", complytime.WorkspaceDir, complytime.ScanOutputDir)`
  to `outDir := filepath.Join(baseDir, complytime.WorkspaceDir, complytime.ScanOutputDir)`
- [ ] Verify `writeScanReports()` receives correct output directory

## Phase 3: Log Writer Update

### Task 3.1: Modify lazyLogWriter
- [ ] Add `baseDir string` field to `lazyLogWriter` struct in `root.go`
- [ ] Add `SetWorkspace(baseDir string)` method
- [ ] Update `Write()` to use `w.baseDir` for log path construction
- [ ] Change `logDir := complytime.WorkspaceDir` to `logDir := filepath.Join(w.baseDir, complytime.WorkspaceDir)`

### Task 3.2: Set workspace in PersistentPreRun
- [ ] Update `PersistentPreRun` in `New()` to resolve workspace
- [ ] Call `lw.SetWorkspace(baseDir)` after resolution
- [ ] Handle resolution error gracefully (fall back to ".")

## Phase 4: Testing

### Task 4.1: Unit tests for workspace resolution
- [ ] `TestResolveWorkspaceDir_FlagPrecedence`
- [ ] `TestResolveWorkspaceDir_EnvVarFallback`
- [ ] `TestResolveWorkspaceDir_DefaultToCwd`
- [ ] `TestResolveWorkspaceDir_TildeExpansion`
- [ ] `TestResolveWorkspaceDir_RelativeToAbsolute`
- [ ] `TestResolveWorkspaceDir_InvalidPath`
- [ ] `TestResolveWorkspaceDir_NotDirectory`

### Task 4.2: Unit tests for config detection
- [ ] `TestDetectConfigPath_NewLocation`
- [ ] `TestDetectConfigPath_LegacyFallback`
- [ ] `TestDetectConfigPath_BothExist`
- [ ] `TestDetectConfigPath_NeitherExists`

### Task 4.3: Unit tests for Workspace constructor
- [ ] `TestNewWorkspace_WithBaseDir`
- [ ] `TestNewWorkspace_BaseDir`
- [ ] `TestNewWorkspace_ConfigPath`
- [ ] `TestNewWorkspace_DeprecationWarning` (capture stderr)

### Task 4.4: Integration tests
- [ ] Add test for `--workspace` flag with scan command
- [ ] Add test for `COMPLYTIME_WORKSPACE` env var
- [ ] Add test for flag overriding env var
- [ ] Add test for relative workspace path
- [ ] Add test for tilde expansion
- [ ] Add test for legacy config deprecation warning
- [ ] Add test for new location preferred when both exist
- [ ] Add test for scan output in correct directory
- [ ] Add test for log file in correct directory
- [ ] Add test for error on invalid workspace path

## Phase 5: Documentation

### Task 5.1: Update user documentation
- [ ] Update README.md with `--workspace` flag examples
- [ ] Add migration guide section to README.md
- [ ] Update CHANGELOG.md with feature description and migration instructions

### Task 5.2: Update project documentation
- [ ] Update AGENTS.md "Recent Changes" section
- [ ] Add entry for workspace-configuration OpenSpec

### Task 5.3: Update command help text
- [ ] Verify `--workspace` flag appears in `complyctl --help`
- [ ] Add workspace env var mention to `scan` command help text
- [ ] Update examples in command help text if needed

## Phase 6: Final Verification

### Task 6.1: Manual testing
- [ ] Test `complyctl init` creates `.complytime/complytime.yaml`
- [ ] Test `complyctl scan --workspace /path` from different directory
- [ ] Test `COMPLYTIME_WORKSPACE=/path complyctl scan`
- [ ] Test legacy config shows deprecation warning
- [ ] Test both locations exist (new location used, no warning)
- [ ] Test invalid workspace path shows error

### Task 6.2: CI verification
- [ ] Run full test suite: `make test-unit`
- [ ] Run integration tests: `make test-integration`
- [ ] Run E2E tests: `make test-e2e`
- [ ] Run linter: `make lint`
- [ ] Verify CRAP scores: `make crapload-check`

## Notes

- All tests must use `t.TempDir()` for filesystem isolation
- All error messages must be clear and actionable
- All path construction must use constants from `consts.go`
- All code must follow Go conventions from `go.md` convention pack
- Tasks marked `[P]` can be executed in parallel if needed
