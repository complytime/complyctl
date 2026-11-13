#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Exercise the real pinned CUE validator, not a stand-in schema or parser.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FIXTURES=$(mktemp -d)
trap 'rm -rf "${FIXTURES}"' EXIT
for tool in cue jq; do
    command -v "${tool}" >/dev/null || { echo "Missing prerequisite: ${tool}" >&2; exit 1; }
done
mkdir -p "${FIXTURES}/json" "${FIXTURES}/mixed" "${FIXTURES}/invalid-json" \
    "${FIXTURES}/invalid-yaml" "${FIXTURES}/empty"
cat > "${FIXTURES}/valid.yaml" <<'YAML'
metadata:
  id: policies/test-branch-protection
  type: EvaluationLog
  gemara-version: 1.6.0
  description: Schema regression fixture
  author:
    id: complytime
    name: complytime
    type: Software
target:
  id: complyctl-repo
  name: complyctl-repo
  type: Software
result: Passed
evaluations:
  - name: force-push-restriction
    result: Passed
    message: Force push blocked
    control:
      reference-id: policies/test-branch-protection
      entry-id: force-push-restriction
    assessment-logs:
      - requirement:
          reference-id: policies/test-branch-protection
          entry-id: block-force-push
        description: Force push check
        result: Passed
        message: Force push blocked
        applicability: [default]
        steps: [check]
        start: "2026-11-13T09:00:00Z"
YAML
# CUE supplies the already-required parser; no extra YAML dependency is needed.
cue export "${FIXTURES}/valid.yaml" > "${FIXTURES}/json/evaluation-log-ampel.json"
cp "${FIXTURES}/valid.yaml" "${FIXTURES}/mixed/evaluation-log-opa.yaml"
cp "${FIXTURES}/json/evaluation-log-ampel.json" "${FIXTURES}/mixed/"
jq '.result = "InvalidResult"' "${FIXTURES}/json/evaluation-log-ampel.json" \
    > "${FIXTURES}/invalid-json/evaluation-log-invalid.json"
sed 's/result: Passed/result: InvalidResult/g' "${FIXTURES}/valid.yaml" \
    > "${FIXTURES}/invalid-yaml/evaluation-log-invalid.yaml"

check_validation() {
    local label="$1" directory="$2" expected_rc="$3" expected_output="$4" output rc=0
    output=$(bash "${SCRIPT_DIR}/validate.sh" "${directory}" 2>&1) || rc=$?
    if [[ "${rc}" -ne "${expected_rc}" || "${output}" != *"${expected_output}"* ]]; then
        echo "FAIL: ${label}: expected exit ${expected_rc} and '${expected_output}'" >&2
        echo "${output}" >&2
        return 1
    fi
    echo "PASS: ${label}"
}
check_validation 'JSON-only actual CUE validation' "${FIXTURES}/json" 0 'Total:  1'
check_validation 'mixed JSON/YAML actual CUE validation' "${FIXTURES}/mixed" 0 'Total:  2'
check_validation 'CUE rejects invalid JSON log' "${FIXTURES}/invalid-json" 1 'schema constraint violated'
check_validation 'CUE rejects invalid YAML log' "${FIXTURES}/invalid-yaml" 1 'schema constraint violated'
check_validation 'absent logs remain a configuration failure' "${FIXTURES}/empty" 2 'No evaluation-log-'
printf '{' > "${FIXTURES}/invalid-json/evaluation-log-invalid.json"
check_validation 'CUE rejects malformed JSON' "${FIXTURES}/invalid-json" 1 'Failed: 1'
printf 'metadata: [' > "${FIXTURES}/invalid-yaml/evaluation-log-invalid.yaml"
check_validation 'CUE rejects malformed YAML' "${FIXTURES}/invalid-yaml" 1 'Failed: 1'
