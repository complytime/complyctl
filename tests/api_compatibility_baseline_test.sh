#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Focused tests for scripts/api-compatibility-baseline.sh.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
readonly REPO_ROOT
readonly HELPER="${REPO_ROOT}/scripts/api-compatibility-baseline.sh"

test_root="$(mktemp -d)"
trap 'rm -rf "${test_root}"' EXIT

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

create_repository() {
    local name="$1" repository remote

    repository="${test_root}/${name}"
    remote="${test_root}/${name}-origin.git"

    git init -q "${repository}"
    git init --bare -q "${remote}"
    git -C "${repository}" config user.email "baseline-test@example.com"
    git -C "${repository}" config user.name "Baseline Test"
    git -C "${repository}" remote add origin "${remote}"
    printf 'initial\n' > "${repository}/fixture"
    git -C "${repository}" add fixture
    git -C "${repository}" commit -qm initial
    git -C "${repository}" push -q origin HEAD:main
    printf '%s\n' "${repository}"
}

commit_and_tag() {
    local repository="$1" tag="$2" remote="${3:-origin}"

    printf '%s\n' "${tag}" >> "${repository}/fixture"
    git -C "${repository}" add fixture
    git -C "${repository}" commit -qm "${tag}"
    git -C "${repository}" tag -a -m "${tag}" "${tag}"
    git -C "${repository}" push -q "${remote}" HEAD:main "refs/tags/${tag}"
}

assert_contains() {
    local output="$1" expected="$2"
    [[ "${output}" == *"${expected}"* ]] || fail "expected output to contain: ${expected}"
}

test_highest_stable_tag_is_selected() {
    local repository output expected_commit

    repository="$(create_repository stable-selection)"
    commit_and_tag "${repository}" v1.0.0
    commit_and_tag "${repository}" v1.2.0
    commit_and_tag "${repository}" v1.10.0
    expected_commit="$(git -C "${repository}" rev-parse 'v1.10.0^{commit}')"

    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.11.0 bash "${HELPER}" --repo "${repository}" -- true)"
    assert_contains "${output}" 'Selected baseline tag: v1.10.0'
    assert_contains "${output}" "Selected baseline commit: ${expected_commit}"
}

test_upstream_remote_is_preferred() {
    local repository upstream output expected_commit

    repository="$(create_repository upstream-preferred)"
    upstream="${test_root}/upstream-preferred.git"
    git init --bare -q "${upstream}"
    git -C "${repository}" remote add upstream "${upstream}"
    commit_and_tag "${repository}" v1.0.0 upstream
    expected_commit="$(git -C "${repository}" rev-parse 'v1.0.0^{commit}')"
    commit_and_tag "${repository}" v1.2.0 origin

    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.3.0 bash "${HELPER}" --repo "${repository}" -- true)"
    assert_contains "${output}" 'Selected baseline tag: v1.0.0'
    assert_contains "${output}" "Selected baseline commit: ${expected_commit}"
}

test_origin_remote_is_used_without_upstream() {
    local repository output expected_commit

    repository="$(create_repository origin-fallback)"
    commit_and_tag "${repository}" v1.0.0
    expected_commit="$(git -C "${repository}" rev-parse 'v1.0.0^{commit}')"

    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 bash "${HELPER}" --repo "${repository}" -- true)"
    assert_contains "${output}" 'Selected baseline tag: v1.0.0'
    assert_contains "${output}" "Selected baseline commit: ${expected_commit}"
}

test_remote_environment_override_is_used() {
    local repository upstream output

    repository="$(create_repository remote-override)"
    upstream="${test_root}/remote-override-upstream.git"
    git init --bare -q "${upstream}"
    git -C "${repository}" remote add upstream "${upstream}"
    commit_and_tag "${repository}" v1.0.0 upstream
    commit_and_tag "${repository}" v1.2.0 origin

    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.3.0 API_COMPATIBILITY_REMOTE=origin \
        bash "${HELPER}" --repo "${repository}" -- true)"
    assert_contains "${output}" 'Selected baseline tag: v1.2.0'
}

test_prerelease_tag_is_excluded() {
    local repository output

    repository="$(create_repository prerelease-exclusion)"
    commit_and_tag "${repository}" v1.4.0
    commit_and_tag "${repository}" v1.5.0-rc.1

    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.5.0 bash "${HELPER}" --repo "${repository}" -- true)"
    assert_contains "${output}" 'Selected baseline tag: v1.4.0'
}

test_leading_zero_versions_are_rejected() {
    local repository output status

    repository="$(create_repository leading-zero-version)"
    commit_and_tag "${repository}" v1.0.9
    commit_and_tag "${repository}" v1.01.10

    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.2.0 bash "${HELPER}" --repo "${repository}" -- true)"
    assert_contains "${output}" 'Selected baseline tag: v1.0.9'

    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.2.0-rc.1 bash "${HELPER}" --repo "${repository}" -- true)"
    assert_contains "${output}" 'Selected baseline tag: v1.0.9'

    set +e
    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.01.0 bash "${HELPER}" --repo "${repository}" -- true 2>&1)"
    status=$?
    set -e
    [[ ${status} -ne 0 ]] || fail 'expected leading-zero candidate version to fail'
    assert_contains "${output}" 'baseline resolution: candidate version is not semantic: 1.01.0'
}

test_remote_tags_are_authoritative() {
    local repository output expected_commit

    repository="$(create_repository remote-selection)"
    commit_and_tag "${repository}" v1.4.0
    expected_commit="$(git -C "${repository}" rev-parse 'v1.4.0^{commit}')"
    printf 'stale\n' >> "${repository}/fixture"
    git -C "${repository}" add fixture
    git -C "${repository}" commit -qm stale
    git -C "${repository}" tag -a -m v1.99.0 v1.99.0

    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.100.0 bash "${HELPER}" --repo "${repository}" -- true)"
    assert_contains "${output}" 'Selected baseline tag: v1.4.0'
    assert_contains "${output}" "Selected baseline commit: ${expected_commit}"
}

test_missing_stable_baseline_fails() {
    local repository output status

    repository="$(create_repository missing-baseline)"
    commit_and_tag "${repository}" v2.0.0-rc.1

    set +e
    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v2.0.0 bash "${HELPER}" --repo "${repository}" -- true 2>&1)"
    status=$?
    set -e
    [[ ${status} -ne 0 ]] || fail 'expected missing baseline to fail'
    assert_contains "${output}" 'baseline resolution: no stable v2 baseline tag is available'
}

test_worktree_is_cleaned_up() {
    local repository output output_path recorded_path status

    repository="$(create_repository cleanup)"
    commit_and_tag "${repository}" v1.0.0
    output_path="${test_root}/baseline-path"
    set +e
    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 bash "${HELPER}" --repo "${repository}" -- \
        bash -c 'test -f "$BASELINE_DIR/fixture" && printf "%s\\n" "$BASELINE_DIR" > "$1" && false' -- \
        "${output_path}")"
    status=$?
    set -e
    recorded_path="$(<"${output_path}")"

    [[ ${status} -ne 0 ]] || fail 'expected the baseline command failure to propagate'
    assert_contains "${output}" 'Selected baseline tag: v1.0.0'
    [[ ! -e "${recorded_path}" ]] || fail "baseline worktree was not removed: ${recorded_path}"
    ! git -C "${repository}" worktree list --porcelain | grep -Fqx "worktree ${recorded_path}" ||
        fail "baseline worktree remains registered: ${recorded_path}"
}

test_highest_stable_tag_is_selected
test_upstream_remote_is_preferred
test_origin_remote_is_used_without_upstream
test_remote_environment_override_is_used
test_prerelease_tag_is_excluded
test_leading_zero_versions_are_rejected
test_remote_tags_are_authoritative
test_missing_stable_baseline_fails
test_worktree_is_cleaned_up
printf 'PASS: API compatibility baseline helper tests\n'
