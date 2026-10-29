#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Checks the provider protobuf contract with a supported Buf CLI.

set -euo pipefail

readonly BUF_MIN_VERSION="1.50.0"
readonly BUF_INCOMPATIBLE_EXIT_CODE=100

die() {
    printf 'protobuf compatibility: %s\n' "$*" >&2
    exit 1
}

version_at_least() {
    local version="$1" minimum="$2" index have need
    local LC_ALL=C
    local -a installed required

    [[ "${version}" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]] ||
        return 1
    IFS=. read -r -a installed <<<"${version%%+*}"
    IFS=. read -r -a required <<<"${minimum}"
    for index in 0 1 2; do
        have="${installed[index]}"
        need="${required[index]}"
        # Compare digit counts first to avoid integer overflow on external input.
        ((${#have} > ${#need})) && return 0
        ((${#have} < ${#need})) && return 1
        [[ "${have}" > "${need}" ]] && return 0
        [[ "${have}" < "${need}" ]] && return 1
    done
    return 0
}

require_buf() {
    local buf_path version

    buf_path="$(command -v buf 2>/dev/null)" || die "tool setup: buf is not installed"
    version="$("${buf_path}" --version)" || die "tool setup: cannot determine Buf version"
    version_at_least "${version}" "${BUF_MIN_VERSION}" ||
        die "tool setup: buf >= ${BUF_MIN_VERSION} required, found ${version}"
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
