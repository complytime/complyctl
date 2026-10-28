#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Runs the protobuf and Go API analyzers within a shared baseline invocation.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

die() {
    printf 'compatibility analysis: %s\n' "$*" >&2
    exit 1
}

(($# == 1)) || die "expected the candidate repository path"
candidate_dir="$1"
[[ -d "${candidate_dir}" ]] || die "candidate repository does not exist: ${candidate_dir}"
[[ -n "${BASELINE_DIR:-}" ]] || die "baseline directory was not provided"
[[ -d "${BASELINE_DIR}" ]] || die "baseline directory does not exist: ${BASELINE_DIR}"

"${SCRIPT_DIR}/api-compatibility-proto.sh" "${candidate_dir}" "${BASELINE_DIR}"
"${SCRIPT_DIR}/api-compatibility-go.sh" "${candidate_dir}" "${BASELINE_DIR}"
