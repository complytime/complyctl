package scan

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/cmd/ampel-plugin/intoto"
	"github.com/complytime/complyctl/cmd/ampel-plugin/targets"
)

func makeTestAttestation(hash string) []byte {
	stmt := map[string]interface{}{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []map[string]interface{}{
			{
				"name": "test-subject",
				"digest": map[string]string{
					"sha256": hash,
				},
			},
		},
		"predicateType": "http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml",
		"predicate":     map[string]interface{}{},
	}
	data, _ := json.Marshal(stmt)
	return data
}

func makeDSSEAttestation(hash string) []byte {
	stmt := map[string]interface{}{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []map[string]interface{}{
			{
				"name": "test-subject",
				"digest": map[string]string{
					"sha256": hash,
				},
			},
		},
		"predicateType": "http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml",
		"predicate":     map[string]interface{}{},
	}
	payload, _ := json.Marshal(stmt)
	envelope := intoto.Envelope{
		PayloadType: "application/vnd.in-toto+json",
		Payload:     base64.RawURLEncoding.EncodeToString(payload),
	}
	data, _ := json.Marshal(envelope)
	return data
}

// mockRunner differentiates between snappy and ampel calls.
type mockRunner struct {
	snappyOutput []byte
	ampelOutput  []byte
	snappyErr    error
	ampelErr     error
}

func (m *mockRunner) Run(name string, args ...string) ([]byte, error) {
	if name == "snappy" {
		if m.snappyErr != nil {
			return nil, m.snappyErr
		}
		return m.snappyOutput, nil
	}
	if name == "ampel" {
		// Write ampel output to the results path specified by --results-path
		for i, arg := range args {
			if arg == "--results-path" && i+1 < len(args) {
				_ = os.WriteFile(args[i+1], m.ampelOutput, 0600)
				break
			}
		}
		if m.ampelErr != nil {
			return nil, m.ampelErr
		}
		return nil, nil
	}
	return nil, fmt.Errorf("unknown command: %s", name)
}


func TestConstructSnappyCommand(t *testing.T) {
	args := constructSnappyCommand("myorg", "myrepo", "main", "/specs/github/branch-rules.yaml")
	require.Equal(t, []string{
		"snappy", "snap",
		"--var", "ORG=myorg",
		"--var", "REPO=myrepo",
		"--var", "BRANCH=main",
		"/specs/github/branch-rules.yaml",
		"--attest",
	}, args)
}

func TestConstructAmpelVerifyCommand(t *testing.T) {
	args := constructAmpelVerifyCommand("abc123", "/policy/path.json", "/attestation/data.json", "/results/output.json")
	require.Equal(t, []string{
		"ampel", "verify",
		"--subject-hash",
		"sha256:abc123",
		"-p", "/policy/path.json",
		"-a", "/attestation/data.json",
		"--attest-results",
		"--results-path", "/results/output.json",
	}, args)
}

func TestExtractSubjectHash_RawStatement(t *testing.T) {
	attestation := makeTestAttestation("deadbeef123456")
	hash, err := extractSubjectHash(attestation)
	require.NoError(t, err)
	require.Equal(t, "deadbeef123456", hash)
}

func TestExtractSubjectHash_DSSEEnvelope(t *testing.T) {
	attestation := makeDSSEAttestation("abc123def456")
	hash, err := extractSubjectHash(attestation)
	require.NoError(t, err)
	require.Equal(t, "abc123def456", hash)
}

func TestExtractSubjectHash_NoSubjects(t *testing.T) {
	stmt := map[string]interface{}{
		"_type":   "https://in-toto.io/Statement/v1",
		"subject": []map[string]interface{}{},
	}
	data, _ := json.Marshal(stmt)
	_, err := extractSubjectHash(data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no subjects")
}

func TestExtractSubjectHash_NoSHA256(t *testing.T) {
	stmt := map[string]interface{}{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []map[string]interface{}{
			{
				"name":   "test",
				"digest": map[string]string{"sha512": "somehash"},
			},
		},
	}
	data, _ := json.Marshal(stmt)
	_, err := extractSubjectHash(data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no sha256 digest")
}

func TestExtractSubjectHash_InvalidJSON(t *testing.T) {
	_, err := extractSubjectHash([]byte("not json"))
	require.Error(t, err)
}

func TestWriteSpecFiles(t *testing.T) {
	dir := t.TempDir()
	err := WriteSpecFiles(dir)
	require.NoError(t, err)

	specPath := filepath.Join(dir, "github", GitHubSpecFile)
	data, err := os.ReadFile(specPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "branch-rules.yaml")
	require.Contains(t, string(data), "${ORG}")
}

func TestResolveSpecPath(t *testing.T) {
	tests := []struct {
		specRef  string
		specDir  string
		expected string
	}{
		{"/opt/specs/custom.yaml", "/tmp/specs", "/opt/specs/custom.yaml"},
		{"github/branch-rules.yaml", "/tmp/specs", "/tmp/specs/github/branch-rules.yaml"},
		{"custom-check.yaml", "/tmp/specs", "/tmp/specs/custom-check.yaml"},
		{"custom-check.yml", "/tmp/specs", "/tmp/specs/custom-check.yml"},
		{"builtin-name", "/tmp/specs", "builtin-name"},
		{"builtin:github/branch-rules.yaml", "/tmp/specs", "builtin:github/branch-rules.yaml"},
	}
	for _, tc := range tests {
		got := ResolveSpecPath(tc.specRef, tc.specDir)
		require.Equal(t, tc.expected, got, "specRef=%s specDir=%s", tc.specRef, tc.specDir)
	}
}

func TestSanitizeSpecName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"github/branch-rules.yaml", "branch-rules"},
		{"/opt/specs/custom-check.yaml", "custom-check"},
		{"branch-rules.yml", "branch-rules"},
		{"builtin-name", "builtin-name"},
		{"builtin:github/branch-rules.yaml", "branch-rules"},
	}
	for _, tc := range tests {
		got := sanitizeSpecName(tc.input)
		require.Equal(t, tc.expected, got, "input: %s", tc.input)
	}
}

func TestScanRepository_MockSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	attestation := makeTestAttestation("abc123def456")
	ampelOutput := []byte(`{"predicate":{"status":"PASS","results":[]}}`)

	runner := &mockRunner{
		snappyOutput: attestation,
		ampelOutput:  ampelOutput,
	}
	repo := targets.TargetRepository{
		URL:      "https://github.com/myorg/myrepo",
		Branches: []string{"main"},
	}
	cfg := ScanConfig{
		PolicyPath: "/policy.json",
		OutputDir:  tmpDir,
		SpecDir:    filepath.Join(tmpDir, "specs"),
	}

	specPath := ResolveSpecPath("github/branch-rules.yaml", cfg.SpecDir)
	result, err := ScanRepository(repo, "main", specPath, cfg, runner)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, ampelOutput, result.Output)

	// Verify snappy attestation was saved with spec label in filename
	attestationFile := filepath.Join(tmpDir, targets.SanitizeRepoURL(repo.URL)+"-main-branch-rules-snappy.intoto.json")
	saved, err := os.ReadFile(attestationFile)
	require.NoError(t, err)
	require.Equal(t, attestation, saved)

	// Verify ampel verify result was saved with spec label in filename
	ampelResultFile := filepath.Join(tmpDir, targets.SanitizeRepoURL(repo.URL)+"-main-branch-rules-ampel.intoto.json")
	savedAmpel, err := os.ReadFile(ampelResultFile)
	require.NoError(t, err)
	require.Equal(t, ampelOutput, savedAmpel)
}

func TestScanRepository_GitLabUnsupported(t *testing.T) {
	runner := &mockRunner{}
	repo := targets.TargetRepository{
		URL:      "https://gitlab.com/myorg/myrepo",
		Branches: []string{"main"},
	}
	cfg := ScanConfig{
		PolicyPath: "/policy.json",
		OutputDir:  t.TempDir(),
		SpecDir:    t.TempDir(),
	}

	_, err := ScanRepository(repo, "main", "/specs/test.yaml", cfg, runner)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not supported")
}

func TestScanRepository_SnappyError(t *testing.T) {
	runner := &mockRunner{
		snappyErr: fmt.Errorf("exec: \"snappy\": executable file not found in $PATH"),
	}
	repo := targets.TargetRepository{URL: "https://github.com/myorg/myrepo"}
	cfg := ScanConfig{
		PolicyPath: "/policy.json",
		OutputDir:  t.TempDir(),
		SpecDir:    t.TempDir(),
	}

	_, err := ScanRepository(repo, "main", "/specs/test.yaml", cfg, runner)
	require.Error(t, err)
	require.Contains(t, err.Error(), "snappy failed")
}

func TestScanRepository_AmpelExitError_SavesResult(t *testing.T) {
	// ampel verify exits non-zero when policy checks fail, but output is valid
	tmpDir := t.TempDir()
	ampelOutput := []byte(`{"predicate":{"status":"FAIL","results":[]}}`)
	runner := &mockRunner{
		snappyOutput: makeTestAttestation("abc123"),
		ampelOutput:  ampelOutput,
		ampelErr:     &exec.ExitError{ProcessState: nil},
	}
	repo := targets.TargetRepository{URL: "https://github.com/myorg/myrepo"}
	cfg := ScanConfig{
		PolicyPath: "/policy.json",
		OutputDir:  tmpDir,
		SpecDir:    t.TempDir(),
	}

	specPath := ResolveSpecPath("github/branch-rules.yaml", cfg.SpecDir)
	result, err := ScanRepository(repo, "main", specPath, cfg, runner)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, ampelOutput, result.Output)

	// Verify ampel intoto file was saved despite non-zero exit
	ampelResultFile := filepath.Join(tmpDir, targets.SanitizeRepoURL(repo.URL)+"-main-branch-rules-ampel.intoto.json")
	saved, err := os.ReadFile(ampelResultFile)
	require.NoError(t, err)
	require.Equal(t, ampelOutput, saved)
}

func TestScanRepository_AmpelNonExecError(t *testing.T) {
	// Non-exec errors (e.g., command not found) are fatal
	runner := &mockRunner{
		snappyOutput: makeTestAttestation("abc123"),
		ampelErr:     fmt.Errorf("exec: \"ampel\": executable file not found in $PATH"),
	}
	repo := targets.TargetRepository{URL: "https://github.com/myorg/myrepo"}
	cfg := ScanConfig{
		PolicyPath: "/policy.json",
		OutputDir:  t.TempDir(),
		SpecDir:    t.TempDir(),
	}

	_, err := ScanRepository(repo, "main", "/specs/test.yaml", cfg, runner)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ampel verify failed")
}

func TestScanRepository_InvalidAttestationHash(t *testing.T) {
	// Snappy returns data that can't be parsed for a hash
	runner := &mockRunner{
		snappyOutput: []byte(`{"_type":"statement","subject":[]}`),
	}
	repo := targets.TargetRepository{URL: "https://github.com/myorg/myrepo"}
	cfg := ScanConfig{
		PolicyPath: "/policy.json",
		OutputDir:  t.TempDir(),
		SpecDir:    t.TempDir(),
	}

	_, err := ScanRepository(repo, "main", "/specs/test.yaml", cfg, runner)
	require.Error(t, err)
	require.Contains(t, err.Error(), "extracting subject hash")
}

