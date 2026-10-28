#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Checks the provider protobuf contract with the pinned Buf CLI.

set -euo pipefail

readonly BUF_VERSION="1.50.0"
readonly BUF_INCOMPATIBLE_EXIT_CODE=100

die() {
    printf 'protobuf compatibility: %s\n' "$*" >&2
    exit 1
}

require_buf() {
    local buf_path version

    buf_path="$(command -v buf 2>/dev/null)" || die "tool setup: buf is not installed"
    version="$("${buf_path}" --version)" || die "tool setup: cannot determine Buf version"
    [[ "${version}" == "${BUF_VERSION}" ]] ||
        die "tool setup: expected buf ${BUF_VERSION}, found ${version}"
    printf '%s\n' "${buf_path}"
}

(($# == 2)) || die "expected candidate and baseline repository paths"
candidate_dir="$1"
baseline_dir="$2"
[[ -f "${candidate_dir}/buf.yaml" ]] || die "analysis: candidate buf.yaml is unavailable"
[[ -f "${baseline_dir}/buf.yaml" ]] || die "analysis: baseline buf.yaml is unavailable"

buf_path="$(require_buf)"
set +e
"${buf_path}" breaking "${candidate_dir}" --against "${baseline_dir}" --error-format text
status=$?
set -e

case "${status}" in
    0)
        ;;
    "${BUF_INCOMPATIBLE_EXIT_CODE}")
        die "incompatible protobuf contract detected"
        ;;
    *)
        die "analysis: buf breaking failed with exit status ${status}"
        ;;
esac
