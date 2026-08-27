// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
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
		// Only write logs when the workspace directory already exists.
		// Do not create it as a side effect of logging — creating
		// ~/.complytime when workspace is $HOME would trigger a false
		// legacy-directory deprecation warning from CheckLegacyDir().
		if _, err := os.Stat(logDir); err != nil {
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

func enableDebug(opts *Common, lw io.Writer, stderrW *os.File) {
	if opts.Debug {
		tw := log.NewTeeWriter(stderrW, lw)
		logger = log.NewLogger(tw)
		logger.SetLevel(hclog.Debug)
		if isatty.IsTerminal(stderrW.Fd()) && !termenv.EnvNoColor() {
			if cl, ok := logger.(interface {
				SetColorProfile(termenv.Profile)
			}); ok {
				cl.SetColorProfile(termenv.ANSI256)
			}
		}
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
		baseDir, err := opts.ResolveWorkspace()
		if err == nil {
			lw.SetWorkspace(baseDir)
		} else {
			lw.SetWorkspace(".")
		}
		enableDebug(&opts, lw, os.Stderr)
		if opts.Debug {
			resolvedBase := lw.baseDir
			if resolvedBase == "" {
				resolvedBase = "."
			}
			logPath := filepath.Join(
				resolvedBase, complytime.WorkspaceDir, complytime.LogFileName,
			)
			fmt.Fprintf(os.Stderr, "Debug log: %s\n", logPath)
		}
		complytime.CheckLegacyDir(os.Stderr)
	}
	cmd.PersistentPostRun = func(_ *cobra.Command, _ []string) {
		_ = lw.Close()
	}

	return cmd
}
