#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Focused compatibility tests for the pinned protobuf and Go API analyzer scripts.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
readonly REPO_ROOT
readonly CHECK_SCRIPT="${REPO_ROOT}/scripts/api-compatibility-check.sh"
readonly GO_SCRIPT="${REPO_ROOT}/scripts/api-compatibility-go.sh"
readonly MAKEFILE="${REPO_ROOT}/Makefile"
readonly PROTO_SCRIPT="${REPO_ROOT}/scripts/api-compatibility-proto.sh"

test_root="$(mktemp -d)"
trap 'rm -rf "${test_root}"' EXIT

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
    local expected output status
    local -a expected_messages=()

    while [[ "$1" != "--" ]]; do
        expected_messages+=("$1")
        shift
    done
    shift

    set +e
    output="$("$@" 2>&1)"
    status=$?
    set -e
    [[ ${status} -ne 0 ]] || fail "expected failure"
    for expected in "${expected_messages[@]}"; do
        assert_contains "${output}" "${expected}"
    done
}

configure_origin() {
    local repository="$1" remote="${repository}-origin.git"

    git init --bare -q "${remote}"
    git -C "${repository}" remote add origin "${remote}"
    git -C "${repository}" push -q origin HEAD:main
}

write_buf_config() {
    local directory="$1"

    mkdir -p "${directory}/api/plugin"
    cat > "${directory}/buf.yaml" <<'EOF'
version: v2
modules:
  - path: api/plugin
breaking:
  use:
    - FILE
EOF
}

write_provider_package() {
    local directory="$1" source="$2"

    mkdir -p "${directory}/pkg/provider"
    cat > "${directory}/go.mod" <<'EOF'
module github.com/complytime/complyctl

go 1.26.10
EOF
    printf '%s\n' "${source}" > "${directory}/pkg/provider/provider.go"
}

test_protobuf_compatibility() {
    local baseline="${test_root}/proto-baseline" candidate="${test_root}/proto-candidate"

    write_buf_config "${baseline}"
    write_buf_config "${candidate}"
    printf '%s\n' 'syntax = "proto3"; package fixture.v1; message Contract { string name = 1; }' \
        > "${baseline}/api/plugin/contract.proto"
    printf '%s\n' 'syntax = "proto3"; package fixture.v1; message Contract {' \
        'string name = 1; string location = 2; }' > "${candidate}/api/plugin/contract.proto"
    assert_success "${PROTO_SCRIPT}" "${candidate}" "${baseline}"

    printf '%s\n' 'syntax = "proto3"; package fixture.v1; message Contract {}' \
        > "${candidate}/api/plugin/contract.proto"
    # Buf emits the source path with its breaking diagnostic. Keep that output
    # visible so a CI failure identifies the contract that needs remediation.
    assert_failure 'incompatible protobuf contract detected' 'contract.proto' -- \
        "${PROTO_SCRIPT}" "${candidate}" "${baseline}"
}

test_go_api_compatibility() {
    local baseline="${test_root}/go-baseline"
    local candidate="${test_root}/go-candidate"

    write_provider_package "${baseline}" $'package provider\n\nfunc Stable() {}'
    write_provider_package "${candidate}" \
        $'package provider\n\nfunc Stable() {}\n\nfunc Added() {}'
    assert_success "${GO_SCRIPT}" "${candidate}" "${baseline}"

    write_provider_package "${candidate}" 'package provider'
    # apidiff emits the incompatible exported symbol before the summary error.
    assert_failure 'incompatible Go API detected' 'Stable' -- \
        "${GO_SCRIPT}" "${candidate}" "${baseline}"
}

test_operational_failures_are_classified() {
    local baseline="${test_root}/failure-baseline"
    local candidate="${test_root}/failure-candidate"
    local fake_bin="${test_root}/fake-bin"

    write_buf_config "${baseline}"
    write_buf_config "${candidate}"
    printf '%s\n' 'syntax = "proto3"; package fixture.v1; message Contract {}' \
        > "${baseline}/api/plugin/contract.proto"
    cp "${baseline}/api/plugin/contract.proto" "${candidate}/api/plugin/contract.proto"
    mkdir "${fake_bin}"
    cat > "${fake_bin}/buf" <<'EOF'
#!/usr/bin/env bash
printf '0.0.0\n'
EOF
    chmod +x "${fake_bin}/buf"
    assert_failure 'tool setup: expected buf 1.50.0, found 0.0.0' -- \
        env PATH="${fake_bin}:${PATH}" "${PROTO_SCRIPT}" "${candidate}" "${baseline}"

    write_provider_package "${baseline}" $'package provider\n\nfunc Stable() {}'
    write_provider_package "${candidate}" 'package provider'
    printf '%s\n' 'func {' >> "${candidate}/pkg/provider/provider.go"
    assert_failure 'package loading: cannot load candidate' -- \
        "${GO_SCRIPT}" "${candidate}" "${baseline}"
}

test_combined_check_uses_one_baseline() {
    local repository="${test_root}/combined" output output_without_baseline

    write_buf_config "${repository}"
    write_provider_package "${repository}" $'package provider\n\nfunc Stable() {}'
    printf '%s\n' 'syntax = "proto3"; package fixture.v1; message Contract { string name = 1; }' \
        > "${repository}/api/plugin/contract.proto"
    git init -q "${repository}"
    git -C "${repository}" config user.email "analyzer-test@example.com"
    git -C "${repository}" config user.name "Analyzer Test"
    git -C "${repository}" add .
    git -C "${repository}" commit -qm baseline
    git -C "${repository}" tag -a -m v1.0.0 v1.0.0
    configure_origin "${repository}"
    git -C "${repository}" push -q origin refs/tags/v1.0.0

    write_provider_package "${repository}" \
        $'package provider\n\nfunc Stable() {}\n\nfunc Added() {}'
    printf '%s\n' 'syntax = "proto3"; package fixture.v1; message Contract {' \
        'string name = 1; string location = 2; }' > "${repository}/api/plugin/contract.proto"
    output="$(API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 "${CHECK_SCRIPT}" --repo "${repository}")"
    assert_contains "${output}" 'Selected baseline tag: v1.0.0'
    [[ "${output}" == *'Selected baseline commit:'* ]] || fail 'expected baseline commit output'
    output_without_baseline="${output/Selected baseline tag: v1.0.0/}"
    [[ "${output_without_baseline}" != *'Selected baseline tag:'* ]] ||
        fail 'expected exactly one baseline invocation'
}

test_make_targets_fail_closed() {
    local repository="${test_root}/make-targets" target

    write_buf_config "${repository}"
    write_provider_package "${repository}" $'package provider\n\nfunc Stable() {}'
    printf '%s\n' 'syntax = "proto3"; package fixture.v1; message Contract { string name = 1; }' \
        > "${repository}/api/plugin/contract.proto"
    git init -q "${repository}"
    git -C "${repository}" config user.email "make-target-test@example.com"
    git -C "${repository}" config user.name "Make Target Test"
    git -C "${repository}" add .
    git -C "${repository}" commit -qm baseline
    configure_origin "${repository}"

    for target in api-compatibility-proto api-compatibility-go api-compatibility-analysis; do
        assert_failure 'baseline resolution: no stable v1 baseline tag is available' -- \
            env API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 make -s -f "${MAKEFILE}" \
            "API_COMPATIBILITY_REPO=${repository}" "${target}"
    done

    git -C "${repository}" tag -a -m v1.0.0 v1.0.0
    git -C "${repository}" push -q origin refs/tags/v1.0.0
    assert_failure 'protobuf compatibility: tool setup: buf is not installed' -- \
        env PATH=/usr/bin:/bin API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 make -s -f "${MAKEFILE}" \
            "API_COMPATIBILITY_REPO=${repository}" api-compatibility-proto
    assert_failure 'Go API compatibility: tool setup: apidiff is not installed' -- \
        env PATH=/usr/bin:/bin API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 make -s -f "${MAKEFILE}" \
            "API_COMPATIBILITY_REPO=${repository}" api-compatibility-go
    assert_failure 'protobuf compatibility: tool setup: buf is not installed' -- \
        env PATH=/usr/bin:/bin API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 make -s -f "${MAKEFILE}" \
            "API_COMPATIBILITY_REPO=${repository}" api-compatibility-analysis
}

test_protobuf_compatibility
test_go_api_compatibility
test_operational_failures_are_classified
test_combined_check_uses_one_baseline
test_make_targets_fail_closed
printf 'PASS: API compatibility analyzer tests\n'
