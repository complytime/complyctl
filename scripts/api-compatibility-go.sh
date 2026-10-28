#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Checks the exported provider Go API with a pinned apidiff binary.

set -euo pipefail

readonly APIDIFF_MODULE="golang.org/x/exp"
# The pinned public module revision resembles a generic API key to secret scanners.
readonly APIDIFF_VERSION="v0.0.0-20251023183803-a4bb9ffd2546" #gitleaks:allow
readonly PROVIDER_PACKAGE="github.com/complytime/complyctl/pkg/provider"

die() {
    printf 'Go API compatibility: %s\n' "$*" >&2
    exit 1
}

require_apidiff() {
    local apidiff_path metadata required_module

    apidiff_path="$(command -v apidiff 2>/dev/null)" || die "tool setup: apidiff is not installed"
    metadata="$(go version -m "${apidiff_path}")" ||
        die "tool setup: cannot inspect the apidiff binary"
    required_module=$'\tmod\t'"${APIDIFF_MODULE}"$'\t'"${APIDIFF_VERSION}"
    [[ "${metadata}" == *"${required_module}"* ]] ||
        die "tool setup: expected ${APIDIFF_MODULE} ${APIDIFF_VERSION}"
    printf '%s\n' "${apidiff_path}"
}

write_export_data() {
    local repository_dir="$1" export_file="$2" label="$3" apidiff_path="$4"

    (
        cd "${repository_dir}"
        GOWORK=off "${apidiff_path}" -w "${export_file}" "${PROVIDER_PACKAGE}"
    ) || die "package loading: cannot load ${label} ${PROVIDER_PACKAGE}"
}

(($# == 2)) || die "expected candidate and baseline repository paths"
candidate_dir="$1"
baseline_dir="$2"
[[ -f "${candidate_dir}/go.mod" ]] || die "package loading: candidate go.mod is unavailable"
[[ -f "${baseline_dir}/go.mod" ]] || die "package loading: baseline go.mod is unavailable"

apidiff_path="$(require_apidiff)"
temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/complyctl-apidiff.XXXXXX")" ||
    die "analysis: cannot create temporary directory"
trap 'rm -rf "${temporary_dir}"' EXIT

baseline_export="${temporary_dir}/baseline.export"
candidate_export="${temporary_dir}/candidate.export"
write_export_data "${baseline_dir}" "${baseline_export}" "baseline" "${apidiff_path}"
write_export_data "${candidate_dir}" "${candidate_export}" "candidate" "${apidiff_path}"

if ! differences="$("${apidiff_path}" -incompatible "${baseline_export}" \
    "${candidate_export}")"; then
    die "analysis: apidiff comparison failed"
fi
if [[ -n "${differences}" ]]; then
    printf '%s\n' "${differences}" >&2
    die "incompatible Go API detected"
fi
