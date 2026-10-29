#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Shared assertions and fixtures; callers own test_root and its cleanup.

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

assert_contains() {
    local output="$1" expected="$2"

    [[ "${output}" == *"${expected}"* ]] || fail "expected output to contain: ${expected}"
}

assert_success() {
    local output

    if ! output="$("$@" 2>&1)"; then
        fail "expected success, got: ${output}"
    fi
}

assert_failure() {
    local expected output
    local -a expected_messages=()

    while [[ "$1" != "--" ]]; do
        expected_messages+=("$1")
        shift
    done
    shift
    if output="$("$@" 2>&1)"; then
        fail "expected failure"
    fi
    for expected in "${expected_messages[@]}"; do
        assert_contains "${output}" "${expected}"
    done
}

configure_origin() {
    local repository="$1"
    local remote="${repository}-origin.git"

    git init --bare -q "${remote}"
    git -C "${repository}" remote add origin "${remote}"
    git -C "${repository}" push -q origin HEAD:main
}

create_repository() {
    local repository="${test_root:?test suite must set test_root}/$1"

    git init -q "${repository}"
    git -C "${repository}" config user.email "api-compatibility-test@example.com"
    git -C "${repository}" config user.name "API Compatibility Test"
    printf 'initial\n' > "${repository}/fixture"
    git -C "${repository}" add fixture
    git -C "${repository}" commit -qm initial
    configure_origin "${repository}"
    git -C "${repository}" config remote.origin.test-url \
        https://github.com/complytime/complyctl.git
    printf '%s\n' "${repository}"
}

commit_and_tag() {
    local repository="$1" tag="$2" remote="${3:-origin}"

    printf '%s\n' "${tag}" >> "${repository}/fixture"
    git -C "${repository}" add .
    git -C "${repository}" commit -qm "${tag}"
    git -C "${repository}" tag -a -m "${tag}" "${tag}"
    git -C "${repository}" push -q "${remote}" HEAD:main "refs/tags/${tag}"
}
