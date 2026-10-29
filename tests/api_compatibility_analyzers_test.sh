#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Focused compatibility tests for the protobuf and Go API analyzer scripts.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
readonly REPO_ROOT
readonly CHECK_SCRIPT="${REPO_ROOT}/scripts/api-compatibility-check.sh"
readonly GO_SCRIPT="${REPO_ROOT}/scripts/api-compatibility-go.sh"
readonly MAKEFILE="${REPO_ROOT}/Makefile"
readonly PROTO_SCRIPT="${REPO_ROOT}/scripts/api-compatibility-proto.sh"

# shellcheck source=tests/api_compat_helpers.sh
source "${REPO_ROOT}/tests/api_compat_helpers.sh"

test_root="$(mktemp -d)"
trap 'rm -rf "${test_root}"' EXIT

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
    local baseline="${test_root}/go-baseline" candidate="${test_root}/go-candidate" workspace

    write_provider_package "${baseline}" $'package provider\n\nfunc Stable() {}'
    write_provider_package "${candidate}" \
        $'package provider\n\nfunc Stable() {}\n\nfunc Added() {}'
    assert_success "${GO_SCRIPT}" "${candidate}" "${baseline}"

    write_provider_package "${candidate}" 'package provider'
    # apidiff emits the incompatible exported symbol before the summary error.
    assert_failure 'incompatible Go API detected' 'Stable' -- \
        "${GO_SCRIPT}" "${candidate}" "${baseline}"

    mkdir "${test_root}/go-workspace"
    workspace="${test_root}/go-workspace/go.work"
    (
        cd "${test_root}/go-workspace"
        go work init "${candidate}"
    )
    assert_failure 'incompatible Go API detected' 'Stable' -- \
        env GOWORK="${workspace}" "${GO_SCRIPT}" "${candidate}" "${baseline}"
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
    assert_failure 'tool setup: buf >= 1.50.0 required, found 0.0.0' -- \
        env PATH="${fake_bin}:${PATH}" "${PROTO_SCRIPT}" "${candidate}" "${baseline}"

    write_provider_package "${baseline}" $'package provider\n\nfunc Stable() {}'
    write_provider_package "${candidate}" 'package provider'
    printf '%s\n' 'func {' >> "${candidate}/pkg/provider/provider.go"
    assert_failure 'package loading: cannot load candidate' -- \
        "${GO_SCRIPT}" "${candidate}" "${baseline}"
}

test_combined_check_uses_one_baseline() {
    local repository output output_without_baseline
    local -x API_COMPATIBILITY_REMOTE=origin

    repository="$(create_repository combined)"
    write_buf_config "${repository}"
    write_provider_package "${repository}" $'package provider\n\nfunc Stable() {}'
    printf '%s\n' 'syntax = "proto3"; package fixture.v1; message Contract { string name = 1; }' \
        > "${repository}/api/plugin/contract.proto"
    commit_and_tag "${repository}" v1.0.0

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
    local repository target
    local -x API_COMPATIBILITY_REMOTE=origin

    repository="$(create_repository make-targets)"
    write_buf_config "${repository}"
    write_provider_package "${repository}" $'package provider\n\nfunc Stable() {}'
    printf '%s\n' 'syntax = "proto3"; package fixture.v1; message Contract { string name = 1; }' \
        > "${repository}/api/plugin/contract.proto"

    for target in api-compatibility-proto api-compatibility-go api-compatibility-analysis; do
        assert_failure 'baseline resolution: no stable v1 baseline tag is available' -- \
            env API_COMPATIBILITY_CANDIDATE_VERSION=v1.1.0 make -s -f "${MAKEFILE}" \
            "API_COMPATIBILITY_REPO=${repository}" "${target}"
    done

    commit_and_tag "${repository}" v1.0.0
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

test_buf_version_validation() {
    local baseline="${test_root}/version-baseline" candidate="${test_root}/version-candidate"
    local fake_bin="${test_root}/version-bin" marker="${test_root}/buf-analysis" version

    write_buf_config "${baseline}"
    write_buf_config "${candidate}"
    mkdir "${fake_bin}"
    cat > "${fake_bin}/buf" <<'EOF'
#!/usr/bin/env bash
if [[ "$1" == --version ]]; then
    printf '%s\n' "${API_TEST_BUF_VERSION-}"
    exit "${API_TEST_BUF_VERSION_STATUS:-0}"
fi
printf '%s\n' "$@" > "${API_TEST_BUF_MARKER}"
exit "${API_TEST_BUF_ANALYSIS_STATUS:-0}"
EOF
    chmod +x "${fake_bin}/buf"

    for version in 1.50.0 1.50.1 1.73.0 1.100.0 2.0.0 1.50.0+build.001 \
        999999999999999999999999.0.0; do
        rm -f "${marker}"
        assert_success env PATH="${fake_bin}:${PATH}" API_TEST_BUF_VERSION="${version}" \
            API_TEST_BUF_MARKER="${marker}" "${PROTO_SCRIPT}" "${candidate}" "${baseline}"
        [[ -f "${marker}" ]] || fail "analysis did not run for supported Buf ${version}"
        assert_contains "$(<"${marker}")" 'breaking'
    done

    for version in 0.99.0 1.9.99 1.49.0 1.49.999999999999999999999999 \
        '' 1.73 01.73.0 1.073.0 1.73.00 v1.73.0 1.73.0-rc.1 \
        1.73.0+ 1.73.0+bad..metadata '1.73.0 garbage' "\$(touch unexpected)" \
        $'1.73.0\n2.0.0'; do
        rm -f "${marker}"
        assert_failure "buf >= 1.50.0 required, found ${version}" -- \
            env PATH="${fake_bin}:${PATH}" API_TEST_BUF_VERSION="${version}" \
            API_TEST_BUF_MARKER="${marker}" "${PROTO_SCRIPT}" "${candidate}" "${baseline}"
        [[ ! -e "${marker}" ]] || fail "analysis ran for unsupported Buf ${version}"
    done

    assert_failure 'tool setup: cannot determine Buf version' -- \
        env PATH="${fake_bin}:${PATH}" API_TEST_BUF_VERSION=1.73.0 API_TEST_BUF_VERSION_STATUS=1 \
        API_TEST_BUF_MARKER="${marker}" "${PROTO_SCRIPT}" "${candidate}" "${baseline}"
    [[ ! -e "${marker}" ]] || fail 'analysis ran after the version command failed'

    assert_failure 'incompatible protobuf contract detected' -- \
        env PATH="${fake_bin}:${PATH}" API_TEST_BUF_VERSION=1.73.0 API_TEST_BUF_ANALYSIS_STATUS=100 \
        API_TEST_BUF_MARKER="${marker}" "${PROTO_SCRIPT}" "${candidate}" "${baseline}"
    assert_failure 'analysis: buf breaking failed with exit status 7' -- \
        env PATH="${fake_bin}:${PATH}" API_TEST_BUF_VERSION=1.73.0 API_TEST_BUF_ANALYSIS_STATUS=7 \
        API_TEST_BUF_MARKER="${marker}" "${PROTO_SCRIPT}" "${candidate}" "${baseline}"
}

test_buf_version_validation
test_protobuf_compatibility
test_go_api_compatibility
test_operational_failures_are_classified
test_combined_check_uses_one_baseline
test_make_targets_fail_closed
printf 'PASS: API compatibility analyzer tests\n'
