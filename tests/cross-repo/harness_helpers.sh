#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Harness-owned fixtures and explicitly selected host binaries remain separate.
# These outputs are consumed by the sourcing live harness and regression tests.
# shellcheck disable=SC2034

select_host_binaries() {
    local harness_root="$1" binary_dir
    binary_dir="${COMPLYCTL_BIN_DIR-${harness_root}/bin}"
    if [[ "${binary_dir}" != /* || ! -d "${binary_dir}" ]]; then
        echo 'FATAL: COMPLYCTL_BIN_DIR must name an existing absolute directory.' >&2
        return 1
    fi
    BINARY="${binary_dir}/complyctl"
    MOCK_REGISTRY="${binary_dir}/mock-oci-registry"
    TESTDATA_DIR="${harness_root}/tests/cross-repo/testdata"
    local executable
    for executable in "${BINARY}" "${MOCK_REGISTRY}"; do
        if [[ ! -f "${executable}" || ! -x "${executable}" ]]; then
            echo "FATAL: selected host executable missing or not executable: ${executable}" >&2
            echo 'Build both selected host binaries before invoking this script.' >&2
            return 1
        fi
    done
}

# Copy the logs actually produced by the harness, without converting formats.
export_evaluation_logs() {
    local scan_dir="$1" destination="$2" file count=0
    mkdir -p "${destination}" || return 1
    if [[ -d "${scan_dir}" ]]; then
        while IFS= read -r -d '' file; do
            cp -- "${file}" "${destination}/" || return 1
            count=$((count + 1))
        done < <(find "${scan_dir}" -maxdepth 1 -type f \
            \( -name 'evaluation-log-*.json' -o -name 'evaluation-log-*.yaml' \) -print0)
    fi
    echo "  Exported ${count} EvaluationLog file(s) to ${destination}"
    if [[ "${count}" -eq 0 ]]; then
        echo '  WARNING: No EvaluationLog files found to export. Scan may not have produced output.'
    fi
}
