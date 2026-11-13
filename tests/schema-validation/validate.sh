#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Validate EvaluationLog JSON and YAML files against the Gemara CUE schema.
# Fetches the schema directly from the CUE Central Registry — no local
# module files to maintain.
#
# Usage:
#   ./tests/schema-validation/validate.sh <scan-output-dir> [scan-output-dir ...]
#
# Exit codes:
#   0 — all files pass validation
#   1 — one or more files failed schema validation
#   2 — no evaluation log files found (configuration error)

set -euo pipefail

# --- Prerequisites ---

if ! command -v cue >/dev/null 2>&1; then
    echo "FATAL: 'cue' is required but not installed."
    echo "       Install: go install cuelang.org/go/cmd/cue@v0.17.1"
    echo "       See: https://cuelang.org/docs/install/"
    exit 2
fi

# Gemara CUE schema module — aligned with go-gemara v0.11.0 used by complyctl.
# Tracked by Renovate custom manager (see renovate.json in repo root).
GEMARA_CUE_MODULE="${GEMARA_CUE_MODULE:-github.com/gemaraproj/gemara@v1.6.0}"

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <scan-output-dir> [scan-output-dir ...]"
    exit 2
fi

PASSED=0
FAILED=0
TOTAL=0

validate_file() {
    local file="$1"
    TOTAL=$((TOTAL + 1))
    echo "  Validating: ${file}"

    local output rc=0
    output=$(cue vet -c -d '#EvaluationLog' "${GEMARA_CUE_MODULE}" "${file}" 2>&1) || rc=$?

    if [[ ${rc} -eq 0 ]]; then
        PASSED=$((PASSED + 1))
        echo "    PASS"
    else
        FAILED=$((FAILED + 1))
        if echo "${output}" | grep -qE "cannot find package|module not found|connection refused|dial tcp|no such host"; then
            echo "    FAIL: tool/infrastructure error (exit code ${rc})"
        else
            echo "    FAIL: schema constraint violated (exit code ${rc})"
        fi
        echo "    ---"
        echo "    ${output//$'\n'/$'\n'    }"
        echo "    ---"
    fi
}

echo "=== EvaluationLog Schema Validation ==="
echo "  Module: ${GEMARA_CUE_MODULE}"
echo ""

FILES_FOUND=0
for dir in "$@"; do
    if [[ ! -d "${dir}" ]]; then
        echo "WARNING: directory does not exist: ${dir}"
        continue
    fi

    while IFS= read -r -d '' file; do
        FILES_FOUND=$((FILES_FOUND + 1))
        validate_file "${file}"
    done < <(find "${dir}" -type f \
        \( -name 'evaluation-log-*.json' -o -name 'evaluation-log-*.yaml' \) -print0 2>/dev/null)
done

echo ""

if [[ ${FILES_FOUND} -eq 0 ]]; then
    echo "ERROR: No evaluation-log-*.json or evaluation-log-*.yaml files found in: $*"
    echo "       Ensure complyctl scan completed successfully before validation."
    exit 2
fi

echo "==============================="
echo "  Total:  ${TOTAL}"
echo "  Passed: ${PASSED}"
echo "  Failed: ${FAILED}"
echo "==============================="

if [[ ${FAILED} -gt 0 ]]; then
    exit 1
fi
