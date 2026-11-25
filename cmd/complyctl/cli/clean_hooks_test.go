// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package cli

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/workspacecoord"
	"github.com/complytime/complyctl/pkg/log"
)

type cleanSnapshotEntry struct {
	info   fs.FileInfo
	data   []byte
	target string
}

// Snapshot every entry, not just deletion candidates. Ignore access times because
// reads may update them; retain identity and modification times to detect rewrites.
func snapshotCleanWorkspace(t *testing.T, workspace string) map[string]cleanSnapshotEntry {
	t.Helper()
	entries := make(map[string]cleanSnapshotEntry)
	root, err := os.OpenRoot(workspace)
	require.NoError(t, err)
	defer func() { require.NoError(t, root.Close()) }()
	err = filepath.WalkDir(workspace, func(path string, entry fs.DirEntry, err error) error {
		require.NoError(t, err)
		info, err := entry.Info()
		require.NoError(t, err)
		relative, err := filepath.Rel(workspace, path)
		require.NoError(t, err)
		item := cleanSnapshotEntry{info: info}
		if info.Mode().IsRegular() {
			item.data, err = root.ReadFile(relative)
			require.NoError(t, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			item.target, err = root.Readlink(relative)
			require.NoError(t, err)
		}
		entries[relative] = item
		return nil
	})
	require.NoError(t, err)
	return entries
}

func assertCleanWorkspaceUnchanged(t *testing.T, workspace string, before map[string]cleanSnapshotEntry) {
	t.Helper()
	after := snapshotCleanWorkspace(t, workspace)
	require.Len(t, after, len(before), "workspace entry count changed")
	for path, original := range before {
		current, exists := after[path]
		require.True(t, exists, "workspace entry disappeared: %s", path)
		assert.Equal(t, original.data, current.data, "bytes changed: %s", path)
		assert.Equal(t, original.target, current.target, "symlink changed: %s", path)
		assert.Equal(t, original.info.Mode(), current.info.Mode(), "mode changed: %s", path)
		assert.Equal(t, original.info.Size(), current.info.Size(), "size changed: %s", path)
		assert.Equal(t, original.info.ModTime(), current.info.ModTime(), "mtime changed: %s", path)
		assert.True(t, os.SameFile(original.info, current.info), "identity changed: %s", path)
	}
}

func seedCleanHookWorkspace(t *testing.T, workspace string, existingLog bool) {
	t.Helper()
	unit := addCleanEvaluation(t, workspace, "retained", "20990101T000000Z")
	addCleanReport(t, workspace, unit)
	files := map[string]string{
		filepath.Join(complytime.WorkspaceDir, complytime.WorkspaceConfigFile): "invalid: [",
		filepath.Join(complytime.WorkspaceDir, "generation", "index.json"):     "generation state",
		"unrelated.txt": "untouched",
	}
	if existingLog {
		files[filepath.Join(complytime.WorkspaceDir, complytime.LogFileName)] = "keep this log\n"
	}
	for relative, data := range files {
		path := filepath.Join(workspace, relative)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
		require.NoError(t, os.Chmod(path, 0o400))
	}
}

// Exercise the same Error entry point used by main after Execute returns, plus
// emitted diagnostics: merely invoking today's quiet cleanup would miss a lazy logger.
func executeCleanHook(t *testing.T, debug bool, args ...string) (string, error) {
	t.Helper()
	cmd := New()
	var out, diagnostics bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&diagnostics)
	cmd.SetArgs(append([]string{"clean"}, args...))
	err := cmd.Execute()
	if err != nil {
		Error(err.Error())
		assert.Contains(t, diagnostics.String(), err.Error())
	}
	logger.Info("cleanup stream diagnostic")
	logger.Debug("cleanup debug diagnostic")
	assert.Contains(t, diagnostics.String(), "cleanup stream diagnostic")
	assert.Equal(t, debug, bytes.Contains(diagnostics.Bytes(), []byte("cleanup debug diagnostic")))
	assert.NotContains(t, diagnostics.String(), "Debug log:")
	assert.NotContains(t, out.String(), "cleanup stream diagnostic")
	assert.NotContains(t, out.String(), "cleanup debug diagnostic")
	assert.Nil(t, lw.file, "clean must not open the workspace file logger")
	return out.String(), err
}

func TestCleanCmd_HooksPreserveWholeWorkspace(t *testing.T) {
	for _, debug := range []bool{false, true} {
		for _, existingLog := range []bool{false, true} {
			t.Run(fmtCleanHookCase(debug, existingLog), func(t *testing.T) {
				for _, scenario := range []struct {
					name string
					args []string
					err  string
				}{
					{name: "preview", args: []string{"--older-than", "30d", "--dry-run"}},
					{name: "execute", args: []string{"--older-than", "30d"}},
					{name: "invalid-age", args: []string{"--older-than", "0d"}, err: "validate"},
					{name: "invalid-preview", args: []string{"--older-than", "0d", "--dry-run"}, err: "validate"},
					{name: "missing-age", args: []string{"--dry-run"}, err: "required flag"},
					{name: "extra-argument", args: []string{"extra"}, err: "unknown command"},
					{name: "unknown-flag", args: []string{"--unknown"}, err: "unknown flag"},
				} {
					t.Run(scenario.name, func(t *testing.T) {
						saveLogger(t)
						workspace := t.TempDir()
						requireCleanSupport(t, workspace)
						seedCleanHookWorkspace(t, workspace, existingLog)
						lw = &lazyLogWriter{baseDir: workspace}
						logger = log.NewLogger(lw)
						before := snapshotCleanWorkspace(t, workspace)
						args := append([]string{"--workspace", workspace}, scenario.args...)
						if debug {
							args = append([]string{"--debug"}, args...)
						}
						_, err := executeCleanHook(t, debug, args...)
						if scenario.err == "" {
							require.NoError(t, err)
						} else {
							require.ErrorContains(t, err, scenario.err)
						}
						assertCleanWorkspaceUnchanged(t, workspace, before)
					})
				}
			})
		}
	}
}

func fmtCleanHookCase(debug, existingLog bool) string {
	return map[bool]string{true: "debug", false: "normal"}[debug] + "/" +
		map[bool]string{true: "existing-log", false: "no-log"}[existingLog]
}

func TestCleanCmd_HookPreviewAliasesAndPrecedence(t *testing.T) {
	for _, selection := range []string{"default", "environment", "flag", "alias"} {
		t.Run(selection, func(t *testing.T) {
			saveLogger(t)
			root := t.TempDir()
			requireCleanSupport(t, root)
			current, environment, flag := filepath.Join(root, "current"),
				filepath.Join(root, "environment"), filepath.Join(root, "flag")
			for _, workspace := range []string{current, environment, flag} {
				// Old eligible groups make accidental execution observable.
				unit := addCleanEvaluation(t, workspace, "old", cleanTestStart)
				addCleanReport(t, workspace, unit)
			}
			alias := filepath.Join(root, "alias")
			require.NoError(t, os.Symlink(flag, alias))
			t.Chdir(current)
			t.Setenv(complytime.WorkspaceEnvVar, "")
			selected := current
			args := []string{"--debug", "--older-than", "30d", "--dry-run"}
			if selection != "default" {
				t.Setenv(complytime.WorkspaceEnvVar, environment)
				selected = environment
			}
			if selection == "flag" || selection == "alias" {
				selected = flag
				path := map[string]string{"flag": flag, "alias": alias}[selection]
				args = append(args, "--workspace", path)
			}
			lw = &lazyLogWriter{baseDir: current}
			logger = log.NewLogger(lw)
			before := snapshotCleanWorkspace(t, root)
			text, err := executeCleanHook(t, true, args...)
			require.NoError(t, err)
			assert.Contains(t, text, selected)
			assert.Contains(t, text, "members would-delete=2 retained=0")
			assertCleanWorkspaceUnchanged(t, root, before)
		})
	}
}

func TestCleanCmd_HookServiceErrorsPreserveWorkspace(t *testing.T) {
	for _, scenario := range []string{"missing-workspace", "busy", "activity", "malformed-report"} {
		for _, preview := range []bool{false, true} {
			t.Run(scenario+map[bool]string{true: "/preview", false: "/execute"}[preview], func(t *testing.T) {
				saveLogger(t)
				workspace := t.TempDir()
				requireCleanSupport(t, workspace)
				seedCleanHookWorkspace(t, workspace, true)
				selected := workspace
				var expected string
				switch scenario {
				case "missing-workspace":
					selected = filepath.Join(workspace, "absent")
					expected = "workspace directory does not exist"
				case "busy":
					access, err := workspacecoord.Acquire(workspacecoord.Options{
						Workspace: workspace, Mode: workspacecoord.Shared,
					})
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, access.Close()) })
					expected = "busy"
				case "activity":
					path := filepath.Join(workspace, complytime.WorkspaceDir, "coordination", "operations")
					require.NoError(t, os.MkdirAll(path, 0o700))
					require.NoError(t, os.WriteFile(filepath.Join(path, "interrupted.json"), []byte("{"), 0o600))
					expected = "interrupted.json"
				case "malformed-report":
					path := filepath.Join(workspace, complytime.WorkspaceDir, "scan", "eval-log-broken.json")
					require.NoError(t, os.WriteFile(path, []byte("{"), 0o600))
					expected = "report"
				}
				lw = &lazyLogWriter{baseDir: workspace}
				logger = log.NewLogger(lw)
				before := snapshotCleanWorkspace(t, workspace)
				args := []string{"--debug", "--workspace", selected, "--older-than", "30d"}
				if preview {
					args = append(args, "--dry-run")
				}
				_, err := executeCleanHook(t, true, args...)
				require.ErrorContains(t, err, expected)
				assertCleanWorkspaceUnchanged(t, workspace, before)
			})
		}
	}
}

func TestCleanCmd_HooksLeaveEmptyWorkspaceUninitialized(t *testing.T) {
	for _, debug := range []bool{false, true} {
		t.Run(fmtCleanHookCase(debug, false), func(t *testing.T) {
			saveLogger(t)
			root := t.TempDir()
			requireCleanSupport(t, root)
			workspace, home := filepath.Join(root, "workspace"), filepath.Join(root, "home")
			require.NoError(t, os.Mkdir(workspace, 0o750))
			require.NoError(t, os.MkdirAll(filepath.Join(home, complytime.WorkspaceDir), 0o700))
			t.Setenv("HOME", home)
			lw = &lazyLogWriter{baseDir: workspace}
			logger = log.NewLogger(lw)
			before := snapshotCleanWorkspace(t, root)
			cmd := New()
			var out, diagnostics bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&diagnostics)
			args := []string{"clean", "--workspace", workspace, "--older-than", "30d", "--dry-run"}
			if debug {
				args = append(args, "--debug")
			}
			cmd.SetArgs(args)
			require.NoError(t, cmd.Execute())
			assert.Contains(t, diagnostics.String(), "deprecated")
			assert.NotContains(t, diagnostics.String(), "Debug log:")
			assert.NotContains(t, out.String(), "deprecated")
			assert.Contains(t, out.String(), "members would-delete=0 retained=0")
			assert.Nil(t, lw.file)
			assertCleanWorkspaceUnchanged(t, root, before)
		})
	}
}

func TestNew_FileLoggingAfterClean(t *testing.T) {
	saveLogger(t)
	workspace := t.TempDir()
	requireCleanSupport(t, workspace)
	seedCleanHookWorkspace(t, workspace, false)
	lw = &lazyLogWriter{baseDir: workspace}
	logger = log.NewLogger(lw)
	_, err := executeCleanHook(t, true,
		"--debug", "--workspace", workspace, "--older-than", "30d", "--dry-run")
	require.NoError(t, err)
	// Other commands must recover their existing file-only logging convention.
	cmd := New()
	require.NoError(t, cmd.PersistentFlags().Set("workspace", workspace))
	cmd.PersistentPreRun(cmd, nil)
	t.Cleanup(func() { require.NoError(t, lw.Close()) })
	logger.Info("unrelated command file diagnostic")
	data, err := os.ReadFile(filepath.Join(workspace, complytime.WorkspaceDir, complytime.LogFileName))
	require.NoError(t, err)
	assert.Contains(t, string(data), "unrelated command file diagnostic")
	assert.NotContains(t, string(data), "cleanup stream diagnostic")
}
