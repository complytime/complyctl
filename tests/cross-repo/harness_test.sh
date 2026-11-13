#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Exercise the live harness's binary selection and export functions offline.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=tests/cross-repo/harness_helpers.sh
source "${SCRIPT_DIR}/harness_helpers.sh"
# shellcheck source=tests/cross-repo/evidence_assertions.sh
source "${SCRIPT_DIR}/evidence_assertions.sh"
for tool in make timeout cmp; do
    command -v "${tool}" >/dev/null || { echo "Missing prerequisite: ${tool}" >&2; exit 1; }
done
FIXTURES=$(mktemp -d)
trap 'rm -rf "${FIXTURES}"' EXIT
PASSED=0
FAILED=0
HARNESS="${FIXTURES}/canonical harness"
EXTERNAL="${FIXTURES}/released host/bin"
mkdir -p "${HARNESS}/bin" "${HARNESS}/tests/cross-repo/testdata" "${EXTERNAL}"

fixture_binary() {
    local path="$1" marker="$2"
    printf '#!/usr/bin/env bash\nprintf "%%s\\n" "%s"\n' "${marker}" > "${path}"
    chmod +x "${path}"
}
for binary in complyctl mock-oci-registry; do
    fixture_binary "${HARNESS}/bin/${binary}" "default-${binary}"
    fixture_binary "${EXTERNAL}/${binary}" "external-${binary}"
done

unset COMPLYCTL_BIN_DIR
select_host_binaries "${HARNESS}"
[[ $("${BINARY}") == default-complyctl ]]
[[ $("${MOCK_REGISTRY}") == default-mock-oci-registry ]]
pass 'default invocation selects both canonical host binaries'
COMPLYCTL_BIN_DIR="${EXTERNAL}"
# A default binary or implicit make invocation must not be used for external hosts.
for binary in complyctl mock-oci-registry; do
    printf '#!/usr/bin/env bash\nexit 97\n' > "${HARNESS}/bin/${binary}"
done
select_host_binaries "${HARNESS}"
[[ $("${BINARY}") == external-complyctl ]]
[[ $("${MOCK_REGISTRY}") == external-mock-oci-registry ]]
[[ "${TESTDATA_DIR}" == "${HARNESS}/tests/cross-repo/testdata" ]]
pass 'external binaries with spaces preserve canonical harness fixture ownership'

for directory in '' relative/path "${FIXTURES}/missing"; do
    COMPLYCTL_BIN_DIR="${directory}"
    if select_host_binaries "${HARNESS}" 2>/dev/null; then
        fail "invalid explicit binary directory accepted: ${directory}"
    else
        pass "invalid explicit binary directory fails without fallback: ${directory}"
    fi
done
COMPLYCTL_BIN_DIR="${EXTERNAL}"
for binary in complyctl mock-oci-registry; do
    chmod -x "${EXTERNAL}/${binary}"
    if select_host_binaries "${HARNESS}" 2>/dev/null; then
        fail "non-executable ${binary} accepted"
    else
        pass "non-executable ${binary} fails without fallback"
    fi
    chmod +x "${EXTERNAL}/${binary}"
    mv "${EXTERNAL}/${binary}" "${EXTERNAL}/${binary}.saved"
    if select_host_binaries "${HARNESS}" 2>/dev/null; then
        fail "missing ${binary} accepted"
    else
        pass "missing ${binary} fails without fallback"
    fi
    mv "${EXTERNAL}/${binary}.saved" "${EXTERNAL}/${binary}"
done

SCAN="${FIXTURES}/scan"
EXPORT="${FIXTURES}/export"
mkdir -p "${SCAN}"
printf '{"actual":"Ampel JSON"}\n' > "${SCAN}/evaluation-log-ampel.json"
printf 'not a log\n' > "${SCAN}/supporting-metadata.json"
export_evaluation_logs "${SCAN}" "${EXPORT}" >/dev/null
cmp "${SCAN}/evaluation-log-ampel.json" "${EXPORT}/evaluation-log-ampel.json"
[[ ! -e "${EXPORT}/supporting-metadata.json" ]]
pass 'JSON-only export includes actual Ampel log and excludes supporting metadata'
printf 'actual: OPA YAML\n' > "${SCAN}/evaluation-log-opa.yaml"
export_evaluation_logs "${SCAN}" "${EXPORT}" >/dev/null
cmp "${SCAN}/evaluation-log-opa.yaml" "${EXPORT}/evaluation-log-opa.yaml"
cmp "${SCAN}/evaluation-log-ampel.json" "${EXPORT}/evaluation-log-ampel.json"
pass 'mixed export preserves both actual serializations byte-for-byte'

# Prove the Make entry point propagates an evidence failure. The shim preserves
# fixture hash generation but corrupts stdin hashing used by the real checker.
mkdir -p "${FIXTURES}/fault-bin"
real_sha256sum=$(command -v sha256sum)
cat > "${FIXTURES}/fault-bin/sha256sum" <<SHIM
#!/usr/bin/env bash
if [[ \$# -eq 0 ]]; then
    printf '%064d  -\\n' 0
else
    exec "${real_sha256sum}" "\$@"
fi
SHIM
chmod +x "${FIXTURES}/fault-bin/sha256sum"
recipe=$(make --no-print-directory -n -C "${SCRIPT_DIR}/../.." test-cross-repo-evidence-mappings)
if [[ "${recipe}" == *harness_test.sh* || "${recipe}" == *test-cross-repo-evidence* ]]; then
    echo 'FAIL: evidence-only dependency must not re-enter the aggregate harness' >&2
    exit 1
fi
pass 'evidence-only dependency cannot recurse into the aggregate harness'
injected_rc=0
PATH="${FIXTURES}/fault-bin:${PATH}" timeout 120 make --no-print-directory \
    -C "${SCRIPT_DIR}/../.." test-cross-repo-evidence-mappings \
    > "${FIXTURES}/injected.log" 2>&1 || injected_rc=$?
if [[ "${injected_rc}" -eq 0 ]]; then
    fail 'offline Make target accepted injected evidence digest failures'
elif [[ "${injected_rc}" -eq 124 ]]; then
    fail 'injected evidence subprocess exceeded its 120-second deadline'
elif grep -q 'digest mismatch' "${FIXTURES}/injected.log"; then
    pass 'offline Make target propagates injected evidence digest failures'
else
    cat "${FIXTURES}/injected.log" >&2
    fail 'injected failure did not exercise the evidence digest check'
fi
echo "Harness regressions: ${PASSED} passed, ${FAILED} failed"
[[ "${FAILED}" -eq 0 ]]
