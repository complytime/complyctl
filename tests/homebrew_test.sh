#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Verify the Homebrew formula builds and installs complyctl from
# the current branch. Requires Homebrew (brew) on the host.
#
# Uses a local tap (homebrew-complyctl-test) because Homebrew
# rejects standalone .rb files outside of taps.
#
# Run locally:  make test-homebrew
# Run directly: ./tests/homebrew_test.sh [branch]

set -euo pipefail

# Fail early if brew is not available (the Makefile also checks,
# but this script can be invoked directly).
if ! command -v brew >/dev/null 2>&1; then
    echo "ERROR: brew not found. Install Homebrew first: https://brew.sh"
    exit 1
fi

BRANCH="${1:-$(git rev-parse --abbrev-ref HEAD)}"
TAP_NAME="complyctl-test/test"

# Validate branch name to prevent injection into Ruby heredoc.
if [[ ! "${BRANCH}" =~ ^[a-zA-Z0-9/_.-]+$ ]]; then
    echo "ERROR: Branch name '${BRANCH}' contains unsupported characters"
    exit 1
fi

# Resolve the git remote URL for the current branch so we clone from
# the correct fork (not always upstream where the branch may not exist).
TRACKING_REMOTE="$(git config "branch.${BRANCH}.remote" 2>/dev/null || echo "origin")"
REPO_URL="$(git remote get-url "${TRACKING_REMOTE}" 2>/dev/null || git remote get-url origin)"
# Ensure HTTPS URL for Homebrew (it does not support git@ SSH URLs).
REPO_URL="${REPO_URL/git@github.com:/https://github.com/}"
REPO_URL="${REPO_URL%.git}.git"

# Warn if complyctl is already installed via Homebrew.
if brew list complyctl &>/dev/null; then
    echo "WARNING: complyctl is already installed via Homebrew."
    echo "This test will uninstall it. Re-install after with: brew install complytime/tap/complyctl"
fi

cleanup() {
    brew uninstall complyctl 2>/dev/null || true
    brew cleanup complyctl 2>/dev/null || true
    brew untap "${TAP_NAME}" 2>/dev/null || true
}
trap cleanup EXIT

# Create a local tap with the Formula.
TAP_DIR="$(brew --repository)/Library/Taps/complyctl-test/homebrew-test"
mkdir -p "${TAP_DIR}/Formula"

# Initialize a git repo in the tap (Homebrew requires taps to be git repos).
if [ ! -d "${TAP_DIR}/.git" ]; then
    git -C "${TAP_DIR}" init -q
fi

cat > "${TAP_DIR}/Formula/complyctl.rb" <<RUBY
class Complyctl < Formula
  desc "CLI for streamlining end-to-end compliance workflows"
  homepage "https://github.com/complytime/complyctl"
  head "${REPO_URL}", branch: "${BRANCH}"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -X github.com/complytime/complyctl/internal/version.version=test
      -X github.com/complytime/complyctl/internal/version.buildDate=#{time.iso8601}
      -X github.com/complytime/complyctl/internal/version.gitTreeState=clean
    ]
    system "go", "build", *std_go_args(ldflags:), "./cmd/complyctl"
    generate_completions_from_executable(bin/"complyctl", "completion")
  end

  test do
    assert_match "Version:", shell_output("#{bin}/complyctl version 2>&1")
  end
end
RUBY

# Commit the formula so Homebrew recognizes the tap.
git -C "${TAP_DIR}" add -A
git -C "${TAP_DIR}" commit -q -m "add complyctl formula" --allow-empty 2>/dev/null || true

echo "=== Installing complyctl from branch '${BRANCH}' via Homebrew ==="
HOMEBREW_NO_AUTO_UPDATE=1 brew install --HEAD "${TAP_NAME}/complyctl"

echo "=== Verifying installation ==="
VERSION_OUTPUT="$(complyctl version 2>/dev/null)"
echo "${VERSION_OUTPUT}"
if ! echo "${VERSION_OUTPUT}" | grep -q "Version:"; then
    echo "FAIL: version output does not contain version information"
    exit 1
fi

echo "=== Running brew test ==="
brew test complyctl

echo "=== PASS: Homebrew formula validated successfully ==="
