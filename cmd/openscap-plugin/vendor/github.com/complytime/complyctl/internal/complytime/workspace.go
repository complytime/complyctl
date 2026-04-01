// SPDX-License-Identifier: Apache-2.0

package complytime

import (
	"fmt"
	"os"
	"path/filepath"
)

// Workspace manages loading, saving, and validating a complytime configuration file.
type Workspace struct {
	configPath string
	config     *WorkspaceConfig
}

// NewWorkspace returns a Workspace that operates on the conventional
// complytime.yaml in the current working directory.
func NewWorkspace() *Workspace {
	return &Workspace{configPath: WorkspaceConfigFile}
}

func (w *Workspace) Load() error {
	config, err := LoadFrom(w.configPath)
	if err != nil {
		return err
	}
	w.config = config
	return nil
}

// LoadAndValidate loads the workspace config and runs structural validation.
// Prefer this over separate Load() + Validate() calls in CLI entry points.
func (w *Workspace) LoadAndValidate() error {
	if err := w.Load(); err != nil {
		return err
	}
	return Validate(w.config)
}

func (w *Workspace) Save() error {
	if w.config == nil {
		return fmt.Errorf("no configuration to save")
	}
	return SaveTo(w.config, w.configPath)
}

func (w *Workspace) Config() *WorkspaceConfig {
	return w.config
}

func (w *Workspace) SetConfig(config *WorkspaceConfig) {
	w.config = config
}

func (w *Workspace) Exists() bool {
	_, err := os.Stat(w.configPath)
	return err == nil
}

func (w *Workspace) Path() string {
	return w.configPath
}

func (w *Workspace) EnsureDir() error {
	dir := filepath.Dir(w.configPath)
	if dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0700)
}
