#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Resolves a release baseline and runs one command while its isolated worktree exists.

set -euo pipefail

readonly FALLBACK_REMOTE="origin"
readonly PREFERRED_REMOTE="upstream"
readonly WORKTREE_PREFIX="complyctl-api-baseline"

die() {
    printf 'baseline resolution: %s\n' "$*" >&2
    exit 1
}

usage() {
    cat >&2 <<'EOF'
Usage: api-compatibility-baseline.sh [--repo <path>] -- <command> [args...]

Set API_COMPATIBILITY_CANDIDATE_VERSION to select the candidate semantic-version
major. When it is unset, the nearest semantic-version Git tag selects the major.

Set API_COMPATIBILITY_REMOTE to select the Git remote used for authoritative
release tags. When it is unset, upstream is preferred and origin is the fallback.
EOF
    exit 2
}

is_semantic_version() {
    local version="$1" core_version prerelease identifier

    [[ "${version}" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]] || return 1
    core_version="${version%%+*}"
    [[ "${core_version}" == *-* ]] || return 0
    prerelease="${core_version#*-}"

    # SemVer permits leading zeros only in non-numeric prerelease identifiers.
    IFS=. read -r -a prerelease_identifiers <<<"${prerelease}"
    for identifier in "${prerelease_identifiers[@]}"; do
        [[ "${identifier}" =~ ^0[0-9]+$ ]] && return 1
    done

    return 0
}

is_stable_tag_in_major() {
    local tag="$1" candidate_major="$2" version

    version="${tag#v}"
    is_semantic_version "${version}" && [[ "${version}" != *-* ]] &&
        [[ "${version%%.*}" == "${candidate_major}" ]]
}

version_is_higher() {
    local candidate="$1" current="$2"
    local candidate_version current_version
    local candidate_minor candidate_patch current_minor current_patch

    candidate_version="${candidate#v}"
    current_version="${current#v}"
    candidate_version="${candidate_version%%+*}"
    current_version="${current_version%%+*}"
    IFS=. read -r _ candidate_minor candidate_patch <<<"${candidate_version}"
    IFS=. read -r _ current_minor current_patch <<<"${current_version}"

    ((10#${candidate_minor} > 10#${current_minor})) ||
        ((10#${candidate_minor} == 10#${current_minor} && 10#${candidate_patch} > 10#${current_patch}))
}

repo=""
while (($# > 0)); do
    case "$1" in
        --repo)
            (($# >= 2)) || usage
            repo="$2"
            shift 2
            ;;
        --)
            shift
            break
            ;;
        *)
            usage
            ;;
    esac
done

(($# > 0)) || usage

if [[ -z "${repo}" ]]; then
    repo="$(git rev-parse --show-toplevel 2>/dev/null)" || die "not inside a Git repository"
fi
repo="$(git -C "${repo}" rev-parse --show-toplevel 2>/dev/null)" || die "invalid repository: ${repo}"

candidate_version="${API_COMPATIBILITY_CANDIDATE_VERSION:-}"
if [[ -z "${candidate_version}" ]]; then
    candidate_tag="$(git -C "${repo}" describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null)" ||
        die "cannot derive the candidate version from a Git tag"
    candidate_version="${candidate_tag#v}"
fi
candidate_version="${candidate_version#v}"
is_semantic_version "${candidate_version}" ||
    die "candidate version is not semantic: ${candidate_version}"
candidate_major="${candidate_version%%.*}"

remote="${API_COMPATIBILITY_REMOTE:-}"
if [[ -z "${remote}" ]]; then
    if git -C "${repo}" remote get-url "${PREFERRED_REMOTE}" >/dev/null 2>&1; then
        remote="${PREFERRED_REMOTE}"
    else
        remote="${FALLBACK_REMOTE}"
    fi
fi

git -C "${repo}" remote get-url "${remote}" >/dev/null 2>&1 ||
    die "${remote} remote is not configured"
remote_tag_refs="$(git -C "${repo}" ls-remote --tags --refs "${remote}")" ||
    die "failed to list tags from ${remote}"

tags=( )
while read -r _ tag_ref; do
    tag="${tag_ref#refs/tags/}"
    [[ "${tag}" == "${tag_ref}" ]] && continue
    [[ -n "${tag}" ]] && tags+=("${tag}")
done <<<"${remote_tag_refs}"

baseline_tag=""
for tag in "${tags[@]}"; do
    if ! is_stable_tag_in_major "${tag}" "${candidate_major}"; then
        continue
    fi
    if [[ -z "${baseline_tag}" ]] || version_is_higher "${tag}" "${baseline_tag}"; then
        baseline_tag="${tag}"
    fi
done
[[ -n "${baseline_tag}" ]] || die "no stable v${candidate_major} baseline tag is available"

git -C "${repo}" fetch --no-tags "${remote}" "refs/tags/${baseline_tag}" ||
    die "failed to fetch selected tag ${baseline_tag}"
baseline_commit="$(git -C "${repo}" rev-parse 'FETCH_HEAD^{commit}')" ||
    die "failed to resolve selected tag ${baseline_tag} to a commit"
temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/${WORKTREE_PREFIX}.XXXXXX")" ||
    die "failed to create a temporary worktree directory"
baseline_dir="${temporary_root}/baseline"

cleanup() {
    git -C "${repo}" worktree remove --force "${baseline_dir}" >/dev/null 2>&1 || true
    rm -rf "${temporary_root}"
}
trap cleanup EXIT

git -C "${repo}" worktree add --detach "${baseline_dir}" "${baseline_commit}" >/dev/null ||
    die "failed to materialize baseline worktree"

printf 'Selected baseline tag: %s\n' "${baseline_tag}"
printf 'Selected baseline commit: %s\n' "${baseline_commit}"
BASELINE_TAG="${baseline_tag}" BASELINE_COMMIT="${baseline_commit}" BASELINE_DIR="${baseline_dir}" "$@"
