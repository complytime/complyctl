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
# Optional environment variables:
#   COMPLYCTL_BIN_DIR   Absolute directory containing both prebuilt host binaries;
#                       defaults to this harness checkout's bin/ directory
#   SCAN_OUTPUT_DIR     Directory to export evaluation-log-*.json and *.yaml for downstream
#                       schema validation (set by CI, not required for local runs)
#   COMPLYCTL_CROSS_REPO_INDEX_FIXTURE_TEST
#                       Test-only: 1 runs index fixtures and skips the live pipeline
#
# Run locally:  make test-cross-repo PROVIDERS_BIN_DIR=/path/to/providers/bin
# Run directly: PROVIDERS_BIN_DIR=... GITHUB_TOKEN=... ./tests/cross-repo/cross_repo_integration_test.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=tests/cross-repo/evidence_helpers.sh
source "${REPO_ROOT}/tests/cross-repo/evidence_helpers.sh"
# shellcheck source=tests/cross-repo/evidence_assertions.sh
source "${REPO_ROOT}/tests/cross-repo/evidence_assertions.sh"
# shellcheck source=tests/cross-repo/harness_helpers.sh
source "${REPO_ROOT}/tests/cross-repo/harness_helpers.sh"

REGISTRY_PORT="${GEMARA_SERVICE_PORT:-8765}"
REGISTRY_URL="http://localhost:${REGISTRY_PORT}"
POLICY_ID="test-ampel-bp"
AMPEL_REPOSITORY="policies/test-branch-protection"
AMPEL_TARGET="complyctl-repo"
AMPEL_REQUIREMENT="block-force-push"
OPA_POLICY_ID="test-opa-k8s"
AMPEL_TARGET_URL="https://github.com/complytime/complyctl"

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
_GITHUB_TOKEN="${GITHUB_TOKEN:-}"
unset GITHUB_TOKEN

cleanup() {
    if [[ -n "${REGISTRY_PID}" ]]; then
        kill "${REGISTRY_PID}" 2>/dev/null || true
        wait "${REGISTRY_PID}" 2>/dev/null || true
    fi
    if [[ -n "${WORK_DIR}" ]]; then
        rm -rf "${WORK_DIR}"
    fi
    if [[ -n "${TEST_HOME}" ]]; then
        rm -rf "${TEST_HOME}"
    fi
}
trap cleanup EXIT

# --- Assertion helpers ---

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

resolve_indexed_ampel_bundle() {
    local generated_root="$1" index_path="$2" target_key content_key
    local resolved_root resolved_bundle bundle_path

    # The provider hashes a normalized URL target key before recording it in the index.
    target_key=$(printf 'url:%s' "${AMPEL_TARGET_URL#*://}" | sha256sum | cut -c1-16)
    content_key=$(jq -er --arg target_key "${target_key}" \
        'if .version == 1 and (.targets | type == "object")
         then .targets[$target_key] // error("missing target entry")
         else error("invalid index schema")
         end' \
        "${index_path}") || {
        fail "generate: index has no bundle for complyctl-repo"
        return 1
    }

    resolved_root=$(realpath -m "${generated_root}")
    bundle_path="${generated_root}/${content_key}/complytime-ampel-policy.json"
    resolved_bundle=$(realpath -m "${bundle_path}")
    if [[ "${resolved_bundle}" != "${resolved_root}/"* ]]; then
        fail "generate: indexed bundle escapes generated-artifact root"
        return 1
    fi
    if [[ ! "${content_key}" =~ ^[0-9a-f]{16}$ ]]; then
        fail "generate: index has an invalid content key for complyctl-repo"
        return 1
    fi

    FOUND_FILE="${resolved_bundle}"
}

assert_ampel_policy_bundle() {
    local generated_root="$1"
    local index_path="${generated_root}/generate-index.json"

    if [[ -e "${index_path}" ]]; then
        resolve_indexed_ampel_bundle "${generated_root}" "${index_path}" || return 1
        assert_file_exists "generate: indexed policy bundle exists" "${FOUND_FILE}"
    else
        assert_file_exists "generate: policy bundle exists" \
            "${WORK_DIR}/.complytime/ampel/policy/complytime-ampel-policy.json"
    fi

    [[ -n "${FOUND_FILE}" ]]
}

test_indexed_ampel_bundle_failures() {
    local fixture_root generated_root legacy_bundle target_key failure_count
    local missing_bundle_key empty_bundle_key

    echo ""
    echo "=== test_indexed_ampel_bundle_failures ==="
    fixture_root="$(mktemp -d)"
    generated_root="${fixture_root}/generated"
    legacy_bundle="${fixture_root}/policy/complytime-ampel-policy.json"
    mkdir -p "${generated_root}" "$(dirname "${legacy_bundle}")"
    printf '{"policies":[{"id":"block-force-push"}]}' > "${legacy_bundle}"
    target_key=$(printf 'url:%s' "${AMPEL_TARGET_URL#*://}" | sha256sum | cut -c1-16)

    printf '{"version":1,"targets":{}}' > "${generated_root}/generate-index.json"
    failure_count=${FAILED}
    FOUND_FILE=""
    if assert_ampel_policy_bundle "${generated_root}" 2>/dev/null; then
        fail "generate fixture: missing target entry must fail"
    elif [[ "${FAILED}" -eq $((failure_count + 1)) && -z "${FOUND_FILE}" ]]; then
        FAILED=${failure_count}
        pass "generate fixture: missing target entry does not use legacy bundle"
    else
        fail "generate fixture: missing target entry did not fail closed"
    fi

    missing_bundle_key="0123456789abcdef"
    printf '{"version":1,"targets":{"%s":"%s"}}' "${target_key}" "${missing_bundle_key}" \
        > "${generated_root}/generate-index.json"
    failure_count=${FAILED}
    FOUND_FILE=""
    if assert_ampel_policy_bundle "${generated_root}" 2>/dev/null; then
        fail "generate fixture: missing indexed bundle must fail"
    elif [[ "${FAILED}" -eq $((failure_count + 1)) && -z "${FOUND_FILE}" ]]; then
        FAILED=${failure_count}
        pass "generate fixture: missing indexed bundle does not use legacy bundle"
    else
        fail "generate fixture: missing indexed bundle did not fail closed"
    fi

    empty_bundle_key="fedcba9876543210"
    mkdir -p "${generated_root}/${empty_bundle_key}"
    : > "${generated_root}/${empty_bundle_key}/complytime-ampel-policy.json"
    printf '{"version":1,"targets":{"%s":"%s"}}' "${target_key}" "${empty_bundle_key}" \
        > "${generated_root}/generate-index.json"
    failure_count=${FAILED}
    FOUND_FILE=""
    if assert_ampel_policy_bundle "${generated_root}" 2>/dev/null; then
        fail "generate fixture: empty indexed bundle must fail"
    elif [[ "${FAILED}" -eq $((failure_count + 1)) && -z "${FOUND_FILE}" ]]; then
        FAILED=${failure_count}
        pass "generate fixture: empty indexed bundle does not use legacy bundle"
    else
        fail "generate fixture: empty indexed bundle did not fail closed"
    fi

    rm -rf "${fixture_root}"
}

# Sanitize output to prevent GITHUB_TOKEN from appearing in CI logs.
sanitize_output() {
    if [[ -n "${_GITHUB_TOKEN:-}" ]]; then
        sed "s/${_GITHUB_TOKEN}/[REDACTED]/g"
    else
        cat
    fi
}

if [[ "${COMPLYCTL_CROSS_REPO_INDEX_FIXTURE_TEST:-}" == "1" ]]; then
    echo 'FIXTURE MODE: generation-index fixtures only; live pipeline is not executed.'
    test_indexed_ampel_bundle_failures
    [[ "${FAILED}" -eq 0 ]]
    exit
fi

# --- Prerequisites ---

echo "=== Prerequisites ==="

select_host_binaries "${REPO_ROOT}"

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

for cmd in jq curl realpath sha256sum; do
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

# Install provider binaries into the isolated home (XDG data directory)
PROVIDER_DIR="${TEST_HOME}/.local/share/complytime/providers"
CACHE_DIR="${TEST_HOME}/.cache/complytime"
mkdir -p "${PROVIDER_DIR}"
mkdir -p "${CACHE_DIR}"
cp "${PROVIDER_BINARY}" "${PROVIDER_DIR}/"
cp "${OPA_PROVIDER_BINARY}" "${PROVIDER_DIR}/"
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
        "${TEST_HOME}/.cache/complytime/policies/policies/test-branch-protection/oci-layout"
    assert_file_exists "get: state.json exists" \
        "${TEST_HOME}/.local/share/complytime/state.json"
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
    local generated_root="${WORK_DIR}/.complytime/ampel/generated"
    assert_ampel_policy_bundle "${generated_root}" || return
    assert_json_contains "generate: bundle contains block-force-push policy" \
        "${FOUND_FILE}" '.policies[].id' "block-force-push"
}

# --- test_scan ---

test_scan() {
    FOUND_FILE=""
    echo ""
    echo "=== test_scan ==="
    local out rc=0
    # complyctl scan exits 0 on tool-level success (even if policy controls FAIL).
    # A non-zero exit indicates a tool-level error (gRPC failure, binary not found, etc.).
    out="$(run_complyctl scan --policy-id "${POLICY_ID}" --log-format json)" || rc=$?
    if [[ "${rc}" -ne 0 ]]; then
        fail "scan: unexpected exit code ${rc}"
        echo "${out}" | sanitize_output >&2
        return
    fi
    echo "${out}" | sanitize_output
    assert_contains "scan: completed" "${out}" "requirements:"

    # The assertion records FAILED; keep running unrelated live checks and export.
    assert_mapped_ampel_evidence "${out}" || return 0
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
        "${TEST_HOME}/.cache/complytime/policies/policies/test-opa-policy/oci-layout"

    # Complypack cache uses evaluator-id/version/ structure.
    # Find any content.tar.gz under the opa evaluator directory.
    local complypack_match
    complypack_match=$(find "${TEST_HOME}/.cache/complytime/complypacks/opa/" \
        -name "content.tar.gz" -print -quit 2>/dev/null) || true
    if [[ -n "${complypack_match}" && -s "${complypack_match}" ]]; then
        pass "get opa: complypack cached"
    else
        fail "get opa: complypack cached: no content.tar.gz under .cache/complytime/complypacks/opa/"
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

# --- Export scan output for schema validation (CI only) ---

if [[ -n "${SCAN_OUTPUT_DIR:-}" ]]; then
    export_evaluation_logs "${WORK_DIR}/${EVIDENCE_SCAN_SUBDIR}" "${SCAN_OUTPUT_DIR}"
fi

if [[ "${FAILED}" -gt 0 ]]; then
    exit 1
fi
