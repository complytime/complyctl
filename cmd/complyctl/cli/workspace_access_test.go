// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gemaraproj/go-gemara/bundle"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/complytime/complyctl/internal/cache"
	"github.com/complytime/complyctl/internal/cache/cachetest"
	"github.com/complytime/complyctl/internal/complytime"
	"github.com/complytime/complyctl/internal/output"
	"github.com/complytime/complyctl/internal/policy"
	"github.com/complytime/complyctl/internal/workspacecoord"
	"github.com/complytime/complyctl/pkg/provider"
)

const (
	coordHelperEnv   = "COMPLYCTL_COORD_TEST_HELPER"
	coordRootEnv     = "COMPLYCTL_COORD_TEST_ROOT"
	coordModeEnv     = "COMPLYCTL_COORD_TEST_MODE"
	coordBarrierEnv  = "COMPLYCTL_COORD_TEST_BARRIER"
	coordCommandEnv  = "COMPLYCTL_COORD_TEST_COMMAND"
	coordTestTimeout = 30 * time.Second
	coordTestPolicy  = `metadata:
  id: coordination
  type: Policy
title: Coordination policy
adherence:
  evaluation-methods:
    - executor:
        id: fixture
  assessment-plans:
    - id: coordination-plan
      requirement-id: coordination-req
`
)

// TestMain also supplies an unchanged-protocol provider executable. The test
// binary is launched without arguments, exactly as a discovered provider is.
func TestMain(m *testing.M) {
	switch os.Getenv(coordHelperEnv) {
	case "provider":
		coordProviderMain()
		os.Exit(0)
	case "host":
		if err := coordHostMain(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	default:
		os.Exit(m.Run())
	}
}

func coordHostMain() error {
	if command := os.Getenv(coordCommandEnv); command == "clean" || command == "clean-preview" {
		return coordCleanMain(command == "clean-preview")
	}
	if os.Getenv(coordModeEnv) == "late-publication" {
		write := writePrettyReport
		writePrettyReport = func(eval *output.Evaluator, logPath, outDir, repository string) error {
			coordBarrier("publication")
			return write(eval, logPath, outDir, repository)
		}
	}
	if err := os.Setenv(coordHelperEnv, "provider"); err != nil {
		return err
	}
	root := os.Getenv(coordRootEnv)
	common := &Common{Workspace: filepath.Join(root, "workspace")}
	logger = hclog.NewNullLogger()
	switch os.Getenv(coordCommandEnv) {
	case workspacecoord.OperationGenerate:
		o := &generateOptions{Common: common, policyID: "coordination", timeout: coordTestTimeout,
			cacheDir: filepath.Join(root, "cache"), dataDir: filepath.Join(root, "data"),
			providerDir: filepath.Join(root, "providers")}
		return o.run(context.Background())
	default:
		o := &scanOptions{Common: common, policyID: "coordination", timeout: coordTestTimeout,
			cacheDir: filepath.Join(root, "cache"), dataDir: filepath.Join(root, "data"),
			providerDir: filepath.Join(root, "providers"), logFormat: complytime.EvalLogFormatJSON,
			formats: []string{complytime.OutputFormatPretty}}
		if os.Getenv(coordModeEnv) == "timeout" {
			o.timeout = time.Second
		}
		return o.run(context.Background())
	}
}

func coordProviderMain() {
	// This runs before go-plugin starts serving, not merely before Describe.
	root := os.Getenv(coordRootEnv)
	records, err := os.ReadDir(coordRecords(filepath.Join(root, "workspace")))
	if err != nil || len(records) != 1 {
		os.Exit(27)
	}
	// #nosec G703 -- root is the parent test's private t.TempDir, not provider/user input.
	if err := os.WriteFile(filepath.Join(root, "controlled-provider.pid"),
		[]byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		os.Exit(27)
	}
	coordBarrier("startup")
	provider.Serve(coordTestProvider{})
	// Serve has returned but the process has not exited. The host must still
	// hold access and retain the marker while go-plugin waits for teardown.
	coordBarrier("shutdown")
	if os.Getenv(coordModeEnv) == "abnormal-shutdown" {
		os.Exit(43)
	}
}

type coordTestProvider struct{}

func (coordTestProvider) Describe(context.Context, *provider.DescribeRequest) (*provider.DescribeResponse, error) {
	if os.Getenv(coordModeEnv) == "describe-error" {
		return nil, status.Error(codes.Unavailable, "describe interrupted")
	}
	return &provider.DescribeResponse{Healthy: true, Version: "test"}, nil
}

func (coordTestProvider) Generate(_ context.Context, req *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	coordBarrier("generate")
	if os.Getenv(coordModeEnv) == "generate-handled" {
		return &provider.GenerateResponse{ErrorMessage: "handled generation failure"}, nil
	}
	if os.Getenv(coordModeEnv) == "generate-rpc" {
		return nil, status.Error(codes.Unavailable, "generation interrupted")
	}
	path := filepath.Join(req.GlobalVariables[complytime.WorkspaceVarKey], complytime.WorkspaceDir,
		"fixture", "generated.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		return nil, err
	}
	return &provider.GenerateResponse{Success: true}, nil
}

func (coordTestProvider) Scan(ctx context.Context, _ *provider.ScanRequest) (*provider.ScanResponse, error) {
	coordBarrier("scan")
	switch os.Getenv(coordModeEnv) {
	case "crash":
		os.Exit(42)
	case "timeout":
		<-ctx.Done()
		return nil, ctx.Err()
	case "scan-rpc":
		return nil, status.Error(codes.Unavailable, "scan interrupted")
	}
	resp := &provider.ScanResponse{Assessments: []provider.AssessmentLog{{
		PlanID: "coordination-plan", Steps: []provider.Step{{Result: provider.ResultPassed}},
	}}}
	if os.Getenv(coordModeEnv) == "scan-handled" {
		resp.Errors = []string{"handled scan failure"}
	}
	return resp, nil
}

func coordBarrier(stage string) {
	address := os.Getenv(coordBarrierEnv)
	if address == "" {
		return
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		os.Exit(28)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		os.Exit(28)
	}
	// #nosec G704 -- host is fixed loopback; only the validated test listener port varies.
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), coordTestTimeout)
	if err != nil {
		os.Exit(28)
	}
	defer func() { _ = conn.Close() }() // Test helper exits on any barrier failure.
	if err := conn.SetDeadline(time.Now().Add(coordTestTimeout)); err != nil {
		os.Exit(28)
	}
	if _, err := fmt.Fprintln(conn, stage); err != nil {
		os.Exit(28)
	}
	var token [1]byte
	if _, err := io.ReadFull(conn, token[:]); err != nil {
		os.Exit(28)
	}
}

func coordRecords(baseDir string) string {
	return filepath.Join(baseDir, complytime.WorkspaceDir, "coordination", "operations")
}

func setupCoordHost(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	providers := filepath.Join(root, "providers")
	require.NoError(t, os.MkdirAll(workspace, 0o700))
	require.NoError(t, os.MkdirAll(providers, 0o700))
	executable, err := os.Executable()
	require.NoError(t, err)
	require.NoError(t, os.Symlink(executable, filepath.Join(providers, "complyctl-provider-fixture")))
	store, err := cache.NewCache(filepath.Join(root, "cache")).NewPolicyStore("coordination")
	require.NoError(t, err)
	source := cachetest.NewMockBundlePolicySource()
	source.SeedBundlePolicy("coordination", "v1", cachetest.DigestA, []bundle.File{{
		Name: "policy.yaml", Type: "Policy", Data: []byte(coordTestPolicy),
	}})
	_, err = source.CopyPolicy(context.Background(), "coordination", "v1", store)
	require.NoError(t, err)
	require.NoError(t, cache.SaveState(&cache.State{
		Policies: map[string]cache.PolicyState{"coordination": {Digest: cachetest.DigestA}},
	}, filepath.Join(root, "data")))
	ws := complytime.NewWorkspace(workspace)
	ws.SetConfig(&complytime.WorkspaceConfig{
		Policies: []complytime.PolicyEntry{{ID: "coordination", URL: "registry.example.com/coordination:v1"}},
		Targets:  []complytime.TargetConfig{{ID: "local", Policies: []string{"coordination"}}},
	})
	require.NoError(t, ws.EnsureDir())
	require.NoError(t, ws.Save())
	t.Setenv(complytime.SystemProviderDirEnvVar, t.TempDir())
	return root, workspace
}

func startCoordHost(t *testing.T, root, command, mode, address string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), coordTestTimeout)
	t.Cleanup(cancel)
	executable, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.CommandContext(ctx, executable)
	// An interrupted host may leave a provider holding inherited output pipes.
	// Bound Wait independently of the host deadline so failures remain diagnosable.
	cmd.WaitDelay = time.Second
	cmd.Env = append(os.Environ(), coordHelperEnv+"=host", coordRootEnv+"="+root,
		coordCommandEnv+"="+command, coordModeEnv+"="+mode, coordBarrierEnv+"="+address)
	var logs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &logs, &logs
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState != nil {
			return // Explicit waits have already completed this test's teardown.
		}
		// On assertion failure the host/provider can still be at a barrier.
		// Resolve the controlled provider before terminating its owning host.
		if pidBytes, err := os.ReadFile(filepath.Join(root, "controlled-provider.pid")); err == nil {
			if pid, err := strconv.Atoi(string(pidBytes)); err == nil {
				if process, err := os.FindProcess(pid); err == nil {
					_ = process.Kill() // Only the provider allocated by this test host.
					_ = process.Release()
				}
			}
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait() // Reap on assertion failures, too.
	})
	return cmd, &logs
}

func coordListener(t *testing.T) *net.TCPListener {
	t.Helper()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	return listener
}

func awaitCoordStage(t *testing.T, listener *net.TCPListener, stage string) net.Conn {
	t.Helper()
	require.NoError(t, listener.SetDeadline(time.Now().Add(coordTestTimeout)))
	conn, err := listener.Accept()
	require.NoError(t, err, "waiting for %s", stage)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, conn.SetDeadline(time.Now().Add(coordTestTimeout)))
	got, err := bufio.NewReader(conn).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, stage, strings.TrimSpace(got))
	return conn
}

func releaseCoordStage(t *testing.T, conn net.Conn) {
	t.Helper()
	_, err := conn.Write([]byte{1})
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func assertCoordActive(t *testing.T, workspace, operation string) {
	t.Helper()
	entries, err := os.ReadDir(coordRecords(workspace))
	require.NoError(t, err)
	require.Len(t, entries, 1, "nested generation must reuse the invocation marker")
	contents, err := os.ReadFile(filepath.Join(coordRecords(workspace), entries[0].Name()))
	require.NoError(t, err)
	assert.Contains(t, string(contents), `"operation":"`+operation+`"`)
	access, err := workspacecoord.Acquire(workspacecoord.Options{
		Workspace: workspace, Mode: workspacecoord.Exclusive,
	})
	if access != nil {
		require.NoError(t, access.Close())
	}
	require.ErrorIs(t, err, workspacecoord.ErrBusy)
}

func TestProviderCommands_CoordinationThroughShutdown(t *testing.T) {
	for _, command := range []string{workspacecoord.OperationScan, workspacecoord.OperationGenerate} {
		t.Run(command, func(t *testing.T) {
			root, workspace := setupCoordHost(t)
			listener := coordListener(t)
			cmd, logs := startCoordHost(t, root, command, "", listener.Addr().String())
			stages := []string{"startup", "generate"}
			if command == workspacecoord.OperationScan {
				stages = append(stages, "scan")
			}
			for _, stage := range stages {
				conn := awaitCoordStage(t, listener, stage)
				assertCoordActive(t, workspace, command)
				if stage == "startup" {
					otherRoot, otherWorkspace := setupCoordHost(t)
					otherHost, otherLogs := startCoordHost(t, otherRoot,
						workspacecoord.OperationGenerate, "", "")
					require.NoError(t, otherHost.Wait(), otherLogs.String())
					assertCoordResolved(t, otherWorkspace, false)
				}
				other, err := workspacecoord.Acquire(workspacecoord.Options{
					Workspace: t.TempDir(), Mode: workspacecoord.Exclusive,
				})
				require.NoError(t, err)
				require.NoError(t, other.Close())
				releaseCoordStage(t, conn)
			}
			conn := awaitCoordStage(t, listener, "shutdown")
			assertCoordActive(t, workspace, command)
			if command == workspacecoord.OperationScan {
				reports, err := os.ReadDir(filepath.Join(workspace, complytime.WorkspaceDir, complytime.ScanOutputDir))
				require.NoError(t, err)
				assert.Len(t, reports, 2, "EvaluationLog and Markdown must precede teardown")
			}
			releaseCoordStage(t, conn)
			require.NoError(t, cmd.Wait(), logs.String())
			assertCoordResolved(t, workspace, false)
		})
	}
}

func assertCoordResolved(t *testing.T, workspace string, unresolved bool) {
	t.Helper()
	access, err := workspacecoord.Acquire(workspacecoord.Options{
		Workspace: workspace, Mode: workspacecoord.Exclusive,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, access.Close()) }()
	err = access.CheckNoActivity()
	if unresolved {
		require.ErrorIs(t, err, workspacecoord.ErrUnresolvedActivity)
	} else {
		require.NoError(t, err)
	}
}

func TestProviderCommands_HandledVersusUncertain(t *testing.T) {
	for _, tc := range []struct {
		mode, command string
		unresolved    bool
	}{
		{"scan-handled", workspacecoord.OperationScan, false},
		{"generate-handled", workspacecoord.OperationGenerate, false},
		{"scan-rpc", workspacecoord.OperationScan, true},
		{"generate-rpc", workspacecoord.OperationGenerate, true},
		{"describe-error", workspacecoord.OperationScan, true},
		{"crash", workspacecoord.OperationScan, true},
		{"timeout", workspacecoord.OperationScan, true},
		{"abnormal-shutdown", workspacecoord.OperationScan, true},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			root, workspace := setupCoordHost(t)
			cmd, logs := startCoordHost(t, root, tc.command, tc.mode, "")
			require.Error(t, cmd.Wait(), logs.String())
			assertCoordResolved(t, workspace, tc.unresolved)
			if tc.unresolved {
				assert.Contains(t, logs.String(), "retaining uncertain activity record")
			}
		})
	}
}

func TestWorkspaceMutation_BusyBeforeWrites(t *testing.T) {
	root, workspace := setupCoordHost(t)
	state := &cache.State{Complypacks: map[string]cache.PolicyState{
		"pack": {EvaluatorID: "fixture"},
	}}
	artifact := filepath.Join(workspace, complytime.WorkspaceDir, "fixture", "keep")
	require.NoError(t, os.MkdirAll(filepath.Dir(artifact), 0o700))
	require.NoError(t, os.WriteFile(artifact, []byte("keep"), 0o600))
	require.NoError(t, policy.SaveGenerationState(workspace, "coordination",
		&policy.GenerationState{PolicyID: "coordination", EvaluatorIDs: []string{"fixture"}}))
	access, err := workspacecoord.Acquire(workspacecoord.Options{Workspace: workspace, Mode: workspacecoord.Shared})
	require.NoError(t, err)
	defer func() { require.NoError(t, access.Close()) }()
	require.ErrorIs(t, invalidateGenerationForComplypack(state, "pack", workspace), workspacecoord.ErrBusy)
	contents, err := os.ReadFile(artifact)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(contents))
	gen, err := policy.LoadGenerationState(workspace, "coordination")
	require.NoError(t, err)
	require.NotNil(t, gen)
	assert.Equal(t, []string{"fixture"}, gen.EvaluatorIDs)
	init := &initOptions{Common: &Common{Workspace: workspace}}
	require.ErrorIs(t, init.run(), workspacecoord.ErrBusy)
	// Existing config bytes must survive even when init refuses before Exists.
	assert.FileExists(t, filepath.Join(workspace, complytime.WorkspaceDir, complytime.WorkspaceConfigFile))
	assert.DirExists(t, filepath.Join(root, "providers"))
}

func TestProviderCommands_ExclusiveRefusesBeforeStartup(t *testing.T) {
	for _, command := range []string{workspacecoord.OperationScan, workspacecoord.OperationGenerate} {
		t.Run(command, func(t *testing.T) {
			root, workspace := setupCoordHost(t)
			access, err := workspacecoord.Acquire(workspacecoord.Options{Workspace: workspace, Mode: workspacecoord.Exclusive})
			require.NoError(t, err)
			defer func() { require.NoError(t, access.Close()) }()
			cmd, logs := startCoordHost(t, root, command, "", "")
			require.Error(t, cmd.Wait())
			assert.Contains(t, logs.String(), "workspace busy")
			assert.NoDirExists(t, coordRecords(workspace))
			assert.NoDirExists(t, filepath.Join(workspace, complytime.WorkspaceDir, "fixture"))
			assert.NoDirExists(t, filepath.Join(workspace, complytime.WorkspaceDir, complytime.ScanOutputDir))
		})
	}
}

func TestProviderOperation_LateReportPublication(t *testing.T) {
	_, workspace := setupCoordHost(t)
	published := filepath.Join(workspace, complytime.WorkspaceDir, complytime.ScanOutputDir)
	reached := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releasePublisher := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releasePublisher)
	done := make(chan error, 1)
	go func() {
		done <- withProviderOperation(workspace, workspacecoord.OperationScan, func(*providerOperation) error {
			// Provider results have returned, but the report publisher has not
			// finished. Exercise the real writer inside the operation interval.
			close(reached)
			<-release
			eval := output.NewEvaluator("coordination", "local", nil, nil, nil, nil)
			return writeScanReports([]string{complytime.OutputFormatPretty}, complytime.EvalLogFormatJSON,
				eval, published, "coordination")
		})
	}()
	select {
	case <-reached:
	case err := <-done:
		t.Fatalf("operation ended before publication barrier: %v", err)
	case <-time.After(coordTestTimeout):
		t.Fatal("operation did not reach publication barrier")
	}
	assertCoordActive(t, workspace, workspacecoord.OperationScan)
	assert.NoDirExists(t, published)
	releasePublisher()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(coordTestTimeout):
		t.Fatal("operation did not finish publication")
	}
	reports, err := os.ReadDir(published)
	require.NoError(t, err)
	assert.Len(t, reports, 2)
	assertCoordResolved(t, workspace, false)
}

func TestProviderOperation_PanicRetainsRecordAndReleasesAccess(t *testing.T) {
	_, workspace := setupCoordHost(t)
	require.PanicsWithValue(t, "interrupted publication", func() {
		_ = withProviderOperation(workspace, workspacecoord.OperationScan,
			func(*providerOperation) error { panic("interrupted publication") })
	})
	assertCoordResolved(t, workspace, true)
}

func TestProviderOperation_ReportFailureClearsAfterNormalShutdown(t *testing.T) {
	root, workspace := setupCoordHost(t)
	t.Setenv(coordHelperEnv, "provider")
	t.Setenv(coordRootEnv, root)
	t.Setenv(coordBarrierEnv, "")
	t.Setenv(coordModeEnv, "")
	previousLogger := logger
	logger = hclog.NewNullLogger()
	t.Cleanup(func() { logger = previousLogger })
	blocked := filepath.Join(workspace, complytime.WorkspaceDir, complytime.ScanOutputDir)
	require.NoError(t, os.WriteFile(blocked, []byte("keep"), 0o600))
	err := withProviderOperation(workspace, workspacecoord.OperationScan, func(op *providerOperation) error {
		if _, err := op.loadProviders(filepath.Join(root, "providers")); err != nil {
			return err
		}
		eval := output.NewEvaluator("coordination", "local", nil, nil, nil, nil)
		return writeScanReports([]string{complytime.OutputFormatPretty}, complytime.EvalLogFormatJSON,
			eval, blocked, "coordination")
	})
	require.ErrorContains(t, err, "not a directory")
	assert.NotContains(t, err.Error(), "retaining uncertain activity record")
	contents, err := os.ReadFile(blocked)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(contents))
	assertCoordResolved(t, workspace, false)
}

func TestProviderCommands_ForcedShutdownRetainsRecord(t *testing.T) {
	root, workspace := setupCoordHost(t)
	listener := coordListener(t)
	cmd, logs := startCoordHost(t, root, workspacecoord.OperationScan, "", listener.Addr().String())
	for _, stage := range []string{"startup", "generate", "scan"} {
		releaseCoordStage(t, awaitCoordStage(t, listener, stage))
	}
	conn := awaitCoordStage(t, listener, "shutdown")
	assertCoordActive(t, workspace, workspacecoord.OperationScan)
	// Do not release the provider's exit barrier. go-plugin's own bounded
	// shutdown must force-kill it; PID termination must not clear the record.
	require.Error(t, cmd.Wait())
	assert.Contains(t, logs.String(), "retaining uncertain activity record")
	_, err := io.ReadAll(conn)
	require.NoError(t, err)
	assertCoordResolved(t, workspace, true)
}

func TestProviderCommands_KilledHostRetainsRecord(t *testing.T) {
	root, workspace := setupCoordHost(t)
	listener := coordListener(t)
	cmd, _ := startCoordHost(t, root, workspacecoord.OperationScan, "", listener.Addr().String())
	conn := awaitCoordStage(t, listener, "startup")
	assertCoordActive(t, workspace, workspacecoord.OperationScan)
	pidBytes, err := os.ReadFile(filepath.Join(root, "controlled-provider.pid"))
	require.NoError(t, err)
	pid, err := strconv.Atoi(string(pidBytes))
	require.NoError(t, err)
	process, err := os.FindProcess(pid)
	require.NoError(t, err)
	defer func() { _ = process.Release() }()
	t.Cleanup(func() { _ = process.Kill() }) // This PID belongs to our barrier-controlled provider.
	require.NoError(t, cmd.Process.Kill())
	// The controlled provider has inherited stdout/stderr pipes. Stop it
	// before Wait so those pipes cannot keep the test host's Wait blocked.
	require.NoError(t, process.Kill())
	_, err = io.ReadAll(conn)
	require.NoError(t, err)
	require.Error(t, cmd.Wait())
	assertCoordResolved(t, workspace, true)
}

func TestProviderCommands_ActivityFailurePreventsStartup(t *testing.T) {
	root, workspace := setupCoordHost(t)
	coordination := filepath.Join(workspace, complytime.WorkspaceDir, "coordination")
	require.NoError(t, os.WriteFile(coordination, []byte("not a directory"), 0o600))
	cmd, logs := startCoordHost(t, root, workspacecoord.OperationGenerate, "", "")
	require.Error(t, cmd.Wait())
	assert.Contains(t, logs.String(), "protect provider startup")
	assert.NoFileExists(t, filepath.Join(root, "controlled-provider.pid"))
	contents, err := os.ReadFile(coordination)
	require.NoError(t, err)
	assert.Equal(t, "not a directory", string(contents))
}

func TestWorkspaceMutation_UnresolvedActivityPreventsRemoval(t *testing.T) {
	_, workspace := setupCoordHost(t)
	access, err := workspacecoord.Acquire(workspacecoord.Options{Workspace: workspace, Mode: workspacecoord.Shared})
	require.NoError(t, err)
	activity, err := access.BeginActivity(workspacecoord.ActivityOptions{
		Operation: workspacecoord.OperationScan, StartedAt: time.Now(),
	})
	require.NoError(t, err)
	require.NoError(t, access.Close())
	state := &cache.State{Complypacks: map[string]cache.PolicyState{"pack": {EvaluatorID: "fixture"}}}
	artifact := filepath.Join(workspace, complytime.WorkspaceDir, "fixture", "keep")
	require.NoError(t, os.MkdirAll(filepath.Dir(artifact), 0o700))
	require.NoError(t, os.WriteFile(artifact, []byte("keep"), 0o600))
	require.ErrorIs(t, invalidateGenerationForComplypack(state, "pack", workspace),
		workspacecoord.ErrUnresolvedActivity)
	contents, err := os.ReadFile(artifact)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(contents))
	assert.FileExists(t, activity.Path())
}

func TestProviderOperation_CleanupIsManagerScoped(t *testing.T) {
	firstRoot, firstWorkspace := setupCoordHost(t)
	secondRoot, secondWorkspace := setupCoordHost(t)
	t.Setenv(coordHelperEnv, "provider")
	t.Setenv(coordBarrierEnv, "")
	t.Setenv(coordModeEnv, "")
	t.Setenv(coordRootEnv, firstRoot)
	previousLogger := logger
	logger = hclog.NewNullLogger()
	t.Cleanup(func() { logger = previousLogger })
	err := withProviderOperation(firstWorkspace, workspacecoord.OperationScan, func(first *providerOperation) error {
		mgr, err := first.loadProviders(filepath.Join(firstRoot, "providers"))
		if err != nil {
			return err
		}
		t.Setenv(coordRootEnv, secondRoot)
		if err := withProviderOperation(secondWorkspace, workspacecoord.OperationGenerate,
			func(second *providerOperation) error {
				_, err := second.loadProviders(filepath.Join(secondRoot, "providers"))
				return err
			}); err != nil {
			return err
		}
		// A process-global CleanupClients would have killed the first provider.
		result, err := mgr.RouteScanResult(context.Background(), "fixture", nil)
		if err != nil {
			return err
		}
		require.Empty(t, result.Errors)
		require.Len(t, result.Assessments, 1)
		assert.Equal(t, "coordination-plan", result.Assessments[0].PlanID)
		return nil
	})
	require.NoError(t, err)
	assertCoordResolved(t, firstWorkspace, false)
	assertCoordResolved(t, secondWorkspace, false)
}
