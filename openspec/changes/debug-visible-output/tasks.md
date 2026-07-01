<!-- Tasks use [P] to mark parallel-eligible items within a phase. -->
<!-- Sequential tasks (no marker) run first, then [P] tasks can run concurrently. -->

## 1. Add teeWriter type and SetColorProfile to pkg/log

- [x] 1.1 Add `teeWriter` struct to `pkg/log/` implementing `io.Writer` that writes to a primary writer (stderr) and secondary writer (log file); errors from the secondary writer MUST NOT prevent output to the primary writer
- [x] 1.2 Add `NewTeeWriter(primary, secondary io.Writer) io.Writer` constructor
- [x] 1.3 Add `SetColorProfile(profile termenv.Profile)` method to `CharmHclog` that delegates to `c.logger.SetColorProfile(profile)`

## 2. Wire debug flag to tee output to stderr

- [x] 2.1 Refactor `enableDebug()` in `cmd/complyctl/cli/root.go` to accept `*os.File` (stderr) and `io.Writer` (file) parameters, reconstruct the logger with `log.NewLogger(teeWriter)`, set level to `hclog.Debug`, and force `termenv.ANSI256` color profile via `SetColorProfile` when `isatty.IsTerminal(stderr.Fd())` returns true and `termenv.EnvNoColor()` returns false
- [x] 2.2 Reorder `PersistentPreRun` in `cmd/complyctl/cli/root.go` so workspace resolution happens before `enableDebug()`, then print `Debug log: <path>` hint to stderr when `--debug` is active

## 3. Update flag description

- [x] 3.1 [P] Change `--debug` flag description in `cmd/complyctl/cli/options.go` from `"output debug logs"` to `"output debug logs to stderr and log file"`

## 4. Tests

- [x] 4.1 [P] Add unit test for `teeWriter` in `pkg/log/`: verify both primary and secondary buffers receive written data; verify primary receives data even when secondary writer returns an error
- [x] 4.2 [P] Add unit test verifying `enableDebug` with `debug=true` reconstructs the logger so that a subsequent `logger.Debug("test")` call writes output containing `DEBUG` to a buffer; save and restore the package-level `logger` via `t.Cleanup`
- [x] 4.3 [P] Add unit test verifying `enableDebug` with `debug=false` produces no output on the buffer; save and restore the package-level `logger` via `t.Cleanup`
- [x] 4.4 [P] Add unit test verifying `PersistentPreRun` prints `Debug log: <resolved-path>` to stderr when `--debug` is active; verify the hint line appears before any `DEBUG`-prefixed log lines in the captured buffer
- [x] 4.5 [P] Add unit test verifying no `Debug log:` hint appears on stderr when `--debug` is not set
- [x] 4.6 [P] Add unit test verifying `--debug` flag description reads `"output debug logs to stderr and log file"`
- [x] 4.7 [P] Add unit test for unwritable log directory: create a read-only temp directory, run with `--debug`, verify debug output still appears on the stderr buffer and a warning about log file creation failure is present
- [x] 4.8 [P] Add unit test verifying `enableDebug` with `debug=true` and `NO_COLOR=1` set via `t.Setenv` does NOT force `ANSIColor` profile; verify stderr output buffer contains no ANSI escape sequences

## 5. Integration audit

- [x] 5.1 Audit `tests/integration_test.sh` for `--debug` usage (line 522 uses `2>&1`) and verify tests remain correct with debug output on stderr; expected outcome: test remains correct because it asserts log file existence, not output content

## 6. Documentation

- [x] 6.1 [P] Update `CHANGELOG.md` with entry under `### Fixed` for `--debug` flag producing visible stderr output
- [x] 6.2 [P] Update `AGENTS.md` Recent Changes section with `debug-visible-output` entry

## 7. Verification

- [x] 7.1 Run `make test-unit` and confirm all tests pass
- [x] 7.2 Run `make lint` and confirm no lint violations (golangci-lint not installed locally; go vet and go fmt pass clean)
- [x] 7.3 Manual smoke test: run `complyctl --debug version` and `complyctl --debug list` and verify: (a) `Debug log: <path>` appears on stderr, (b) without --debug no hint appears, (c) --help shows updated flag description
<!-- spec-review: passed -->
<!-- code-review: passed -->
