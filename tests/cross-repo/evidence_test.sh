#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Deterministic evidence regressions: no token, providers, or network required.
# jq programs passed through mutate_log intentionally use jq variable expansion.
# shellcheck disable=SC2016
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=tests/cross-repo/evidence_helpers.sh
source "${SCRIPT_DIR}/evidence_helpers.sh"
# shellcheck source=tests/cross-repo/evidence_assertions.sh
source "${SCRIPT_DIR}/evidence_assertions.sh"
for tool in jq realpath sha256sum; do
    command -v "${tool}" >/dev/null || { echo "Missing prerequisite: ${tool}" >&2; exit 1; }
done
FIXTURES=$(mktemp -d)
trap 'rm -rf "${FIXTURES}"' EXIT
PASSED=0
FAILED=0
SKIPPED=0
POLICY='policies/test-branch-protection'
TARGET='complyctl-repo'
REQUIREMENT='block-force-push'
WORKSPACE="${FIXTURES}/workspace with spaces % café"
mkdir -p "${WORKSPACE}/${EVIDENCE_SCAN_SUBDIR}"

expect_success() {
    local label="$1" output
    shift
    if output=$("$@" 2>&1); then
        PASSED=$((PASSED + 1))
        echo "PASS: ${label}"
    else
        FAILED=$((FAILED + 1))
        echo "FAIL: ${label}: ${output}" >&2
    fi
}

expect_failure() {
    local label="$1" diagnostic="$2" output
    shift 2
    if output=$("$@" 2>&1); then
        FAILED=$((FAILED + 1))
        echo "FAIL: ${label}: unexpectedly succeeded" >&2
    elif [[ "${output}" == *"${diagnostic}"* ]]; then
        PASSED=$((PASSED + 1))
        echo "PASS: ${label}"
    else
        FAILED=$((FAILED + 1))
        echo "FAIL: ${label}: wrong diagnostic: ${output}" >&2
    fi
}

# Each fixture publishes a provider-shaped log plus two mapped artifacts.
# Different tokens model evaluation identity, never a required naming layout.
publish_fixture() {
    local token="$1" layout="$2" role path coordinate digest url evidence='[]' references='[]'
    for role in ampel snappy; do
        if [[ "${layout}" == legacy ]]; then
            coordinate=".complytime/ampel/results/${token}-${role}.intoto.json"
        else
            coordinate=".complytime/ampel/evaluations/${token}/arbitrary ${role} % café.json"
        fi
        path="${WORKSPACE}/${coordinate}"
        mkdir -p "$(dirname "${path}")"
        if [[ "${role}" == ampel ]]; then
            printf '{"predicate":{"results":[{"policy":{"id":"%s"}}]}}\n' \
                "${REQUIREMENT}" > "${path}"
        else
            printf '{"predicate":{"observed":true}}\n' > "${path}"
        fi
        digest=$(sha256sum "${path}")
        digest="sha256:${digest:0:64}"
        url=$(jq -rn --arg path "${path}" '"file://" + ($path | split("/") | map(@uri) | join("/"))')
        evidence=$(jq -cn --argjson entries "${evidence}" --arg id "${role}_${token}" \
            --arg coordinate "${coordinate}" --arg digest "${digest}" \
            '$entries + [{id:$id, type:"IntotoAttestation", source:{"reference-id":$id,
                coordinate:$coordinate, digest:$digest}}]')
        references=$(jq -cn --argjson entries "${references}" --arg id "${role}_${token}" \
            --arg url "${url}" '$entries + [{id:$id, url:$url}]')
    done
    jq -n --arg policy "${POLICY}" --arg target "${TARGET}" --arg requirement "${REQUIREMENT}" \
        --argjson evidence "${evidence}" --argjson references "${references}" '
        {metadata:{id:$policy, "mapping-references":$references}, target:{id:$target},
        evaluations:[{"assessment-logs":[{requirement:{"entry-id":$requirement,
            "reference-id":$policy}, evidence:$evidence}]}]}
    ' > "${WORKSPACE}/${EVIDENCE_SCAN_SUBDIR}/evaluation-log-${token}.json"
}

check_log() {
    validate_mapped_evidence "${WORKSPACE}" "$1" "${POLICY}" "${TARGET}" "${REQUIREMENT}"
}

select_log() {
    select_evaluation_log "${WORKSPACE}" "$1" "${POLICY}" "${TARGET}"
}

mutate_log() {
    jq "$@" "${ORIGINAL}" > "${MUTATED}"
}

publish_fixture legacy legacy
ORIGINAL="${WORKSPACE}/${EVIDENCE_SCAN_SUBDIR}/evaluation-log-legacy.json"
MUTATED="${WORKSPACE}/${EVIDENCE_SCAN_SUBDIR}/evaluation-log-mutated.json"
REPORT="Evaluation log written: ${ORIGINAL} [target: ${TARGET}]"
expect_success 'legacy mapped files' check_log "${ORIGINAL}"
records=$(check_log "${ORIGINAL}")
jq -e --arg root "${WORKSPACE}" '
    length == 2 and any(.[]; .id == "ampel_legacy" and .reference == "ampel_legacy" and
        .path == ($root + "/.complytime/ampel/results/legacy-ampel.intoto.json")) and
    any(.[]; .id == "snappy_legacy" and .reference == "snappy_legacy")
' <<< "${records}" >/dev/null
WORK_DIR="${WORKSPACE}"
AMPEL_REPOSITORY="${POLICY}"
AMPEL_TARGET="${TARGET}"
AMPEL_REQUIREMENT="${REQUIREMENT}"
before_passed=${PASSED}
before_failed=${FAILED}
assert_mapped_ampel_evidence "${REPORT}" >/dev/null
[[ "${PASSED}" -eq $((before_passed + 1)) && "${FAILED}" -eq "${before_failed}" ]]
if assert_mapped_ampel_evidence 'No reported log' 2>/dev/null; then
    echo 'Expected live assertion failure' >&2
    exit 1
fi
[[ "${FAILED}" -eq $((before_failed + 1)) ]]
FAILED=${before_failed}
pass 'expected helper failure increments exactly one parent counter'
expect_success 'reported invocation selects exact JSON path' select_log "${REPORT}"
selected=$(select_log "${REPORT}")
[[ "${selected}" == "${ORIGINAL}" ]] || { echo 'Selected wrong log' >&2; exit 1; }
expect_success 'relative reported scan path' select_log \
    "Evaluation log written: .complytime/scan/evaluation-log-legacy.json [target: ${TARGET}]"
expect_failure 'stale log without invocation report' 'exactly one' select_log 'Scan complete'
expect_failure 'wrong reported target' 'exactly one' select_log "${REPORT/complyctl-repo/other}"
expect_failure 'ambiguous reported logs' 'exactly one' select_log "${REPORT}"$'\n'"${REPORT}"
expect_failure 'reported missing file' 'missing path' select_log "${REPORT/legacy/absent}"
printf '{' > "${MUTATED}"
expect_failure 'malformed reported JSON' 'malformed or mismatched' select_log \
    "${REPORT/legacy/mutated}"
mutate_log '.target.id = "other"'
expect_failure 'wrong log target despite matching report' 'malformed or mismatched' select_log \
    "${REPORT/legacy/mutated}"
mutate_log '.metadata.id = "other"'
expect_failure 'wrong log repository' 'malformed or mismatched' select_log "${REPORT/legacy/mutated}"
expect_failure 'reported non-JSON log' 'exactly one' select_log "${REPORT/.json/.yaml}"
cp "${ORIGINAL}" "${FIXTURES}/outside.json"
expect_failure 'reported log outside scan tree' 'escapes workspace' select_log \
    "Evaluation log written: ${FIXTURES}/outside.json [target: ${TARGET}]"
mkdir -p "${FIXTURES}/escaped-scan/.complytime"
ln -s "${WORKSPACE}/.complytime/scan" "${FIXTURES}/escaped-scan/.complytime/scan"
expect_failure 'scan directory symlink cannot escape workspace' 'escapes workspace' \
    select_evaluation_log "${FIXTURES}/escaped-scan" "${REPORT}" "${POLICY}" "${TARGET}"

mutate_log --arg root "${WORKSPACE}/" \
    '(.evaluations[]."assessment-logs"[].evidence[].source.coordinate) |= ($root + .)'
expect_success 'absolute legacy coordinates inside workspace' check_log "${MUTATED}"

for field in id source.coordinate 'source."reference-id"' source.digest; do
    mutate_log "del(.evaluations[0].\"assessment-logs\"[0].evidence[0].${field})"
    expect_failure "missing ${field}" 'missing or malformed evidence' check_log "${MUTATED}"
done
mutate_log '.evaluations[0]."assessment-logs"[0].evidence[0].source.coordinate = 42'
expect_failure 'wrong source field type' 'missing or malformed evidence' check_log "${MUTATED}"
mutate_log '.evaluations[0]."assessment-logs"[0].evidence |= (map({key:.id,value:.}) | from_entries)'
expect_failure 'evidence object instead of array' 'evidence must be an array' check_log "${MUTATED}"
for digest in 'sha512:abcd' 'sha256:abc' 'sha256:zzzz' "sha256:$(printf '%064d' 0)"; do
    mutate_log --arg digest "${digest}" \
        '.evaluations[0]."assessment-logs"[0].evidence[0].source.digest = $digest'
    expect_failure "bad digest ${digest}" 'digest' check_log "${MUTATED}"
done

AMPEL_FILE="${WORKSPACE}/.complytime/ampel/results/legacy-ampel.intoto.json"
cp "${AMPEL_FILE}" "${FIXTURES}/saved-ampel"
printf 'changed' >> "${AMPEL_FILE}"
expect_failure 'modified artifact bytes' 'digest mismatch' check_log "${ORIGINAL}"
: > "${AMPEL_FILE}"
expect_failure 'empty artifact' 'nonempty regular file' check_log "${ORIGINAL}"
cp "${FIXTURES}/saved-ampel" "${AMPEL_FILE}"
chmod 000 "${AMPEL_FILE}"
if [[ -r "${AMPEL_FILE}" ]]; then
    SKIPPED=$((SKIPPED + 1))
    echo 'SKIP: unreadable artifact: current user can read mode-000 files (e.g. root)'
else
    expect_failure 'unreadable artifact' 'readable nonempty regular file' check_log "${ORIGINAL}"
fi
chmod 600 "${AMPEL_FILE}"
rm "${AMPEL_FILE}"
expect_failure 'missing artifact' 'missing path' check_log "${ORIGINAL}"
mkdir "${AMPEL_FILE}"
expect_failure 'directory is not evidence' 'regular file' check_log "${ORIGINAL}"
rmdir "${AMPEL_FILE}"
cp "${FIXTURES}/saved-ampel" "${AMPEL_FILE}"
cp "${AMPEL_FILE}" "${FIXTURES}/outside.json"
mutate_log --arg path "${FIXTURES}/outside.json" \
    '.evaluations[0]."assessment-logs"[0].evidence[0].source.coordinate = $path'
expect_failure 'absolute coordinate escape' 'escapes workspace' check_log "${MUTATED}"
mutate_log '.evaluations[0]."assessment-logs"[0].evidence[0].source.coordinate = "../outside.json"'
expect_failure 'traversal coordinate escape' 'escapes workspace' check_log "${MUTATED}"
ln -s "${FIXTURES}/outside.json" "${WORKSPACE}/escape.json"
mutate_log '.evaluations[0]."assessment-logs"[0].evidence[0].source.coordinate = "escape.json"'
expect_failure 'symlink coordinate escape' 'escapes workspace' check_log "${MUTATED}"

mutate_log 'del(.metadata."mapping-references"[0])'
expect_failure 'missing mapping reference' 'missing, duplicate, or malformed reference' \
    check_log "${MUTATED}"
mutate_log '.metadata."mapping-references" += [.metadata."mapping-references"[0]]'
expect_failure 'duplicate mapping reference' 'missing, duplicate, or malformed reference' \
    check_log "${MUTATED}"
mutate_log 'del(.metadata."mapping-references"[0].url)'
expect_failure 'missing reference URL' 'missing, duplicate, or malformed reference' \
    check_log "${MUTATED}"
for escape in '\u0000' '\n' '\t'; do
    mutate_log ".metadata.\"mapping-references\"[0].url += \"${escape}\""
    expect_failure 'raw control character in reference URL' 'malformed reference' check_log "${MUTATED}"
done
for url in 'https://example.org/file' 'file://remote/etc/passwd' 'file:relative' \
    'file:///bad%xy' 'file:///bad%0' 'file:///bad%00' 'file:///bad%0A' \
    'file:///bad?query' 'file:///bad#fragment' 'file:///raw space'; do
    mutate_log --arg url "${url}" '.metadata."mapping-references"[0].url = $url'
    expect_failure "invalid locator ${url}" 'file URL' check_log "${MUTATED}"
done
cp "${AMPEL_FILE}" "${WORKSPACE}/equal-content.json"
url=$(jq -rn --arg path "${WORKSPACE}/equal-content.json" \
    '"file://" + ($path | split("/") | map(@uri) | join("/"))')
mutate_log --arg url "${url}" '.metadata."mapping-references"[0].url = $url'
expect_failure 'equal bytes at wrong locator' 'different files' check_log "${MUTATED}"

mutate_log 'del(.evaluations[0]."assessment-logs"[0].evidence[1])'
expect_failure 'unmapped decoy cannot supply missing Snappy evidence' \
    'missing or malformed evidence' check_log "${MUTATED}"
mutate_log '.evaluations[0]."assessment-logs"[0].requirement."entry-id" = "unrelated"'
expect_failure 'unrelated assessment evidence' 'missing or malformed evidence' check_log "${MUTATED}"
mutate_log '.evaluations[0]."assessment-logs"[0] as $assessment |
    .evaluations[0]."assessment-logs" =
    [($assessment | .evidence = [$assessment.evidence[0]]),
     ($assessment | .evidence = [$assessment.evidence[1]])]'
expect_failure 'roles split between matching assessments cannot form a pair' \
    'missing or malformed evidence' check_log "${MUTATED}"
printf '{"predicate":{"results":[]}}' > "${AMPEL_FILE}"
digest=$(sha256sum "${AMPEL_FILE}")
mutate_log --arg digest "sha256:${digest:0:64}" \
    '.evaluations[0]."assessment-logs"[0].evidence[0].source.digest = $digest'
expect_failure 'selected Ampel file lacks requirement content' 'lacks block-force-push' \
    check_log "${MUTATED}"
cp "${FIXTURES}/saved-ampel" "${AMPEL_FILE}"

publish_fixture first retained
FIRST="${WORKSPACE}/${EVIDENCE_SCAN_SUBDIR}/evaluation-log-first.json"
cp "${FIRST}" "${FIXTURES}/snapshot-first.json"
expect_success 'retained layout with encoded spaces percent and Unicode' check_log "${FIRST}"
publish_fixture second retained
SECOND="${WORKSPACE}/${EVIDENCE_SCAN_SUBDIR}/evaluation-log-second.json"
cp "${SECOND}" "${FIXTURES}/snapshot-second.json"
jq -en --argjson first "$(check_log "${FIRST}")" --argjson second "$(check_log "${SECOND}")" '
    ($first | map({role,digest}) | sort_by(.role)) ==
        ($second | map({role,digest}) | sort_by(.role))
' >/dev/null
history() {
    validate_evidence_history "${WORKSPACE}" "${FIXTURES}/snapshot-first.json" \
        "${FIXTURES}/snapshot-second.json" "${POLICY}" "${TARGET}" "${REQUIREMENT}"
}
expect_success 'two retained equal-content evaluations remain independently addressable' history
first_file=$(jq -r '.evaluations[0]."assessment-logs"[0].evidence[0].source.coordinate' "${FIRST}")
printf 'overwritten by B' >> "${WORKSPACE}/${first_file}"
expect_failure 'later evaluation overwrites historical evidence' 'digest mismatch' history
rm "${WORKSPACE}/${first_file}"
expect_failure 'later evaluation removes historical evidence' 'missing path' history
publish_fixture first retained
cp "${FIRST}" "${FIXTURES}/snapshot-second.json"
expect_failure 'equal-content evaluations collapse identities' 'collapsed distinct' history

ORIGINAL="${FIRST}"
mutate_log '.evaluations[0]."assessment-logs" += [.evaluations[0]."assessment-logs"[0]]'
expect_success 'one evidence instance reused across assessments' check_log "${MUTATED}"
records=$(check_log "${MUTATED}")
[[ $(jq 'length' <<< "${records}") -eq 2 ]] || { echo 'Shared instance counted twice'; exit 1; }
mutate_log '.evaluations[0]."assessment-logs"[0].evidence +=
    [.evaluations[0]."assessment-logs"[0].evidence[0] | .id = "ampel_collapsed"]'
expect_failure 'distinct evidence IDs collapse onto one reference and coordinate' \
    'reuse an identity' check_log "${MUTATED}"
printf '{"supporting":"metadata, not an attestation"}' \
    > "${WORKSPACE}/.complytime/ampel/evaluations/second/metadata.json"
expect_success 'nearby supporting metadata is not evidence' check_log "${SECOND}"
expect_failure 'assertion runner recognizes injected mapping failure' 'missing or malformed' \
    validate_mapped_evidence "${WORKSPACE}" "${SECOND}" "wrong-policy" "${TARGET}" "${REQUIREMENT}"
echo "Evidence regressions: ${PASSED} passed, ${FAILED} failed, ${SKIPPED} skipped"
[[ "${FAILED}" -eq 0 ]]
