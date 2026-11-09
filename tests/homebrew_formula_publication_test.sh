#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Focused regression test for scoped Homebrew formula push credentials.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
readonly REPO_ROOT
readonly WORKFLOW_PATH="${REPO_ROOT}/.github/workflows/release.yml"
readonly SYNTHETIC_GH_TOKEN="synthetic-homebrew-token"
readonly SETUP_GIT_COMMAND="GH_TOKEN=\"\${TAP_TOKEN}\" gh auth setup-git"
readonly PUSH_COMMAND="GH_TOKEN=\"\${TAP_TOKEN}\" git push origin \"\${BRANCH}\""

assert_workflow_uses_scoped_push_token() {
    local workflow

    workflow="$(<"${WORKFLOW_PATH}")"
    if [[ "${workflow}" != *"${SETUP_GIT_COMMAND}"* ]]; then
        printf 'FAIL: Homebrew workflow must retain gh auth setup-git\n' >&2
        return 1
    fi

    if [[ "${workflow}" != *"${PUSH_COMMAND}"* ]]; then
        printf 'FAIL: Homebrew workflow must scope GH_TOKEN to git push\n' >&2
        return 1
    fi
}

write_git_fixture() {
    local fixture_directory="$1"

    mkdir -p "${fixture_directory}/bin"
    cat > "${fixture_directory}/bin/git" <<'GIT'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$1" != "push" ]]; then
    printf 'unexpected git command: %s\n' "$1" >&2
    exit 1
fi

"${GIT_CREDENTIAL_HELPER}" get <<<'protocol=https'
GIT
    cat > "${fixture_directory}/credential-helper" <<'HELPER'
#!/usr/bin/env bash
set -euo pipefail

read -r request
if [[ "${request}" != "protocol=https" ]] || [[ "${GH_TOKEN:-}" != "${EXPECTED_GH_TOKEN}" ]]; then
    exit 1
fi

touch "${HELPER_CALLED_PATH}"
HELPER
    cat > "${fixture_directory}/bin/gh" <<'GH'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$1 $2" != "auth setup-git" ]] || [[ "${GH_TOKEN:-}" != "${EXPECTED_GH_TOKEN}" ]]; then
    exit 1
fi
GH
    chmod +x "${fixture_directory}/bin/git" "${fixture_directory}/bin/gh" \
        "${fixture_directory}/credential-helper"
}

assert_token_is_not_written_to_fixture() {
    local fixture_directory="$1"

    if grep --recursive --fixed-strings --quiet "${SYNTHETIC_GH_TOKEN}" "${fixture_directory}"; then
        printf 'FAIL: synthetic token was written to the test fixture\n' >&2
        return 1
    fi
}

assert_push_scopes_token_to_credential_helper() {
    local fixture_directory

    fixture_directory="$(mktemp -d)"
    trap 'rm -rf "${fixture_directory}"' RETURN
    write_git_fixture "${fixture_directory}"

    PATH="${fixture_directory}/bin:${PATH}" \
        GIT_CREDENTIAL_HELPER="${fixture_directory}/credential-helper" \
        EXPECTED_GH_TOKEN="${SYNTHETIC_GH_TOKEN}" \
        HELPER_CALLED_PATH="${fixture_directory}/helper-called" \
        GH_TOKEN="${SYNTHETIC_GH_TOKEN}" \
        git push origin formula/test

    test -f "${fixture_directory}/helper-called"
    assert_token_is_not_written_to_fixture "${fixture_directory}"
}

assert_unscoped_push_fails() {
    local fixture_directory

    fixture_directory="$(mktemp -d)"
    trap 'rm -rf "${fixture_directory}"' RETURN
    write_git_fixture "${fixture_directory}"

    PATH="${fixture_directory}/bin:${PATH}" \
        EXPECTED_GH_TOKEN="${SYNTHETIC_GH_TOKEN}" \
        GH_TOKEN="${SYNTHETIC_GH_TOKEN}" \
        gh auth setup-git

    if PATH="${fixture_directory}/bin:${PATH}" \
        GIT_CREDENTIAL_HELPER="${fixture_directory}/credential-helper" \
        EXPECTED_GH_TOKEN="${SYNTHETIC_GH_TOKEN}" \
        HELPER_CALLED_PATH="${fixture_directory}/helper-called" \
        env -u GH_TOKEN git push origin formula/test; then
        printf 'FAIL: unscoped git push unexpectedly authenticated\n' >&2
        return 1
    fi

    if [[ -e "${fixture_directory}/helper-called" ]]; then
        printf 'FAIL: credential helper accepted an unscoped token\n' >&2
        return 1
    fi
    assert_token_is_not_written_to_fixture "${fixture_directory}"
}

assert_workflow_uses_scoped_push_token
assert_push_scopes_token_to_credential_helper
assert_unscoped_push_fails
printf 'PASS: Homebrew formula publication credential scoping\n'
