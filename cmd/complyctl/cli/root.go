// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/version"
	"github.com/complytime/complyctl/pkg/log"
)

var (
	logger hclog.Logger
	lw     *lazyLogWriter
)

// lazyLogWriter defers log file creation until something actually writes to it.
// See FR-011 (workspace-configuration spec): log lives at {WorkspaceDir}/{LogFileName} (.complytime/complyctl.log).
type lazyLogWriter struct {
	once    sync.Once
	file    *os.File
	baseDir string
}

// SetWorkspace configures the base directory for resolving the workspace log path.
// Must be called before first Write() to take effect.
func (w *lazyLogWriter) SetWorkspace(baseDir string) {
	w.baseDir = baseDir
}

// Close closes the underlying log file if it was opened.
func (w *lazyLogWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *lazyLogWriter) Write(p []byte) (int, error) {
	w.once.Do(func() {
		baseDir := w.baseDir
		if baseDir == "" {
			baseDir = "."
		}
		logDir := filepath.Join(baseDir, complytime.WorkspaceDir)
		if err := os.MkdirAll(logDir, 0700); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create log directory: %v\n", err)
			return
		}
		logPath := filepath.Join(logDir, complytime.LogFileName)
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open log file: %v\n", err)
			return
		}
		w.file = f
	})
	if w.file == nil {
		return len(p), nil
	}
	return w.file.Write(p)
}

func init() {
	lw = &lazyLogWriter{}
	logger = log.NewLogger(lw)
}

// Error logs an error message to the workspace log file.
func Error(msg string) {
	logger.Error(msg)
}

func enableDebug(opts *Common) {
	if opts.Debug {
		logger.SetLevel(hclog.Debug)
	}
}

func New() *cobra.Command {

	var buf strings.Builder
	_ = version.WriteVersion(&buf)

	cmd := &cobra.Command{
		Use:           "complyctl [command]",
		Version:       buf.String(),
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.SetVersionTemplate(`{{.Version}}`)
	cmd.Flags().Bool("version", false, "version for complyctl")

	opts := Common{
		Output: Output{
			Out:    cmd.OutOrStdout(),
			ErrOut: cmd.ErrOrStderr(),
		},
	}
	opts.BindFlags(cmd.PersistentFlags())

	cmd.AddCommand(
		versionCmd(&opts),
		initCmd(&opts),
		getCmd(&opts),
		scanCmd(&opts),
		generateCmd(&opts),
		listCmd(&opts),
		providersCmd(&opts),
		doctorCmd(&opts),
	)
	cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		enableDebug(&opts)
		baseDir, err := opts.ResolveWorkspace()
		if err == nil {
			lw.SetWorkspace(baseDir)
		} else {
			lw.SetWorkspace(".")
		}
	}
	cmd.PersistentPostRun = func(_ *cobra.Command, _ []string) {
		_ = lw.Close()
	}

	return cmd
}
