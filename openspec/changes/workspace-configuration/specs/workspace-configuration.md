# Workspace Configuration Specification

## ADDED Requirements

### FR-001: Workspace Directory Flag

complyctl commands MUST accept a `--workspace` / `-w` flag that specifies the workspace directory path.

**Scenario: Run command with workspace flag**
- Given a valid workspace directory at `/home/user/project`
- And a config file exists at `/home/user/project/.complytime/complytime.yaml`
- When I run `complyctl scan --workspace /home/user/project`
- Then the command MUST load config from `/home/user/project/.complytime/complytime.yaml`
- And all output files MUST be written relative to `/home/user/project/.complytime/`

### FR-002: Workspace Environment Variable

complyctl commands MUST support a `COMPLYTIME_WORKSPACE` environment variable for workspace directory resolution.

**Scenario: Run command with environment variable**
- Given a valid workspace directory at `/workspace/project`
- And `COMPLYTIME_WORKSPACE=/workspace/project` is set
- When I run `complyctl scan`
- Then the command MUST load config from `/workspace/project/.complytime/complytime.yaml`
- And all output files MUST be written relative to `/workspace/project/.complytime/`

### FR-003: Workspace Resolution Precedence

Workspace directory resolution MUST follow this precedence order:
1. `--workspace` flag value (highest priority)
2. `COMPLYTIME_WORKSPACE` environment variable
3. Current working directory (default)

**Scenario: Flag overrides environment variable**
- Given `COMPLYTIME_WORKSPACE=/path/one` is set
- When I run `complyctl scan --workspace /path/two`
- Then the workspace directory MUST be `/path/two`

### FR-004: Path Expansion

Workspace path resolution MUST expand `~/` prefix to the user's home directory.

**Scenario: Tilde expansion**
- Given the user's home directory is `/home/user`
- When I run `complyctl scan --workspace ~/projects/myapp`
- Then the workspace directory MUST resolve to `/home/user/projects/myapp`

### FR-005: Absolute Path Resolution

Workspace path resolution MUST convert relative paths to absolute paths.

**Scenario: Relative path resolution**
- Given the current directory is `/home/user/work`
- When I run `complyctl scan --workspace ../projects/myapp`
- Then the workspace directory MUST resolve to `/home/user/projects/myapp`

### FR-006: Workspace Path Validation

Workspace directory resolution MUST validate that the path exists and is a directory.

**Scenario: Nonexistent workspace path**
- Given `/nonexistent/path` does not exist
- When I run `complyctl scan --workspace /nonexistent/path`
- Then the command MUST fail with error "workspace directory does not exist: /nonexistent/path"

**Scenario: Workspace path is a file**
- Given `/etc/passwd` exists and is a file
- When I run `complyctl scan --workspace /etc/passwd`
- Then the command MUST fail with error "workspace path is not a directory: /etc/passwd"

### FR-007: Config File Subdirectory Location

complyctl MUST check for config file at `.complytime/complytime.yaml` within the workspace directory as the primary location.

**Scenario: Load config from new location**
- Given workspace directory is `/home/user/project`
- And config file exists at `/home/user/project/.complytime/complytime.yaml`
- When I run `complyctl scan --workspace /home/user/project`
- Then the command MUST load config from `/home/user/project/.complytime/complytime.yaml`

### FR-008: Legacy Config Location Fallback

complyctl MUST fall back to `complytime.yaml` at the workspace root when `.complytime/complytime.yaml` does not exist.

**Scenario: Fallback to legacy location**
- Given workspace directory is `/home/user/project`
- And config file exists at `/home/user/project/complytime.yaml`
- And config file does NOT exist at `/home/user/project/.complytime/complytime.yaml`
- When I run `complyctl scan --workspace /home/user/project`
- Then the command MUST load config from `/home/user/project/complytime.yaml`

### FR-009: Legacy Config Deprecation Warning

complyctl MUST print a deprecation warning when loading config from the legacy root location.

**Scenario: Deprecation warning for legacy location**
- Given config is loaded from legacy location `complytime.yaml`
- When any command executes
- Then a warning MUST be printed to stderr
- And the warning MUST include migration instructions

### FR-010: New Location Preferred When Both Exist

When both `.complytime/complytime.yaml` and `complytime.yaml` exist, complyctl MUST use the new location without printing a deprecation warning.

**Scenario: New location preferred**
- Given config file exists at `/home/user/project/.complytime/complytime.yaml`
- And config file exists at `/home/user/project/complytime.yaml`
- When I run `complyctl scan --workspace /home/user/project`
- Then the command MUST load config from `/home/user/project/.complytime/complytime.yaml`
- And no deprecation warning MUST be printed

### FR-011: Workspace-Relative Output Paths

All complyctl output files MUST be written relative to the resolved workspace directory.

**Scenario: Scan output in workspace**
- Given workspace directory is `/home/user/project`
- When I run `complyctl scan --workspace /home/user/project`
- Then scan output MUST be written to `/home/user/project/.complytime/scan/`

**Scenario: Log file in workspace**
- Given workspace directory is `/home/user/project`
- When I run any command with `--workspace /home/user/project`
- Then the log file MUST be written to `/home/user/project/.complytime/complyctl.log`

### FR-012: Workspace Constructor Parameter

The `NewWorkspace()` function MUST accept a `baseDir` parameter specifying the workspace root directory.

**Scenario: Constructor with base directory**
- Given I call `NewWorkspace("/home/user/project")`
- Then the Workspace MUST load config from the detected path within `/home/user/project`
- And the Workspace MUST expose the base directory via `BaseDir()` method

### FR-013: Config Path Detection Function

A `DetectConfigPath(baseDir string)` function MUST check for config in both locations and return the path, legacy flag, and error.

**Scenario: Detect new location**
- Given config exists at `/home/user/project/.complytime/complytime.yaml`
- When I call `DetectConfigPath("/home/user/project")`
- Then it MUST return `("/home/user/project/.complytime/complytime.yaml", false, nil)`

**Scenario: Detect legacy location**
- Given config exists at `/home/user/project/complytime.yaml`
- And config does NOT exist at `/home/user/project/.complytime/complytime.yaml`
- When I call `DetectConfigPath("/home/user/project")`
- Then it MUST return `("/home/user/project/complytime.yaml", true, nil)`

**Scenario: Config not found**
- Given config does NOT exist at either location
- When I call `DetectConfigPath("/home/user/project")`
- Then it MUST return `("", false, error)`
- And the error message MUST indicate both locations were checked

## MODIFIED Requirements

### MR-001: Workspace Constructor Signature

The `NewWorkspace()` function signature changes from zero parameters to one parameter.

**Before:**
```go
func NewWorkspace() *Workspace
```

**After:**
```go
func NewWorkspace(baseDir string) *Workspace
```

### MR-002: Common Struct Options

The `Common` struct gains a `Workspace` field for the `--workspace` flag value.

**Added Field:**
```go
type Common struct {
    Debug     bool
    Workspace string  // NEW
    Output
}
```

## REMOVED Requirements

None. This change is purely additive with backward compatibility.

## Constants

### CR-001: Workspace Environment Variable Name

A constant MUST define the workspace environment variable name.

```go
const WorkspaceEnvVar = "COMPLYTIME_WORKSPACE"
```

## Error Messages

### ER-001: Workspace Does Not Exist

When workspace path does not exist:
```text
workspace directory does not exist: <path>
```

### ER-002: Workspace Not A Directory

When workspace path is a file:
```text
workspace path is not a directory: <path>
```

### ER-003: Config Not Found

When config file is not found in either location:
```text
config file not found in <baseDir> (checked .complytime/complytime.yaml and complytime.yaml)
```

### ER-004: Invalid Path Resolution

When path resolution fails:
```text
failed to resolve workspace path: <error>
```

## Deprecation Warning Text

### DW-001: Legacy Config Location Warning

```text
WARNING: complytime.yaml found at repository root (legacy location).
Please move it to .complytime/complytime.yaml for better organization.
Run: mkdir -p .complytime && mv complytime.yaml .complytime/complytime.yaml

```

(Note: Warning includes trailing blank line for visual separation)
