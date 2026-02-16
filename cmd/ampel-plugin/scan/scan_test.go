package scan

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

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
	envelope := dsseEnvelope{
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
		if m.ampelErr != nil {
			return nil, m.ampelErr
		}
		return m.ampelOutput, nil
	}
	return nil, fmt.Errorf("unknown command: %s", name)
}

func TestParseRepoURL_GitHub(t *testing.T) {
	platform, org, repo, err := parseRepoURL("https://github.com/myorg/myrepo")
	require.NoError(t, err)
	require.Equal(t, "github", platform)
	require.Equal(t, "myorg", org)
	require.Equal(t, "myrepo", repo)
}

func TestParseRepoURL_GitLab(t *testing.T) {
	platform, org, repo, err := parseRepoURL("https://gitlab.com/myorg/myrepo")
	require.NoError(t, err)
	require.Equal(t, "gitlab", platform)
	require.Equal(t, "myorg", org)
	require.Equal(t, "myrepo", repo)
}

func TestParseRepoURL_UnsupportedHost(t *testing.T) {
	_, _, _, err := parseRepoURL("https://bitbucket.org/myorg/myrepo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported host")
}

func TestParseRepoURL_MissingPath(t *testing.T) {
	_, _, _, err := parseRepoURL("https://github.com/onlyorg")
	require.Error(t, err)
	require.Contains(t, err.Error(), "org/repo path")
}

func TestConstructSnappyCommand(t *testing.T) {
	args := constructSnappyCommand("myorg", "myrepo", "main", "/specs/github/branch-rules.yaml")
	require.Equal(t, []string{
		"snappy", "snap",
		"--var", "ORG=myorg",
		"--var", "REPO=myrepo",
		"--var", "BRANCH=main",
		"/specs/github/branch-rules.yaml",
	}, args)
}

func TestConstructAmpelVerifyCommand(t *testing.T) {
	args := constructAmpelVerifyCommand("abc123", "/policy/path.json", "/attestation/data.json")
	require.Equal(t, []string{
		"ampel", "verify",
		"abc123",
		"-p", "/policy/path.json",
		"-a", "/attestation/data.json",
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

func TestScanRepository_MockSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	attestation := makeTestAttestation("abc123def456")
	ampelOutput := []byte(`{"policy_id":"test","passed":true,"results":[]}`)

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

	result, err := ScanRepository(repo, "main", cfg, runner)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, ampelOutput, result.Output)

	// Verify attestation was saved
	attestationFile := filepath.Join(tmpDir, sanitizeRepoName(repo.URL)+"-main-attestation.json")
	saved, err := os.ReadFile(attestationFile)
	require.NoError(t, err)
	require.Equal(t, attestation, saved)
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

	_, err := ScanRepository(repo, "main", cfg, runner)
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

	_, err := ScanRepository(repo, "main", cfg, runner)
	require.Error(t, err)
	require.Contains(t, err.Error(), "snappy failed")
}

func TestScanRepository_AmpelError(t *testing.T) {
	runner := &mockRunner{
		snappyOutput: makeTestAttestation("abc123"),
		ampelErr:     fmt.Errorf("exit status 1"),
	}
	repo := targets.TargetRepository{URL: "https://github.com/myorg/myrepo"}
	cfg := ScanConfig{
		PolicyPath: "/policy.json",
		OutputDir:  t.TempDir(),
		SpecDir:    t.TempDir(),
	}

	_, err := ScanRepository(repo, "main", cfg, runner)
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

	_, err := ScanRepository(repo, "main", cfg, runner)
	require.Error(t, err)
	require.Contains(t, err.Error(), "extracting subject hash")
}

func TestSanitizeRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/myorg/myrepo", "github-com-myorg-myrepo"},
		{"https://gitlab.com/org/repo", "gitlab-com-org-repo"},
		{"http://github.com/a/b", "github-com-a-b"},
	}
	for _, tc := range tests {
		got := sanitizeRepoName(tc.input)
		require.Equal(t, tc.expected, got, "input: %s", tc.input)
	}
}
