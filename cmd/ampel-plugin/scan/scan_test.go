package scan

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/cmd/ampel-plugin/targets"
)

type mockRunner struct {
	outputs map[string][]byte
	errors  map[string]error
}

func (m *mockRunner) Run(name string, args ...string) ([]byte, error) {
	if err, ok := m.errors[name]; ok {
		return nil, err
	}
	if out, ok := m.outputs[name]; ok {
		return out, nil
	}
	return []byte("{}"), nil
}

func TestConstructSnappyCommand_GitHub(t *testing.T) {
	repo := targets.TargetRepository{
		URL:      "https://github.com/myorg/myrepo",
		Branches: []string{"main"},
	}
	args := constructSnappyCommand(repo, "main", "/tmp/out")
	require.Equal(t, "snappy", args[0])
	require.Contains(t, args, "--repo")
	require.Contains(t, args, "https://github.com/myorg/myrepo")
	require.Contains(t, args, "--branch")
	require.Contains(t, args, "main")
}

func TestConstructSnappyCommand_GitLab(t *testing.T) {
	repo := targets.TargetRepository{
		URL:      "https://gitlab.com/myorg/myrepo",
		Branches: []string{"develop"},
	}
	args := constructSnappyCommand(repo, "develop", "/tmp/out")
	require.Equal(t, "snappy", args[0])
	require.Contains(t, args, "https://gitlab.com/myorg/myrepo")
	require.Contains(t, args, "develop")
}

func TestConstructAmpelVerifyCommand(t *testing.T) {
	args := constructAmpelVerifyCommand("/policy/path.json", "/attestation/data.json", "https://github.com/org/repo")
	require.Equal(t, "ampel", args[0])
	require.Equal(t, "verify", args[1])
	require.Contains(t, args, "--policy")
	require.Contains(t, args, "/policy/path.json")
	require.Contains(t, args, "--attestation")
	require.Contains(t, args, "/attestation/data.json")
}

func TestScanRepository_MockSuccess(t *testing.T) {
	runner := &mockRunner{
		outputs: map[string][]byte{
			"snappy": []byte("{}"),
			"ampel":  []byte(`{"policy_id":"test","passed":true,"results":[]}`),
		},
	}
	repo := targets.TargetRepository{
		URL:      "https://github.com/myorg/myrepo",
		Branches: []string{"main"},
	}
	cfg := ScanConfig{PolicyPath: "/policy.json", OutputDir: t.TempDir()}

	result, err := ScanRepository(repo, "main", cfg, runner)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Output)
}

func TestScanRepository_CommandNotFound(t *testing.T) {
	runner := &mockRunner{
		errors: map[string]error{
			"snappy": fmt.Errorf("exec: \"snappy\": executable file not found in $PATH"),
		},
	}
	repo := targets.TargetRepository{URL: "https://github.com/myorg/myrepo"}
	cfg := ScanConfig{PolicyPath: "/policy.json", OutputDir: t.TempDir()}

	_, err := ScanRepository(repo, "main", cfg, runner)
	require.Error(t, err)
	require.Contains(t, err.Error(), "snappy failed")
}

func TestScanRepository_AmpelError(t *testing.T) {
	runner := &mockRunner{
		outputs: map[string][]byte{
			"snappy": []byte("{}"),
		},
		errors: map[string]error{
			"ampel": fmt.Errorf("exit status 1"),
		},
	}
	repo := targets.TargetRepository{URL: "https://github.com/myorg/myrepo"}
	cfg := ScanConfig{PolicyPath: "/policy.json", OutputDir: t.TempDir()}

	_, err := ScanRepository(repo, "main", cfg, runner)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ampel verify failed")
}
