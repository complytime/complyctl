#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Integration test for the Gemara-native complyctl workflow.
# Exercises: get → list → generate → scan (oscal, pretty, sarif)
#
# Run locally:  make test-integration
# Run directly: ./tests/integration_test.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BINARY="${REPO_ROOT}/bin/complyctl"
MOCK_REGISTRY="${REPO_ROOT}/bin/mock-oci-registry"
TEST_PLUGIN="${REPO_ROOT}/bin/complyctl-provider-test"

POLICY_ID="policies/test-branch-protection"
REGISTRY_PORT="${GEMARA_SERVICE_PORT:-8765}"
REGISTRY_URL="http://localhost:${REGISTRY_PORT}"

WORK_DIR=""
TEST_HOME=""
REGISTRY_PID=""
PASSED=0
FAILED=0

cleanup() {
    if [[ -n "${REGISTRY_PID}" ]]; then
        kill "${REGISTRY_PID}" 2>/dev/null || true
        wait "${REGISTRY_PID}" 2>/dev/null || true
    fi
    [[ -n "${WORK_DIR}" ]] && rm -rf "${WORK_DIR}"
    [[ -n "${TEST_HOME}" ]] && rm -rf "${TEST_HOME}"
}
trap cleanup EXIT

# --- Assertions ---

pass() {
    PASSED=$((PASSED + 1))
    echo "  PASS: $1" >&2
}

fail() {
    FAILED=$((FAILED + 1))
    echo "  FAIL: $1" >&2
}

assert_contains() {
    local label="$1" haystack="$2" needle="$3"
    if echo "${haystack}" | grep -q "${needle}"; then
        pass "${label}"
    else
        fail "${label}: expected output to contain '${needle}'"
        echo "  --- actual output ---" >&2
        echo "${haystack}" | head -20 >&2
        echo "  ---" >&2
    fi
}

# Prints the matching file path to stdout for capture.
# Status messages go to stderr.
assert_file_exists() {
    local label="$1" pattern="$2"
    local match dir base
    dir="$(dirname "${pattern}")"
    base="$(basename "${pattern}")"
    match=$(find "${dir}" -maxdepth 1 -name "${base}" -print -quit 2>/dev/null) || true
    if [[ -n "${match}" && -s "${match}" ]]; then
        pass "${label}"
        echo "${match}"
    else
        fail "${label}: no non-empty file matching ${pattern}"
        echo ""
    fi
}

assert_json_eq() {
    local label="$1" file="$2" query="$3" expected="$4"
    local actual
    actual=$(jq -r "${query}" "${file}" 2>/dev/null) || true
    if [[ "${actual}" == "${expected}" ]]; then
        pass "${label}"
    else
        fail "${label}: expected '${expected}', got '${actual}'"
    fi
}

assert_json_gte() {
    local label="$1" file="$2" query="$3" min="$4"
    local actual
    actual=$(jq -r "${query}" "${file}" 2>/dev/null) || true
    if [[ "${actual}" -ge "${min}" ]] 2>/dev/null; then
        pass "${label}"
    else
        fail "${label}: expected >= ${min}, got '${actual}'"
    fi
}

# --- Prerequisites ---

echo "=== Prerequisites ==="
for bin in "${BINARY}" "${MOCK_REGISTRY}" "${TEST_PLUGIN}"; do
    if [[ ! -x "${bin}" ]]; then
        echo "FATAL: ${bin} not found. Run 'make build build-test-plugin' first."
        exit 1
    fi
done

for cmd in jq curl; do
    if ! command -v "${cmd}" >/dev/null 2>&1; then
        echo "FATAL: '${cmd}' is required but not installed."
        exit 1
    fi
done
echo "  All prerequisites met."

# --- Setup ---

echo "=== Setup ==="
TEST_HOME="$(mktemp -d)"
WORK_DIR="$(mktemp -d)"
export HOME="${TEST_HOME}"

mkdir -p "${TEST_HOME}/.complytime/providers"
cp "${TEST_PLUGIN}" "${TEST_HOME}/.complytime/providers/complyctl-provider-ampel"
echo "  HOME=${TEST_HOME}"
echo "  WORK=${WORK_DIR}"

GEMARA_SERVICE_PORT="${REGISTRY_PORT}" "${MOCK_REGISTRY}" &
REGISTRY_PID=$!

for _ in $(seq 1 15); do
    if curl -sf "${REGISTRY_URL}/v2/" >/dev/null 2>&1; then
        break
    fi
    sleep 0.5
done
if ! curl -sf "${REGISTRY_URL}/v2/" >/dev/null 2>&1; then
    echo "FATAL: mock registry did not start on ${REGISTRY_URL}"
    exit 1
fi
echo "  Mock registry ready (PID ${REGISTRY_PID})"

cat > "${WORK_DIR}/complytime.yaml" <<EOF
policies:
  - url: ${REGISTRY_URL}/${POLICY_ID}
    id: ${POLICY_ID}
variables:
  workspace: /tmp/integration-workspace
targets:
  - id: integration-target
    policies:
      - ${POLICY_ID}
    variables:
      env: test
EOF
echo "  Workspace config written."

run_complyctl() {
    (cd "${WORK_DIR}" && "${BINARY}" "$@" 2>&1)
}

# --- Tests ---

echo ""
echo "=== get ==="
OUT="$(run_complyctl get)"
echo "${OUT}"
assert_contains "get: sync completed" "${OUT}" "Synchronization completed."
assert_file_exists "get: oci-layout exists" \
    "${TEST_HOME}/.complytime/policies/policies/test-branch-protection/oci-layout" >/dev/null
assert_file_exists "get: state.json exists" \
    "${TEST_HOME}/.complytime/state.json" >/dev/null

echo ""
echo "=== list ==="
OUT="$(run_complyctl list)"
echo "${OUT}"
assert_contains "list: shows policy" "${OUT}" "test-branch-protection"

echo ""
echo "=== generate ==="
OUT="$(run_complyctl generate --policy-id "${POLICY_ID}")"
echo "${OUT}"
assert_contains "generate: completed" "${OUT}" "Generation completed."

echo ""
echo "=== scan --format oscal ==="
rm -rf "${WORK_DIR}/.complytime/scan"
OUT="$(run_complyctl scan --policy-id "${POLICY_ID}" --format oscal)"
echo "${OUT}"
assert_contains "scan oscal: completed" "${OUT}" "requirements:"

assert_file_exists "scan oscal: evaluation-log exists" \
    "${WORK_DIR}/.complytime/scan/evaluation-log-*.yaml" >/dev/null

OSCAL_FILE="$(assert_file_exists "scan oscal: assessment-results exists" \
    "${WORK_DIR}/.complytime/scan/assessment-results-*.json")"

if [[ -n "${OSCAL_FILE}" ]]; then
    assert_json_eq "scan oscal: oscal-version is 1.1.3" \
        "${OSCAL_FILE}" '.["assessment-results"].metadata["oscal-version"]' "1.1.3"
    assert_json_gte "scan oscal: at least 1 result entry" \
        "${OSCAL_FILE}" '.["assessment-results"].results | length' 1
fi

echo ""
echo "=== scan --format pretty ==="
rm -rf "${WORK_DIR}/.complytime/scan"
OUT="$(run_complyctl scan --policy-id "${POLICY_ID}" --format pretty)"
echo "${OUT}"
assert_contains "scan pretty: completed" "${OUT}" "requirements:"

MD_FILE="$(assert_file_exists "scan pretty: markdown report exists" \
    "${WORK_DIR}/.complytime/scan/report-*.md")"

if [[ -n "${MD_FILE}" ]]; then
    MD_CONTENT="$(cat "${MD_FILE}")"
    assert_contains "scan pretty: has report header" "${MD_CONTENT}" "Compliance Scan Report"
fi

echo ""
echo "=== scan --format sarif ==="
rm -rf "${WORK_DIR}/.complytime/scan"
OUT="$(run_complyctl scan --policy-id "${POLICY_ID}" --format sarif)"
echo "${OUT}"
assert_contains "scan sarif: completed" "${OUT}" "requirements:"

SARIF_FILE="$(assert_file_exists "scan sarif: sarif file exists" \
    "${WORK_DIR}/.complytime/scan/scan-*.sarif.json")"

if [[ -n "${SARIF_FILE}" ]]; then
    assert_json_eq "scan sarif: version is 2.1.0" \
        "${SARIF_FILE}" '.version' "2.1.0"
fi

echo ""
echo "=== scan (default, no --format) ==="
rm -rf "${WORK_DIR}/.complytime/scan"
OUT="$(run_complyctl scan --policy-id "${POLICY_ID}")"
echo "${OUT}"
assert_contains "scan default: completed" "${OUT}" "requirements:"
assert_file_exists "scan default: evaluation-log exists" \
    "${WORK_DIR}/.complytime/scan/evaluation-log-*.yaml" >/dev/null

NO_OSCAL=$(ls "${WORK_DIR}/.complytime/scan/assessment-results-"* 2>/dev/null || true)
NO_MD=$(ls "${WORK_DIR}/.complytime/scan/report-"* 2>/dev/null || true)
NO_SARIF=$(ls "${WORK_DIR}/.complytime/scan/scan-"* 2>/dev/null || true)
if [[ -z "${NO_OSCAL}" && -z "${NO_MD}" && -z "${NO_SARIF}" ]]; then
    pass "scan default: no formatted output without --format"
else
    fail "scan default: formatted output found without --format"
fi

# --- Workspace Configuration Test Helpers ---

# Helper function to create standard test workspace config
create_test_workspace_config() {
    local workspace_dir="$1"
    local target_id="${2:-test-target}"

    mkdir -p "${workspace_dir}/.complytime"
    cat > "${workspace_dir}/.complytime/complytime.yaml" <<EOF
policies:
  - url: ${REGISTRY_URL}/${POLICY_ID}
    id: ${POLICY_ID}
targets:
  - id: ${target_id}
    policies:
      - ${POLICY_ID}
EOF
}

echo ""
echo "=== workspace flag ==="
test_workspace_flag() {
    local workspace_dir
    workspace_dir=$(mktemp -d)
    trap 'rm -rf "$workspace_dir"' RETURN

    create_test_workspace_config "${workspace_dir}"

    local output
    output=$("${BINARY}" list --workspace "${workspace_dir}" 2>&1)
    local exit_code=$?

    if [ $exit_code -eq 0 ]; then
        pass "workspace flag test"
    else
        fail "workspace flag test: exit code ${exit_code}"
        echo "${output}" >&2
    fi
}
test_workspace_flag

echo ""
echo "=== workspace env var ==="
test_workspace_env_var() {
    local workspace_dir
    workspace_dir=$(mktemp -d)
    trap 'rm -rf "$workspace_dir"' RETURN

    create_test_workspace_config "${workspace_dir}"

    local output
    output=$(COMPLYTIME_WORKSPACE="${workspace_dir}" "${BINARY}" list 2>&1)
    local exit_code=$?

    if [ $exit_code -eq 0 ]; then
        pass "workspace env var test"
    else
        fail "workspace env var test: exit code ${exit_code}"
        echo "${output}" >&2
    fi
}
test_workspace_env_var

echo ""
echo "=== workspace flag precedence ==="
test_workspace_precedence() {
    local flag_workspace
    flag_workspace=$(mktemp -d)
    local env_workspace
    env_workspace=$(mktemp -d)
    trap 'rm -rf "$flag_workspace" "$env_workspace"' RETURN

    create_test_workspace_config "${flag_workspace}"
    create_test_workspace_config "${env_workspace}" "invalid-target"

    local output
    output=$(COMPLYTIME_WORKSPACE="${env_workspace}" "${BINARY}" list --workspace "${flag_workspace}" 2>&1)
    local exit_code=$?

    if [ $exit_code -eq 0 ]; then
        pass "workspace precedence test"
    else
        fail "workspace precedence test: exit code ${exit_code}"
        echo "${output}" >&2
    fi
}
test_workspace_precedence

echo ""
echo "=== legacy config warning ==="
test_legacy_config_warning() {
    local workspace_dir
    workspace_dir=$(mktemp -d)
    trap 'rm -rf "$workspace_dir"' RETURN

    cat > "${workspace_dir}/complytime.yaml" <<EOF
policies:
  - url: ${REGISTRY_URL}/${POLICY_ID}
    id: ${POLICY_ID}
targets:
  - id: test-target
    policies:
      - ${POLICY_ID}
EOF

    local output
    output=$(COMPLYTIME_WORKSPACE="${workspace_dir}" "${BINARY}" list 2>&1)
    local exit_code=$?

    if echo "${output}" | grep -q "WARNING.*legacy location"; then
        pass "legacy config warning test"
    else
        fail "legacy config warning test: expected deprecation warning"
        echo "${output}" >&2
    fi
}
test_legacy_config_warning

echo ""
echo "=== new location preferred ==="
test_new_location_preferred() {
    local workspace_dir
    workspace_dir=$(mktemp -d)
    trap 'rm -rf "$workspace_dir"' RETURN

    # Create both locations
    create_test_workspace_config "${workspace_dir}"

    cat > "${workspace_dir}/complytime.yaml" <<EOF
policies:
  - url: ${REGISTRY_URL}/${POLICY_ID}
    id: ${POLICY_ID}
targets:
  - id: legacy-target
    policies:
      - ${POLICY_ID}
EOF

    local output
    output=$(COMPLYTIME_WORKSPACE="${workspace_dir}" "${BINARY}" list 2>&1)
    local exit_code=$?

    # Should NOT show warning when new location exists
    if [ $exit_code -eq 0 ] && ! echo "${output}" | grep -q "WARNING"; then
        pass "new location preferred test"
    else
        fail "new location preferred test: should not show warning"
        echo "${output}" >&2
    fi
}
test_new_location_preferred

echo ""
echo "=== invalid workspace path ==="
test_invalid_workspace_path() {
    local output exit_code
    output=$("${BINARY}" list --workspace /nonexistent/path/that/does/not/exist 2>&1) || exit_code=$?

    if [ "${exit_code:-0}" -ne 0 ] && echo "${output}" | grep -q "workspace directory does not exist"; then
        pass "invalid workspace path test"
    else
        fail "invalid workspace path test: expected error for nonexistent path"
        echo "exit_code=${exit_code:-0}, output=${output}" >&2
    fi
}
test_invalid_workspace_path

echo ""
echo "=== relative workspace path ==="
test_relative_workspace_path() {
    local workspace_dir
    workspace_dir=$(mktemp -d)
    trap 'rm -rf "$workspace_dir"' RETURN

    create_test_workspace_config "${workspace_dir}"

    # cd to parent directory and use relative path
    cd "$(dirname "${workspace_dir}")"
    local rel_path
    rel_path="./$(basename "${workspace_dir}")"

    local output exit_code
    output=$("${BINARY}" list --workspace "${rel_path}" 2>&1)
    exit_code=$?

    cd - > /dev/null

    if [ $exit_code -eq 0 ]; then
        pass "relative workspace path test"
    else
        fail "relative workspace path test: exit code ${exit_code}"
        echo "${output}" >&2
    fi
}
test_relative_workspace_path

echo ""
echo "=== tilde expansion ==="
test_tilde_expansion() {
    # Create test workspace in home directory
    local test_subdir="complytime-tilde-test-$$"
    local workspace_dir="${HOME}/${test_subdir}"
    trap 'rm -rf "$workspace_dir"' RETURN

    create_test_workspace_config "${workspace_dir}"

    # Use tilde path
    local output exit_code
    output=$("${BINARY}" list --workspace ~/"${test_subdir}" 2>&1)
    exit_code=$?

    if [ $exit_code -eq 0 ]; then
        pass "tilde expansion test"
    else
        fail "tilde expansion test: exit code ${exit_code}"
        echo "${output}" >&2
    fi
}
test_tilde_expansion

echo ""
echo "=== scan output directory ==="
test_scan_output_directory() {
    local workspace_dir
    workspace_dir=$(mktemp -d)
    trap 'rm -rf "$workspace_dir"' RETURN

    create_test_workspace_config "${workspace_dir}"

    # Run scan and verify output directory
    local output exit_code
    output=$("${BINARY}" scan --policy-id "${POLICY_ID}" --workspace "${workspace_dir}" 2>&1)
    exit_code=$?

    # Check if scan output was created in correct location
    if [ $exit_code -eq 0 ] && [ -d "${workspace_dir}/.complytime/scan" ]; then
        # Verify evaluation-log was created in scan directory
        if ls "${workspace_dir}/.complytime/scan/evaluation-log-"*.yaml >/dev/null 2>&1; then
            pass "scan output directory test"
        else
            fail "scan output directory test: no evaluation-log in .complytime/scan/"
            echo "${output}" >&2
        fi
    else
        fail "scan output directory test: scan failed or directory not created"
        echo "exit_code=${exit_code}, output=${output}" >&2
    fi
}
test_scan_output_directory

echo ""
echo "=== log file directory ==="
test_log_file_directory() {
    local workspace_dir
    workspace_dir=$(mktemp -d)
    trap 'rm -rf "$workspace_dir"' RETURN

    mkdir -p "${workspace_dir}/.complytime"

    # Create invalid config to trigger an error (which will be logged)
    cat > "${workspace_dir}/.complytime/complytime.yaml" <<EOF
policies: []
EOF

    # Run list command with invalid config (will fail and log error)
    local output
    output=$("${BINARY}" list --workspace "${workspace_dir}" --debug 2>&1) || true

    # Check if log file was created in correct location
    # The lazy log writer creates the file when an error is logged
    if [ -f "${workspace_dir}/.complytime/complyctl.log" ]; then
        pass "log file directory test"
    else
        fail "log file directory test: log file not created in .complytime/"
        echo "output=${output}" >&2
        ls -la "${workspace_dir}/.complytime/" >&2
    fi
}
test_log_file_directory

# --- Summary ---

echo ""
echo "==============================="
echo "  Passed: ${PASSED}"
echo "  Failed: ${FAILED}"
echo "==============================="

if [[ "${FAILED}" -gt 0 ]]; then
    exit 1
fi
