#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Focused tests for scripts/api-compatibility-baseline.sh.

set -euo pipefail

# Default-selection scenarios must not inherit the caller's explicit override.
unset API_COMPATIBILITY_REMOTE

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
readonly REPO_ROOT
readonly HELPER="${REPO_ROOT}/scripts/api-compatibility-baseline.sh"

# shellcheck source=tests/api_compat_helpers.sh
source "${REPO_ROOT}/tests/api_compat_helpers.sh"

test_root="$(mktemp -d)"
trap 'rm -rf "${test_root}"' EXIT

# Substitute only reported URLs so canonical selection uses local Git transports.
git() {
    local reported_url
    if [[ $# -eq 5 && "$1" == -C && "$3" == remote && "$4" == get-url ]] &&
        reported_url="$(command git -C "$2" config --get "remote.$5.test-url")"; then
        printf '%s\n' "${reported_url}"
    else
        command git "$@"
    fi
}
export -f git

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
    git -C "${repository}" config remote.upstream.test-url \
        git@github.com:complytime/complyctl.git
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

test_noncanonical_remotes_fail_closed() {
    local repository output remote_url marker

    repository="$(create_repository noncanonical-remotes)"
    commit_and_tag "${repository}" v1.0.0
    marker="${test_root}/unexpected-command"
    for remote_url in \
        https://github.com/contributor/complyctl.git \
        https://github.com/complytime/complyctl-fork.git \
        https://github.com.evil.example/complytime/complyctl.git \
        https://github.com/complytime/complyctl.git/other; do
        git -C "${repository}" config remote.origin.test-url "${remote_url}"
        if output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 \
            bash "${HELPER}" --repo "${repository}" -- touch "${marker}" 2>&1)"; then
            fail "expected noncanonical origin to fail: ${remote_url}"
        fi
        assert_contains "${output}" 'no canonical remote found'
        assert_contains "${output}" 'API_COMPATIBILITY_REMOTE'
    done

    git -C "${repository}" remote add upstream "${test_root}/misleading-upstream.git"
    git -C "${repository}" config remote.upstream.test-url \
        git@github.com:contributor/complyctl.git
    if output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 \
        bash "${HELPER}" --repo "${repository}" -- touch "${marker}" 2>&1)"; then
        fail 'expected misleading upstream and noncanonical origin to fail'
    fi
    assert_contains "${output}" 'no canonical remote found'
    [[ ! -e "${marker}" ]] || fail 'command ran despite noncanonical baseline'
}

test_canonical_origin_is_used_with_noncanonical_upstream() {
    local repository output

    repository="$(create_repository misleading-upstream)"
    commit_and_tag "${repository}" v1.0.0
    git -C "${repository}" remote add upstream "${test_root}/unavailable-upstream.git"
    git -C "${repository}" config remote.upstream.test-url \
        https://github.com/contributor/complyctl.git
    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 \
        bash "${HELPER}" --repo "${repository}" -- true)"
    assert_contains "${output}" 'Selected baseline tag: v1.0.0'
}

test_canonical_url_forms_are_accepted() {
    local repository remote_url output

    repository="$(create_repository canonical-url-forms)"
    commit_and_tag "${repository}" v1.0.0
    for remote_url in \
        https://github.com/complytime/complyctl \
        https://github.com/complytime/complyctl.git \
        git@github.com:complytime/complyctl \
        git@github.com:complytime/complyctl.git \
        ssh://git@github.com/complytime/complyctl \
        ssh://git@github.com/complytime/complyctl.git; do
        git -C "${repository}" config remote.origin.test-url "${remote_url}"
        output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 \
            bash "${HELPER}" --repo "${repository}" -- true)"
        assert_contains "${output}" 'Selected baseline tag: v1.0.0'
    done
}

test_remote_environment_override_is_used() {
    local repository upstream output

    repository="$(create_repository remote-override)"
    upstream="${test_root}/remote-override-upstream.git"
    git init --bare -q "${upstream}"
    git -C "${repository}" remote add upstream "${upstream}"
    commit_and_tag "${repository}" v1.0.0 upstream
    commit_and_tag "${repository}" v1.2.0 origin
    git -C "${repository}" config remote.origin.test-url \
        https://github.com/contributor/complyctl.git

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

test_progress_is_quiet_and_errors_remain_visible() {
    local repository output

    repository="$(create_repository quiet-diagnostics)"
    commit_and_tag "${repository}" v1.0.0
    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 \
        bash "${HELPER}" --repo "${repository}" -- true 2>&1)"
    assert_contains "${output}" 'Selected baseline tag: v1.0.0'
    assert_contains "${output}" 'Selected baseline commit:'
    [[ "${output}" != *'From '* && "${output}" != *'Preparing worktree'* ]] ||
        fail "routine Git progress leaked: ${output}"

    git -C "${repository}" remote add unavailable "${repository}/missing.git"
    assert_failure 'fatal:' 'baseline resolution: failed to list tags' -- \
        env API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 \
        API_COMPATIBILITY_REMOTE=unavailable \
        bash "${HELPER}" --repo "${repository}" -- true
}

test_highest_stable_tag_is_selected
test_upstream_remote_is_preferred
test_origin_remote_is_used_without_upstream
test_noncanonical_remotes_fail_closed
test_canonical_origin_is_used_with_noncanonical_upstream
test_canonical_url_forms_are_accepted
test_remote_environment_override_is_used
test_prerelease_tag_is_excluded
test_leading_zero_versions_are_rejected
test_remote_tags_are_authoritative
test_missing_stable_baseline_fails
test_worktree_is_cleaned_up
test_progress_is_quiet_and_errors_remain_visible
printf 'PASS: API compatibility baseline helper tests\n'
