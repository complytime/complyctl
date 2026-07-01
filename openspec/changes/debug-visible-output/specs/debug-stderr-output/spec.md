## ADDED Requirements

### Requirement: FR-001 Debug logs appear on stderr when --debug is active

When the user passes the `--debug` (or `-d`) flag, the CLI MUST write all
log messages (debug, info, warn, error) to stderr in addition to the log
file. Without `--debug`, log messages MUST continue to go only to the log
file. Color output on stderr MUST respect TTY detection and the `NO_COLOR`
environment variable.

#### Scenario: Debug flag emits logs to stderr

- **GIVEN** a workspace with a valid `complytime.yaml` and at least one configured policy
- **WHEN** user runs `complyctl --debug scan`
- **THEN** stderr contains lines with the `DEBUG` level prefix
- **AND** the log file at `.complytime/complyctl.log` contains the same log message text

#### Scenario: No debug flag keeps stderr quiet

- **GIVEN** a workspace with a valid `complytime.yaml`
- **WHEN** user runs `complyctl scan` without `--debug`
- **THEN** no log-level-prefixed messages appear on stderr (only user-facing output like errors and progress)
- **AND** info-level and above messages are written to the log file

#### Scenario: Debug output respects NO_COLOR

- **GIVEN** the environment variable `NO_COLOR=1` is set
- **WHEN** user runs `complyctl --debug scan`
- **THEN** debug output on stderr contains no ANSI escape codes

### Requirement: FR-002 Debug mode prints log file location hint

When `--debug` is active, the CLI MUST print a single line to stderr
indicating the log file path after workspace resolution. The format MUST
be `Debug log: <resolved-path>`. This format is informational output and
not a stable API contract.

#### Scenario: Log file hint on debug startup

- **GIVEN** a workspace at `/project` with a valid `complytime.yaml`
- **WHEN** user runs `complyctl --debug scan`
- **THEN** stderr includes `Debug log: /project/.complytime/complyctl.log`
  before any other debug log messages

#### Scenario: Log file hint with workspace flag

- **GIVEN** a workspace at `/other` with a valid `complytime.yaml`
- **WHEN** user runs `complyctl --debug --workspace /other scan`
- **THEN** stderr includes `Debug log: /other/.complytime/complyctl.log`

#### Scenario: Debug mode with unwritable log directory

- **GIVEN** the workspace directory is read-only
- **WHEN** user runs `complyctl --debug scan`
- **THEN** debug-level log messages still appear on stderr
- **AND** stderr includes a warning about the log file creation failure

### Requirement: FR-003 Debug flag description is accurate

The `--debug` flag help text MUST read
`"output debug logs to stderr and log file"`.

#### Scenario: Help text reflects behavior

- **GIVEN** a default complyctl installation
- **WHEN** user runs `complyctl --help`
- **THEN** the `--debug` flag description reads
  `output debug logs to stderr and log file`

## MODIFIED Requirements

(none)

## REMOVED Requirements

(none)
