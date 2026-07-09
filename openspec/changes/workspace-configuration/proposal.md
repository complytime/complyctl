## Why

Currently, complyctl commands require users to run from the directory containing `complytime.yaml`. This creates workflow friction — users must `cd` into the project directory before every command. Additionally, the repository root contains `complytime.yaml` as a visible file, contributing to repository clutter alongside other dot-files and configuration.

Moving to a configurable workspace directory (via `--workspace` flag and `COMPLYTIME_WORKSPACE` env var) removes the `cd` requirement. Simultaneously migrating `complytime.yaml` into `.complytime/complytime.yaml` consolidates all complyctl artifacts into a hidden directory, reducing visual noise in repositories and making the tool more polite to the repository structure.

This aligns with how other tools work (git uses `.git/`, docker uses `.docker/`, etc.) and improves the developer experience for CI/CD pipelines where the workspace location may not be the current working directory.

## What Changes

- **Add** `--workspace` / `-w` flag to all commands via the `Common` struct
- **Add** `COMPLYTIME_WORKSPACE` environment variable with precedence: flag > env > cwd
- **Move** `complytime.yaml` config file from repository root to `.complytime/complytime.yaml`
- **Support** backward compatibility: check `.complytime/complytime.yaml` first, fall back to root `complytime.yaml` with deprecation warning
- **Update** `NewWorkspace()` to accept a `baseDir` parameter
- **Update** all workspace-relative path construction to use resolved workspace directory
- **Update** log file writer to use resolved workspace for `.complytime/complyctl.log`
- **Update** scan output directory to use resolved workspace for `.complytime/scan/`

## Capabilities

### New Capabilities
- `workspace-flag`: `--workspace` / `-w` flag on all commands to specify workspace directory
- `workspace-envvar`: `COMPLYTIME_WORKSPACE` environment variable for workspace directory resolution
- `config-subdir`: Config file location at `.complytime/complytime.yaml` instead of repository root

### Modified Capabilities
- `workspace-loading`: `NewWorkspace()` now accepts a base directory parameter instead of hardcoding current directory
- `log-file-location`: Log file resolves to `<workspace>/.complytime/complyctl.log` instead of `./.complytime/complyctl.log`
- `scan-output-location`: Scan output resolves to `<workspace>/.complytime/scan/` instead of `./.complytime/scan/`

### Backward Compatible Capabilities
- `config-legacy-fallback`: Commands check `.complytime/complytime.yaml` first, fall back to root `complytime.yaml` with deprecation warning
- `config-migration-path`: Deprecation warning includes migration instructions (`mkdir -p .complytime && mv complytime.yaml .complytime/`)

## Constitution Alignment

| Principle | Status | Evidence |
| ----------- | -------- | ---------- |
| I. Autonomous Collaboration | PASS | Workspace resolution happens early in command lifecycle; all subsystems receive resolved paths via dependency injection |
| II. Composability First | PASS | Workspace resolution is a separate concern from config loading; components receive paths as parameters |
| III. Observable Quality | PASS | Error messages clearly state which workspace directory failed validation; deprecation warnings guide users to new config location |
| IV. Testability | PASS | Workspace resolution is pure function (`ResolveWorkspaceDir(flagValue) -> (absPath, error)`); config detection is testable with `t.TempDir()` fixtures |

## Impact

- **CLI surface**: All commands gain `--workspace` / `-w` flag. Default behavior unchanged (defaults to current directory).
- **Environment**: New `COMPLYTIME_WORKSPACE` environment variable for workspace resolution.
- **Config file location**: `complytime.yaml` moves to `.complytime/complytime.yaml` with backward compatibility fallback.
- **Code changes**:
  - `internal/complytime/workspace.go`: Add `ResolveWorkspaceDir()`, `DetectConfigPath()`, modify `NewWorkspace()` signature
  - `internal/complytime/consts.go`: Add `WorkspaceEnvVar` constant
  - `cmd/complyctl/cli/options.go`: Add `Workspace` field and flag binding
  - `cmd/complyctl/cli/root.go`: Update `lazyLogWriter` to use resolved workspace
  - `cmd/complyctl/cli/init.go`, `list.go`, `scan.go`, `generate.go`: Pass resolved workspace to `NewWorkspace()`
  - `cmd/complyctl/cli/scan.go`: Update scan output directory construction
- **Tests**: Unit tests for workspace resolution, config detection, backward compatibility. Integration tests for flag, env var, and legacy config fallback.
- **Documentation**: Update README.md with `--workspace` flag examples. Add migration guide to CHANGELOG.md. Update AGENTS.md "Recent Changes" section.
- **Breaking changes**: None. The change is purely additive with backward compatibility.
- **Deprecation timeline**:
  - Current release: Support both locations, print deprecation warning for root location
  - Future releases: Continue supporting both locations
  - No hard removal date set — will be determined based on user adoption metrics
