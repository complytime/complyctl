#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Runs all public API analyzers against one materialized release baseline.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

usage() {
    printf 'Usage: api-compatibility-check.sh [--repo <path>]\n' >&2
    exit 2
}

repo=""
while (($# > 0)); do
    case "$1" in
        --repo)
            (($# >= 2)) || usage
            repo="$2"
            shift 2
            ;;
        *)
            usage
            ;;
    esac
done

if [[ -z "${repo}" ]]; then
    repo="$(git rev-parse --show-toplevel 2>/dev/null)" || {
        printf 'baseline resolution: not inside a Git repository\n' >&2
        exit 1
    }
fi
repo="$(git -C "${repo}" rev-parse --show-toplevel 2>/dev/null)" || {
    printf 'baseline resolution: invalid repository: %s\n' "${repo}" >&2
    exit 1
}

exec bash "${SCRIPT_DIR}/api-compatibility-baseline.sh" --repo "${repo}" -- \
    "${SCRIPT_DIR}/api-compatibility-analyze.sh" "${repo}"
