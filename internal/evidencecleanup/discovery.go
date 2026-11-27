// SPDX-License-Identifier: Apache-2.0

package evidencecleanup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	managedDirectory   = ".complytime"
	resultsDirectory   = "results"
	readDirectoryBatch = 128
	excludedLayout     = "unrecognized provider results layout"
)

// DiscoveryOptions selects an existing workspace. Discovery neither initializes
// it nor loads its configuration. Callers must hold exclusive workspace access
// and check cleanup support and unresolved activity before invoking discovery.
type DiscoveryOptions struct {
	Workspace string
}

// Evaluation is an indivisible retained-layout unit, not an authorized deletion
// candidate. Path is workspace-relative. Identity records the opened directory
// for later revalidation. A nonempty Reason means its timestamp cannot be used;
// StartedAt is then zero. Valid timestamps still require AgeWindow checks.
// Contents, report relationships, activity, and removal permissions are not
// preflighted here; in particular, absent metadata never proves inactivity.
type Evaluation struct {
	Path      string
	Provider  string
	StartedAt time.Time
	Reason    string
	Identity  os.FileInfo
}

// DiscoveryExclusion identifies an unrecognized entry that discovery did not
// descend into. Path is workspace-relative; the entry remains untouched.
type DiscoveryExclusion struct {
	Path   string
	Reason string
}

// DiscoveryResult inventories provider evaluations and exclusions in stable path
// order. It is deliberately not a cleanup plan or a complete report inventory.
type DiscoveryResult struct {
	Evaluations []Evaluation
	Exclusions  []DiscoveryExclusion
}

type discoveryEntry struct {
	directory bool
	validate  func(*os.File) error
}

// Discover inventories only the three specified provider results roots through
// valid labels using descriptor-relative, no-follow, no-mount-crossing traversal.
// It never descends into an evaluation or reads metadata. Missing roots are
// empty inventories. Unsafe or incomplete traversal returns a nil result and a
// contextual error rather than a partial inventory that could authorize cleanup.
// Workspace aliases are resolved before pinning the existing directory; callers
// must keep the namespace stable, as required by the cleanup operating contract.
func Discover(opts DiscoveryOptions) (result *DiscoveryResult, err error) {
	if opts.Workspace == "" {
		return nil, fmt.Errorf("discover evaluations: workspace is required")
	}
	canonical, err := filepath.EvalSymlinks(opts.Workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve discovery workspace: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute discovery workspace: %w", err)
	}
	workspace, validate, err := openDiscoveryWorkspace(canonical)
	if err != nil {
		return nil, fmt.Errorf("open discovery workspace %q: %w", canonical, err)
	}
	defer func() {
		if closeErr := workspace.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close discovery workspace: %w", closeErr))
		}
		if err != nil {
			result = nil
		}
	}()
	result = &DiscoveryResult{}
	for _, provider := range []string{"ampel", "opa", "openscap"} {
		path := filepath.Join(managedDirectory, provider, resultsDirectory)
		if err := discoverRoot(workspace, path, provider, result); err != nil {
			return nil, fmt.Errorf("discover provider results %q: %w", path, err)
		}
	}
	if err := validate(); err != nil {
		return nil, fmt.Errorf("revalidate discovery workspace: %w", err)
	}
	sort.Slice(result.Evaluations, func(i, j int) bool {
		return result.Evaluations[i].Path < result.Evaluations[j].Path
	})
	sort.Slice(result.Exclusions, func(i, j int) bool {
		return result.Exclusions[i].Path < result.Exclusions[j].Path
	})
	return result, nil
}

func discoverRoot(workspace *os.File, path, provider string, result *DiscoveryResult) (err error) {
	root, err := openDiscoveryDirectory(workspace, path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	return discoverLabels(root, path, provider, result)
}

func discoverLabels(dir *os.File, path, provider string, result *DiscoveryResult) error {
	for {
		entries, err := dir.Readdirnames(readDirectoryBatch)
		for _, entry := range entries {
			if err := discoverEntry(dir, path, provider, entry, result); err != nil {
				return err
			}
		}
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("results directory disappeared %q: %w", path, err)
		}
		if err != nil {
			// ReadDir returns io.EOF only after all entries have been consumed.
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read provider labels %q: %w", path, err)
		}
	}
}

func discoverEntry(parent *os.File, path, provider string, name string,
	result *DiscoveryResult) (err error) {
	path = filepath.Join(path, name)
	inspected, err := inspectDiscoveryEntry(parent, name)
	if err != nil {
		return fmt.Errorf("inspect provider results entry %q: %w", path, err)
	}
	if !inspected.directory {
		result.Exclusions = append(result.Exclusions, DiscoveryExclusion{path, excludedLayout})
		return nil
	}
	startedAt, nameErr := ParseEvaluationName(name)
	unit := nameErr == nil || evaluationShape(name)
	if !unit && (strings.HasPrefix(name, evaluationPrefix) || !IsLabel(name)) {
		result.Exclusions = append(result.Exclusions, DiscoveryExclusion{path, excludedLayout})
		return nil
	}
	child, err := openDiscoveryDirectory(parent, name)
	if err != nil {
		return fmt.Errorf("open provider results entry %q: %w", path, err)
	}
	defer func() { err = errors.Join(err, child.Close()) }()
	if err := inspected.validate(child); err != nil {
		return fmt.Errorf("revalidate provider results entry %q: %w", path, err)
	}
	identity, err := child.Stat()
	if err != nil {
		return fmt.Errorf("inspect opened results directory %q: %w", path, err)
	}
	if !unit {
		return discoverLabels(child, path, provider, result)
	}
	evaluation := Evaluation{Path: path, Provider: provider, StartedAt: startedAt, Identity: identity}
	if nameErr != nil {
		evaluation.Reason = nameErr.Error()
	}
	result.Evaluations = append(result.Evaluations, evaluation)
	return nil
}

// Keep malformed dates in the inventory so a later report can protect their
// entire group. Unknown token formats are exclusions, never traversable labels.
func evaluationShape(name string) bool {
	remainder, found := strings.CutPrefix(name, evaluationPrefix)
	timestamp, token, separated := strings.Cut(remainder, evaluationSeparator)
	return found && separated && len(timestamp) == len(evaluationTimeLayout) &&
		len(token) == evaluationTokenSize && strings.Trim(token, lowercaseHexDigits) == ""
}
