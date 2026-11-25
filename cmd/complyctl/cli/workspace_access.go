// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/complytime/complyctl/internal/workspacecoord"
	"github.com/complytime/complyctl/pkg/provider"
)

// acquireCommandAccess preserves existing commands on platforms without the
// coordination backend. On Linux/Darwin, failures must never run unprotected.
func acquireCommandAccess(baseDir string, mode workspacecoord.Mode) (*workspacecoord.Access, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return nil, nil
	}
	access, err := workspacecoord.Acquire(workspacecoord.Options{Workspace: baseDir, Mode: mode})
	if err != nil {
		return nil, fmt.Errorf("conflicting workspace operation cannot proceed: %w", err)
	}
	return access, nil
}

func closeCommandAccess(access *workspacecoord.Access) error {
	if access == nil {
		return nil
	}
	return access.Close()
}

// withWorkspaceMutation acquires once before any parent-artifact/configuration
// writes. Unresolved provider activity must not lose its parents after a crash.
func withWorkspaceMutation(baseDir string, work func() error) (err error) {
	access, err := acquireCommandAccess(baseDir, workspacecoord.Exclusive)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closeCommandAccess(access)) }()
	if access != nil {
		if err := access.CheckNoActivity(); err != nil {
			return err
		}
	}
	return work()
}

type providerOperation struct {
	manager *provider.Manager
}

// withProviderOperation owns one access/record interval including nested
// generation, publication, and teardown. A panic closes handles but cannot
// assert normal completion and therefore leaves its record intact.
func withProviderOperation(baseDir, operation string, work func(*providerOperation) error) (err error) {
	access, err := acquireCommandAccess(baseDir, workspacecoord.Shared)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closeCommandAccess(access)) }()
	var activity *workspacecoord.Activity
	if access != nil {
		activity, err = access.BeginActivity(workspacecoord.ActivityOptions{
			Operation: operation, StartedAt: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("protect provider startup: %w", err)
		}
	}
	op := &providerOperation{}
	returned := false
	defer func() {
		var shutdownErr error
		if op.manager != nil {
			if activity == nil {
				// Other operating systems keep the legacy lifecycle behavior.
				op.manager.Cleanup()
			} else {
				shutdownErr = op.manager.CleanupNormal()
			}
		}
		if !returned {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("host operation did not return normally"))
		}
		if activity != nil {
			if shutdownErr == nil {
				err = errors.Join(err, activity.CompleteNormal())
			} else {
				err = errors.Join(err, fmt.Errorf("retaining uncertain activity record %q: %v",
					activity.Path(), shutdownErr))
			}
		}
		err = errors.Join(err, shutdownErr)
	}()
	err = work(op)
	returned = true
	return err
}

func (op *providerOperation) loadProviders(providerDir string) (*provider.Manager, error) {
	mgr, err := provider.NewManager(providerDir, logger)
	if err != nil {
		return nil, fmt.Errorf("provider manager init failed: %w", err)
	}
	// Assign before loading: a partial load still belongs to this operation.
	op.manager = mgr
	if err := mgr.LoadProviders(); err != nil {
		return nil, fmt.Errorf("provider discovery failed: %w", err)
	}
	if len(mgr.ListProviders()) == 0 {
		return nil, fmt.Errorf("no providers found in %s (Describe may have failed)", providerDir)
	}
	return mgr, nil
}
