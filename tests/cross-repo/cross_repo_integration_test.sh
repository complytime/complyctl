#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Cross-repo integration test: complyctl + complytime-providers (ampel + opa).
# Validates the full get → generate → scan pipeline using real binaries.
# Ampel tests use the real GitHub API via snappy and ampel.
# OPA tests use a local K8s Deployment fixture evaluated by conftest.
#
# Required environment variables:
#   PROVIDERS_BIN_DIR   Directory containing complyctl-provider-ampel and complyctl-provider-opa
#   GITHUB_TOKEN        GitHub token with read access to public repositories
#
# Run locally:  make test-cross-repo PROVIDERS_BIN_DIR=/path/to/providers/bin
# Run directly: PROVIDERS_BIN_DIR=... GITHUB_TOKEN=... ./tests/cross-repo/cross_repo_integration_test.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BINARY="${REPO_ROOT}/bin/complyctl"
MOCK_REGISTRY="${REPO_ROOT}/bin/mock-oci-registry"
TESTDATA_DIR="${REPO_ROOT}/tests/cross-repo/testdata"

REGISTRY_PORT="${GEMARA_SERVICE_PORT:-8765}"
REGISTRY_URL="http://localhost:${REGISTRY_PORT}"
POLICY_ID="test-ampel-bp"
OPA_POLICY_ID="test-opa-k8s"

WORK_DIR=""
TEST_HOME=""
REGISTRY_PID=""
PASSED=0
FAILED=0

# FOUND_FILE is set by assert_file_exists to return a matched path without
# using a subshell capture ($()). This avoids the counter-loss bug present in
# integration_test.sh where PASSED/FAILED increments inside $() are discarded.
FOUND_FILE=""

# Capture and unset GITHUB_TOKEN so only run_complyctl inherits it.
# This follows least-privilege for environment inheritance.
# shellcheck disable=SC2153 # GITHUB_TOKEN is set externally, not a misspelling of _GITHUB_TOKEN
_GITHUB_TOKEN="${GITHUB_TOKEN}"
unset GITHUB_TOKEN

cleanup() {
    if [[ -n "${REGISTRY_PID}" ]]; then
        kill "${REGISTRY_PID}" 2>/dev/null || true
        wait "${REGISTRY_PID}" 2>/dev/null || true
    fi
    [[ -n "${WORK_DIR}" ]] && rm -rf "${WORK_DIR}"
    [[ -n "${TEST_HOME}" ]] && rm -rf "${TEST_HOME}"
}
trap cleanup EXIT

# --- Assertion helpers ---

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
    if echo "${haystack}" | grep -qF "${needle}"; then
        pass "${label}"
    else
        fail "${label}: expected output to contain '${needle}'"
        echo "  --- actual output ---" >&2
        echo "${haystack}" | head -20 >&2
        echo "  ---" >&2
    fi
}

# Sets FOUND_FILE to the matched path (or empty) and records pass/fail.
# Call directly — do NOT capture in $() or counter updates are lost.
assert_file_exists() {
    local label="$1" pattern="$2"
    local match dir base
    dir="$(dirname "${pattern}")"
    base="$(basename "${pattern}")"
    match=$(find "${dir}" -maxdepth 1 -name "${base}" -print -quit 2>/dev/null) || true
    if [[ -n "${match}" && -s "${match}" ]]; then
        pass "${label}"
        FOUND_FILE="${match}"
    else
        fail "${label}: no non-empty file matching ${pattern}"
        FOUND_FILE=""
    fi
}

assert_json_contains() {
    local label="$1" file="$2" query="$3" expected="$4"
    local actual
    actual=$(jq -r "${query}" "${file}" 2>/dev/null) || true
    if echo "${actual}" | grep -qF "${expected}"; then
        pass "${label}"
    else
        fail "${label}: expected jq output to contain '${expected}', got '${actual}'"
    fi
}

# Sanitize output to prevent GITHUB_TOKEN from appearing in CI logs.
sanitize_output() {
    if [[ -n "${_GITHUB_TOKEN:-}" ]]; then
        sed "s/${_GITHUB_TOKEN}/[REDACTED]/g"
    else
        cat
    fi
}

# --- Prerequisites ---

echo "=== Prerequisites ==="

# Validate required environment variables
if [[ -z "${PROVIDERS_BIN_DIR:-}" ]]; then
    echo "FATAL: PROVIDERS_BIN_DIR is not set. Set it to the directory containing complyctl-provider-ampel."
    exit 1
fi

if [[ -z "${_GITHUB_TOKEN:-}" ]]; then
    echo "FATAL: GITHUB_TOKEN is not set. A GitHub token is required for snappy to read branch protection rules."
    exit 1
fi

PROVIDER_BINARY="${PROVIDERS_BIN_DIR}/complyctl-provider-ampel"
if [[ ! -x "${PROVIDER_BINARY}" ]]; then
    echo "FATAL: complyctl-provider-ampel not found or not executable at ${PROVIDER_BINARY}"
    echo "       Build complytime-providers first and set PROVIDERS_BIN_DIR to its bin/ directory."
    exit 1
fi

OPA_PROVIDER_BINARY="${PROVIDERS_BIN_DIR}/complyctl-provider-opa"
if [[ ! -x "${OPA_PROVIDER_BINARY}" ]]; then
    echo "FATAL: complyctl-provider-opa not found or not executable at ${OPA_PROVIDER_BINARY}"
    echo "       Build complytime-providers first and set PROVIDERS_BIN_DIR to its bin/ directory."
    exit 1
fi

if ! command -v conftest >/dev/null 2>&1; then
    echo "FATAL: 'conftest' is required but not installed. The OPA provider requires conftest."
    exit 1
fi

if [[ ! -x "${BINARY}" ]]; then
    echo "FATAL: complyctl binary not found at ${BINARY}. Run 'make build' first."
    exit 1
fi

if [[ ! -x "${MOCK_REGISTRY}" ]]; then
    echo "FATAL: mock-oci-registry binary not found at ${MOCK_REGISTRY}. Run 'make build' first."
    exit 1
fi

for cmd in jq curl; do
    if ! command -v "${cmd}" >/dev/null 2>&1; then
        echo "FATAL: '${cmd}' is required but not installed."
        exit 1
    fi
done

echo "  All prerequisites met."

# --- Setup ---

echo ""
echo "=== Setup ==="

TEST_HOME="$(mktemp -d)"
WORK_DIR="$(mktemp -d)"
export HOME="${TEST_HOME}"

# Install provider binaries into the isolated home
mkdir -p "${TEST_HOME}/.complytime/providers"
cp "${PROVIDER_BINARY}" "${TEST_HOME}/.complytime/providers/"
cp "${OPA_PROVIDER_BINARY}" "${TEST_HOME}/.complytime/providers/"
echo "  HOME=${TEST_HOME}"
echo "  WORK=${WORK_DIR}"

# Generate workspace config with the correct registry port.
# The static testdata/complytime.yaml uses port 8765 as default; sed replaces it
# so the GEMARA_SERVICE_PORT override works end-to-end.
mkdir -p "${WORK_DIR}/.complytime/ampel/granular-policies"

sed "s|http://localhost:8765|${REGISTRY_URL}|" \
    "${TESTDATA_DIR}/complytime.yaml" > "${WORK_DIR}/.complytime/complytime.yaml"
cp "${TESTDATA_DIR}/granular-policies/block-force-push.json" \
    "${WORK_DIR}/.complytime/ampel/granular-policies/"

# Copy OPA test fixture into the workspace.
# Start with the non-compliant (bad) fixture; the compliant test swaps it.
cp "${TESTDATA_DIR}/test-deployment-bad.yaml" "${WORK_DIR}/test-deployment.yaml"
echo "  Workspace config, granular policy, and OPA test fixture copied."

# Start mock registry.
# 30 retries (15s) — longer than integration_test.sh (15 retries / 7.5s) because
# the cross-repo CI builds two repos before starting the registry, and runner
# resource contention can delay process startup.
GEMARA_SERVICE_PORT="${REGISTRY_PORT}" "${MOCK_REGISTRY}" &
REGISTRY_PID=$!

for _ in $(seq 1 30); do
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

# Only complyctl subprocesses receive GITHUB_TOKEN (least-privilege).
run_complyctl() {
    (cd "${WORK_DIR}" && GITHUB_TOKEN="${_GITHUB_TOKEN}" "${BINARY}" "$@" 2>&1)
}

# --- test_get ---

test_get() {
    FOUND_FILE=""
    echo ""
    echo "=== test_get ==="
    local out rc=0
    out="$(run_complyctl get)" || rc=$?
    if [[ "${rc}" -ne 0 ]]; then
        fail "get: unexpected exit code ${rc}"
        echo "${out}" | sanitize_output >&2
        return
    fi
    echo "${out}" | sanitize_output
    assert_contains "get: sync completed" "${out}" "Synchronization completed."
    assert_file_exists "get: oci-layout exists" \
        "${TEST_HOME}/.complytime/policies/policies/test-branch-protection/oci-layout"
    assert_file_exists "get: state.json exists" \
        "${TEST_HOME}/.complytime/state.json"
}

# --- test_generate ---

test_generate() {
    FOUND_FILE=""
    echo ""
    echo "=== test_generate ==="
    local out rc=0
    out="$(run_complyctl generate --policy-id "${POLICY_ID}")" || rc=$?
    if [[ "${rc}" -ne 0 ]]; then
        fail "generate: unexpected exit code ${rc}"
        echo "${out}" | sanitize_output >&2
        return
    fi
    echo "${out}" | sanitize_output
    assert_contains "generate: completed" "${out}" "Generation completed."
    assert_file_exists "generate: policy bundle exists" \
        "${WORK_DIR}/.complytime/ampel/policy/complytime-ampel-policy.json"
    if [[ -n "${FOUND_FILE}" ]]; then
        assert_json_contains "generate: bundle contains block-force-push policy" \
            "${FOUND_FILE}" '.policies[].id' "block-force-push"
    fi
}

# --- test_scan ---

test_scan() {
    FOUND_FILE=""
    echo ""
    echo "=== test_scan ==="
    local out rc=0
    # complyctl scan exits 0 on tool-level success (even if policy controls FAIL).
    # A non-zero exit indicates a tool-level error (gRPC failure, binary not found, etc.).
    out="$(run_complyctl scan --policy-id "${POLICY_ID}")" || rc=$?
    if [[ "${rc}" -ne 0 ]]; then
        fail "scan: unexpected exit code ${rc}"
        echo "${out}" | sanitize_output >&2
        return
    fi
    echo "${out}" | sanitize_output
    assert_contains "scan: completed" "${out}" "requirements:"

    assert_file_exists "scan: snappy attestation exists" \
        "${WORK_DIR}/.complytime/ampel/results/*-snappy.intoto.json"

    assert_file_exists "scan: ampel attestation exists" \
        "${WORK_DIR}/.complytime/ampel/results/*-ampel.intoto.json"
    local ampel_file="${FOUND_FILE}"

    if [[ -n "${ampel_file}" ]]; then
        assert_json_contains "scan: ampel result contains block-force-push requirement ID" \
            "${ampel_file}" '.predicate.results[].policy.id' "block-force-push"
    fi
}

# --- test_generate_bad_policy (negative test) ---

test_generate_bad_policy() {
    FOUND_FILE=""
    echo ""
    echo "=== test_generate_bad_policy ==="
    local out rc=0
    out="$(run_complyctl generate --policy-id nonexistent-policy 2>&1)" || rc=$?
    if [[ "${rc}" -ne 0 ]]; then
        pass "generate bad policy: non-zero exit code"
    else
        fail "generate bad policy: expected non-zero exit code, got 0"
    fi
    assert_contains "generate bad policy: error message" "${out}" "not found"
}

# --- test_get_opa ---

test_get_opa() {
    FOUND_FILE=""
    echo ""
    echo "=== test_get_opa ==="
    # test_get already ran complyctl get which pulls all policies and complypacks.
    # Verify OPA-specific artifacts were fetched.
    assert_file_exists "get opa: oci-layout exists" \
        "${TEST_HOME}/.complytime/policies/policies/test-opa-policy/oci-layout"

    # Complypack cache uses evaluator-id/version/ structure.
    # Find any content.tar.gz under the opa evaluator directory.
    local complypack_match
    complypack_match=$(find "${TEST_HOME}/.complytime/complypacks/opa/" \
        -name "content.tar.gz" -print -quit 2>/dev/null) || true
    if [[ -n "${complypack_match}" && -s "${complypack_match}" ]]; then
        pass "get opa: complypack cached"
    else
        fail "get opa: complypack cached: no content.tar.gz under complypacks/opa/"
    fi
}

# --- test_generate_opa ---

test_generate_opa() {
    FOUND_FILE=""
    echo ""
    echo "=== test_generate_opa ==="
    local out rc=0
    out="$(run_complyctl generate --policy-id "${OPA_POLICY_ID}")" || rc=$?
    if [[ "${rc}" -ne 0 ]]; then
        fail "generate opa: unexpected exit code ${rc}"
        echo "${out}" | sanitize_output >&2
        return
    fi
    echo "${out}" | sanitize_output
    assert_contains "generate opa: completed" "${out}" "Generation completed."
}

# --- test_scan_opa (non-compliant fixture — expects failures) ---

test_scan_opa() {
    FOUND_FILE=""
    echo ""
    echo "=== test_scan_opa ==="
    local out rc=0
    out="$(run_complyctl scan --policy-id "${OPA_POLICY_ID}")" || rc=$?
    if [[ "${rc}" -ne 0 ]]; then
        fail "scan opa: unexpected exit code ${rc}"
        echo "${out}" | sanitize_output >&2
        return
    fi
    echo "${out}" | sanitize_output
    assert_contains "scan opa: completed" "${out}" "requirements:"
    assert_contains "scan opa: check-run-as-nonroot requirement" "${out}" "check-run-as-nonroot"
    assert_contains "scan opa: check-resource-limits requirement" "${out}" "check-resource-limits"
    assert_contains "scan opa: failures detected" "${out}" "failed"
}

# --- test_scan_opa_compliant (compliant fixture — expects all pass) ---

test_scan_opa_compliant() {
    FOUND_FILE=""
    echo ""
    echo "=== test_scan_opa_compliant ==="
    # Swap in the compliant (good) deployment fixture.
    cp "${TESTDATA_DIR}/test-deployment-good.yaml" "${WORK_DIR}/test-deployment.yaml"

    local out rc=0
    # Re-generate to pick up fresh state, then scan.
    out="$(run_complyctl generate --policy-id "${OPA_POLICY_ID}")" || rc=$?
    if [[ "${rc}" -ne 0 ]]; then
        fail "scan opa compliant: generate failed with exit code ${rc}"
        echo "${out}" | sanitize_output >&2
        return
    fi
    rc=0
    out="$(run_complyctl scan --policy-id "${OPA_POLICY_ID}")" || rc=$?
    if [[ "${rc}" -ne 0 ]]; then
        fail "scan opa compliant: unexpected exit code ${rc}"
        echo "${out}" | sanitize_output >&2
        return
    fi
    echo "${out}" | sanitize_output
    assert_contains "scan opa compliant: completed" "${out}" "requirements:"
    assert_contains "scan opa compliant: all passed" "${out}" "passed"

    # Restore the non-compliant (bad) fixture for any subsequent tests.
    cp "${TESTDATA_DIR}/test-deployment-bad.yaml" "${WORK_DIR}/test-deployment.yaml"
}

# --- Run all tests ---

test_get
test_generate
test_scan
test_generate_bad_policy
test_get_opa
test_generate_opa
test_scan_opa
test_scan_opa_compliant

# --- Summary ---

echo ""
echo "==============================="
echo "  Passed: ${PASSED}"
echo "  Failed: ${FAILED}"
echo "==============================="

if [[ "${FAILED}" -gt 0 ]]; then
    exit 1
fi
