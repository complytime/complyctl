#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Live assertions must run in the parent shell, not inside command substitution.

pass() {
    PASSED=$((PASSED + 1))
    echo "  PASS: $1" >&2
}

fail() {
    FAILED=$((FAILED + 1))
    echo "  FAIL: $1" >&2
}

assert_mapped_ampel_evidence() {
    local output="$1" log records
    if ! log=$(select_evaluation_log "${WORK_DIR}" "${output}" \
        "${AMPEL_REPOSITORY}" "${AMPEL_TARGET}"); then
        fail 'scan: invocation-specific JSON EvaluationLog selection'
        return 1
    fi
    if ! records=$(validate_mapped_evidence "${WORK_DIR}" "${log}" \
        "${AMPEL_REPOSITORY}" "${AMPEL_TARGET}" "${AMPEL_REQUIREMENT}"); then
        fail 'scan: mapped Ampel/Snappy evidence integrity, linkage, and requirement content'
        return 1
    fi
    pass 'scan: invocation-specific JSON EvaluationLog and mapped Ampel/Snappy evidence'
    echo "${records}" | jq -r '.[] | "  Verified \(.role) evidence: \(.path)"'
}
