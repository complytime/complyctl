// SPDX-License-Identifier: Apache-2.0

package workspacecoord

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	activityDirectory      = ".complytime"
	coordinationDirectory  = "coordination"
	operationsDirectory    = "operations"
	activityVersion        = 1
	activityTokenBytes     = 16
	activityDirectoryDepth = 3
)

const (
	// OperationScan identifies a provider-starting scan invocation.
	OperationScan = "scan"
	// OperationGenerate identifies a provider-starting generation invocation.
	OperationGenerate = "generate"
)

// ErrUnresolvedActivity means cleanup cannot establish an inactive workspace.
var ErrUnresolvedActivity = errors.New("unresolved workspace activity")

// ActivityOptions describes one provider-starting invocation. Operation must be
// scan or generate and StartedAt must be nonzero. ProviderPIDs, when available
// before startup, are positive diagnostic hints only; they are never acted on.
type ActivityOptions struct {
	Operation    string
	StartedAt    time.Time
	ProviderPIDs []int
}

// Activity owns exactly one persistent record. Obtain it through BeginActivity;
// do not copy it or call its methods concurrently with Access methods. There is
// deliberately no deferred Close that removes the record. Access.Close closes
// its descriptor without clearing uncertainty.
type Activity struct {
	access      *Access
	file        *os.File
	name        string
	contents    []byte
	directories []os.FileInfo
	completed   bool
}

type activityRecord struct {
	Version      int       `json:"version"`
	ID           string    `json:"id"`
	Operation    string    `json:"operation"`
	StartedAt    time.Time `json:"started_at"`
	Workspace    string    `json:"workspace"`
	HostPID      int       `json:"host_pid"`
	ProviderPIDs []int     `json:"provider_pids,omitempty"`
}

// ActivityError identifies exact filesystem-discovered paths without trusting
// record contents. Paths may identify an unsafe directory rather than a record.
// Errors.Is(err, ErrUnresolvedActivity) is true for every ActivityError.
type ActivityError struct {
	Workspace string
	Paths     []string
	Cause     error
}

// Error reports blocked activity, quotes paths to escape terminal controls, and
// gives manual recovery conditions without constructing executable commands.
func (e *ActivityError) Error() string {
	paths := make([]string, len(e.Paths))
	for i, path := range e.Paths {
		paths[i] = fmt.Sprintf("%q", path)
	}
	return fmt.Sprintf("cleanup blocked in workspace %q: %v; "+
		"no evidence was deleted and no reports were deleted; "+
		"unresolved activity paths: %s: %v. "+
		"Manual recovery: pause scheduled jobs and prevent new scans against this workspace. "+
		"Stop all writers, including affected hosts, providers, and provider-launched child tools. "+
		"If shutdown is uncertain, restart the machine or VM that owns the writers, or stop "+
		"the complete isolated container environment containing them; restarting a terminal, "+
		"complyctl alone, or an unrelated container is insufficient. Prevent automatic job "+
		"restarts; stop writers in other machines/environments too, or do not proceed. "+
		"Only after all writers have stopped and new scans remain paused, manually remove "+
		"ONLY the exact reported activity records. Do not remove coordination directories, "+
		"reports, evidence, configuration, or unrelated files. A reported unsafe directory "+
		"requires inspection and repair, not recursive removal. Record contents and PIDs are "+
		"diagnostic hints only, not proof of inactivity or authority to kill processes. "+
		"There is no automatic stale-record or force override; record removal cannot bypass "+
		"a live workspace lock. Rerun cleanup preview before explicit cleanup. "+
		"See docs/EVIDENCE.md#interrupted-scan-recovery.", e.Workspace, ErrUnresolvedActivity,
		strings.Join(paths, ", "), e.Cause)
}

// Unwrap preserves both unresolved-activity and underlying filesystem errors.
func (e *ActivityError) Unwrap() error {
	return errors.Join(ErrUnresolvedActivity, e.Cause)
}

// Path returns the exact canonical path allocated by this invocation, never a
// path supplied by a record. It remains diagnostic after completion or Close.
func (a *Activity) Path() string {
	return filepath.Join(a.access.path, activityDirectory, coordinationDirectory,
		operationsDirectory, a.name)
}

func (opts ActivityOptions) validate() error {
	if opts.Operation != OperationScan && opts.Operation != OperationGenerate {
		return fmt.Errorf("activity operation must be scan or generate")
	}
	if opts.StartedAt.IsZero() {
		return fmt.Errorf("activity start time must not be zero")
	}
	for _, pid := range opts.ProviderPIDs {
		if pid <= 0 {
			return fmt.Errorf("activity provider PID must be positive")
		}
	}
	return nil
}

func (a *Access) activityContents(opts ActivityOptions, id string) ([]byte, error) {
	contents, err := json.Marshal(activityRecord{
		Version: activityVersion, ID: id, Operation: opts.Operation,
		StartedAt: opts.StartedAt.UTC(), Workspace: a.path, HostPID: os.Getpid(),
		ProviderPIDs: opts.ProviderPIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("encode workspace activity: %w", err)
	}
	return append(contents, '\n'), nil
}
