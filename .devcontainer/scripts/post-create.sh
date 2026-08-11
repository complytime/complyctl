#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# ---------------------------------------------------------------------------
# PATH setup — ensure built binaries and Go-installed tools are available
# ---------------------------------------------------------------------------
export PATH="./bin:${GOPATH:-$(go env GOPATH)}/bin:${PATH}"

# ---------------------------------------------------------------------------
# GITHUB_TOKEN least-privilege: capture, unset, use only for clone
#
# The token is saved in _GITHUB_TOKEN and removed from the environment.
# It is passed to the providers clone (Step 3) via env-var prefix syntax
# which scopes it to that single subprocess. _GITHUB_TOKEN is explicitly
# unset after the clone so it does not leak to later steps.
# ---------------------------------------------------------------------------
_GITHUB_TOKEN="${GITHUB_TOKEN:-}"
unset GITHUB_TOKEN

if [[ -z "${_GITHUB_TOKEN}" ]]; then
    echo "WARNING: GITHUB_TOKEN was not set when the container started."
    echo "  complyctl scan requires a GitHub token to query branch"
    echo "  protection rules via snappy. get and generate work without it."
    echo "  Set it in your shell: export GITHUB_TOKEN=<your-token>"
else
    echo "NOTE: GITHUB_TOKEN was unset from the environment during setup"
    echo "  (least-privilege). Re-export it for scan commands:"
    echo "  export GITHUB_TOKEN=<your-token>"
fi

# ---------------------------------------------------------------------------
# Step 1: Build complyctl and mock-oci-registry
# make build compiles all cmd/ packages (complyctl, mock-oci-registry, etc.)
# ---------------------------------------------------------------------------
echo ">>> Building complyctl and mock-oci-registry..."
make build
echo "    Build complete. Binaries in ./bin/"

# ---------------------------------------------------------------------------
# Step 2: Install snappy, ampel, conftest, and cue
#
# Pinned versions — update these when upgrading:
#   snappy    v0.2.4   https://github.com/carabiner-dev/snappy
#   ampel     v1.2.1   https://github.com/carabiner-dev/ampel
#   conftest  v0.68.2  https://github.com/open-policy-agent/conftest
#   cue       v0.17.1  https://github.com/cue-lang/cue
# ---------------------------------------------------------------------------
echo ">>> Installing snappy, ampel, conftest, and cue..."
go install github.com/carabiner-dev/snappy@v0.2.4
go install github.com/carabiner-dev/ampel/cmd/ampel@v1.2.1
go install github.com/open-policy-agent/conftest@v0.68.2
go install cuelang.org/go/cmd/cue@v0.17.1
echo "    snappy, ampel, conftest, and cue installed."

# ---------------------------------------------------------------------------
# Step 3: Clone and build complytime-providers (all providers)
# ---------------------------------------------------------------------------
echo ">>> Cloning complytime-providers..."
PROVIDERS_TMP="$(mktemp -d)"
trap 'rm -rf "${PROVIDERS_TMP}"' EXIT

# D6: Intentionally unpinned — tracks main for latest provider code.
# The commit SHA is logged below for auditability.
#
# Export the saved token for this clone only — avoids unauthenticated
# rate limits (60 req/hr) in shared CI or rapid devcontainer rebuilds.
# The repo is public so auth isn't required, but authenticated requests
# get 5,000 req/hr which prevents transient failures.
if ! GITHUB_TOKEN="${_GITHUB_TOKEN}" git clone --depth 1 \
        https://github.com/complytime/complytime-providers.git \
        "${PROVIDERS_TMP}/complytime-providers"; then
    echo "FATAL: Failed to clone complytime-providers."
    echo "       This is an upstream dependency required for the dev environment."
    exit 1
fi

PROVIDERS_SHA="$(git -C "${PROVIDERS_TMP}/complytime-providers" \
    rev-parse HEAD)"
echo "    Cloned complytime-providers at ${PROVIDERS_SHA}"

# Token served its purpose — discard it.
unset _GITHUB_TOKEN

echo ">>> Building complytime-providers..."
make -C "${PROVIDERS_TMP}/complytime-providers" build

echo ">>> Installing provider binaries..."
PROVIDER_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/complytime/providers"
mkdir -p "${PROVIDER_DIR}"
for provider in ampel openscap opa; do
    binary="complyctl-provider-${provider}"
    src="${PROVIDERS_TMP}/complytime-providers/bin/${binary}"
    if [[ -f "${src}" ]]; then
        cp "${src}" "${PROVIDER_DIR}/"
        echo "    Installed ${binary}"
    else
        echo "    WARNING: ${binary} not found in build output, skipping."
    fi
done

# ---------------------------------------------------------------------------
# Step 4: Workspace setup — test-workspace with config and policies
# ---------------------------------------------------------------------------
echo ">>> Setting up test workspace..."
mkdir -p "${HOME}/test-workspace/.complytime/ampel/granular-policies"

# Only copy if complytime.yaml doesn't exist yet.
if [[ ! -f "${HOME}/test-workspace/.complytime/complytime.yaml" ]]; then
    cp tests/cross-repo/testdata/complytime.yaml \
        "${HOME}/test-workspace/.complytime/"
fi

cp tests/cross-repo/testdata/granular-policies/block-force-push.json \
    "${HOME}/test-workspace/.complytime/ampel/granular-policies/"

# Generate OPA test deployment input for the test-k8s-deployment target.
# Created inline to avoid shipping a K8s manifest in the repo testdata
# (which triggers Trivy/security scanner false positives).
cat > "${HOME}/test-workspace/test-deployment.yaml" << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
        - name: web
          image: nginx:1.27
          securityContext:
            runAsNonRoot: true
          resources:
            limits:
              cpu: "500m"
              memory: "128Mi"
EOF
echo "    Generated OPA test deployment input"

echo "    Test workspace ready at ~/test-workspace/"

# ---------------------------------------------------------------------------
# Step 4b: Register mounted policy bundles with mock registry (optional)
#
# If a bundles directory exists (default: /bundles/, override via
# COMPLYCTL_BUNDLES_DIR), discover Gemara policy directories and add
# them to complytime.yaml. The mock registry (Step 5) serves these
# files via seedFromDirectory(), so complyctl get populates the cache
# through normal code paths — no cache bypass needed.
#
# Expected bundle format (raw Gemara YAML, not OCI Layout):
#   /bundles/my-policy/catalog.yaml
#   /bundles/my-policy/policy.yaml
# ---------------------------------------------------------------------------
BUNDLES_DIR="${COMPLYCTL_BUNDLES_DIR:-/bundles}"
if [[ -d "${BUNDLES_DIR}" ]]; then
    echo ">>> [optional] Registering mounted policies from ${BUNDLES_DIR}..."
    CONFIG_FILE="${HOME}/test-workspace/.complytime/complytime.yaml"

    BUNDLE_COUNT=0
    NEW_BUNDLES_ADDED=false
    for bundle_dir in "${BUNDLES_DIR}"/*/; do
        # Guard against glob matching nothing
        [[ -d "${bundle_dir}" ]] || continue

        bundle_name="$(basename "${bundle_dir}")"

        # Validate bundle name: only alphanumeric, hyphens, underscores
        if [[ ! "${bundle_name}" =~ ^[a-zA-Z0-9_-]+$ ]]; then
            echo "    Skipping ${bundle_name} (invalid characters in name)"
            continue
        fi

        # Verify required Gemara YAML files exist
        if [[ ! -f "${bundle_dir}/catalog.yaml" ]]; then
            echo "    Skipping ${bundle_name} (no catalog.yaml)"
            continue
        fi
        if [[ ! -f "${bundle_dir}/policy.yaml" ]]; then
            echo "    Skipping ${bundle_name} (no policy.yaml)"
            continue
        fi

        # Insert policy entry into complytime.yaml if not already present.
        # Points at the mock registry (localhost:8765) which serves files
        # from the bundles directory via seedFromDirectory().
        if ! grep -q "http://localhost:8765/policies/${bundle_name}" "${CONFIG_FILE}" 2>/dev/null; then
            # Insert before the first 'targets:' line to stay in policies block.
            if grep -q "^targets:" "${CONFIG_FILE}" 2>/dev/null; then
                awk -v name="${bundle_name}" '
                    /^targets:/ {
                        print "  - url: http://localhost:8765/policies/" name
                        print "    id: " name
                    }
                    { print }
                ' "${CONFIG_FILE}" > "${CONFIG_FILE}.tmp"
                mv "${CONFIG_FILE}.tmp" "${CONFIG_FILE}"
            else
                printf '  - url: http://localhost:8765/policies/%s\n    id: %s\n' \
                    "${bundle_name}" "${bundle_name}" >> "${CONFIG_FILE}"
            fi
            echo "    Added ${bundle_name} to complytime.yaml"
            NEW_BUNDLES_ADDED=true
        else
            echo "    ${bundle_name} already in complytime.yaml, skipping."
        fi

        BUNDLE_COUNT=$((BUNDLE_COUNT + 1))
    done

    if [[ ${BUNDLE_COUNT} -gt 0 ]]; then
        echo "    Registered ${BUNDLE_COUNT} bundle(s) for mock registry."
        echo "    After registry starts, use: complyctl get && complyctl generate"
    else
        echo "    No valid Gemara policy directories found in ${BUNDLES_DIR}."
    fi
else
    echo ">>> No bundles directory at ${BUNDLES_DIR}, skipping policy"
    echo "    registration. Mount Gemara YAML policies at /bundles/ or"
    echo "    set COMPLYCTL_BUNDLES_DIR to enable."
fi

# ---------------------------------------------------------------------------
# Step 5: Start mock OCI registry
# ---------------------------------------------------------------------------
SKIP_REGISTRY_START=false
if curl -sf http://localhost:8765/v2/ > /dev/null 2>&1; then
    # Registry is running - check if we need to restart because new bundles have been added
    if [[ "${NEW_BUNDLES_ADDED:-false}" == "true" ]]; then
        echo ">>> New bundles detected. Restarting mock OCI registry to re-seed..."
        pkill mock-oci-registry || true
        for _ in {1..10}; do
            curl -sf http://localhost:8765/v2/ > /dev/null 2>&1 || break
            sleep 0.5
        done
        # Fall through to start registry with new bundles
    else
        echo ">>> Mock OCI registry already running on port 8765 (no new bundles)."
        SKIP_REGISTRY_START=true
    fi
fi

if [[ "${SKIP_REGISTRY_START}" != "true" ]]; then
    echo ">>> Starting mock OCI registry..."
    MOCK_REGISTRY_CONTENT_DIR="${COMPLYCTL_BUNDLES_DIR:-/bundles}" \
        nohup ./bin/mock-oci-registry > /tmp/mock-oci-registry.log 2>&1 &
    REGISTRY_PID=$!
    disown ${REGISTRY_PID}

    RETRIES=0
    MAX_RETRIES=30
    until curl -sf http://localhost:8765/v2/ > /dev/null 2>&1; do
        RETRIES=$((RETRIES + 1))
        if [[ ${RETRIES} -ge ${MAX_RETRIES} ]]; then
            echo "FATAL: Mock OCI registry failed to start after ${MAX_RETRIES} retries."
            exit 1
        fi
        sleep 0.5
    done

    echo "    Mock OCI registry running (PID: ${REGISTRY_PID}, port: 8765)"
fi

# ---------------------------------------------------------------------------
# Step 6: Record build commit for auto-rebuild detection
# Non-fatal: fails gracefully when run outside a git repo (e.g. docker run
# with a mounted worktree whose .git pointer resolves outside the container).
# ---------------------------------------------------------------------------
if git rev-parse HEAD > ./bin/.build-commit 2>/dev/null; then
    echo "    Recorded build commit: $(cat ./bin/.build-commit)"
else
    echo "unknown" > ./bin/.build-commit
    echo "    WARNING: Not a git repo — build commit set to 'unknown'."
    echo "             Auto-rebuild detection will trigger on first shell login."
fi

# ---------------------------------------------------------------------------
# Step 7: Persist PATH and auto-rebuild hook for interactive shells
# ---------------------------------------------------------------------------
REPO_ROOT="$(pwd)"
if ! grep -q "complyctl dev environment" "${HOME}/.bashrc" 2>/dev/null; then
    cat >> "${HOME}/.bashrc" << 'BASHRC'

# complyctl dev environment — added by post-create.sh
export PATH="REPO_ROOT_PLACEHOLDER/bin:${GOPATH:-$(go env GOPATH)}/bin:${PATH}"

# Auto-rebuild complyctl when source has changed (e.g., after
# checking out a PR branch). Skip with: export COMPLYCTL_SKIP_REBUILD=1
if [[ -z "${COMPLYCTL_SKIP_REBUILD:-}" ]]; then
    _repo="REPO_ROOT_PLACEHOLDER"
    _build_commit=""
    if [[ -f "${_repo}/bin/.build-commit" ]]; then
        _build_commit="$(cat "${_repo}/bin/.build-commit")"
    fi
    _head="$(git -C "${_repo}" rev-parse HEAD 2>/dev/null || true)"
    if [[ -n "${_head}" && "${_head}" != "${_build_commit}" ]]; then
        echo ">>> Source changed (${_build_commit:0:8}..${_head:0:8}), rebuilding complyctl..."
        if make -C "${_repo}" build 2>&1; then
            echo "${_head}" > "${_repo}/bin/.build-commit"
            echo "    Rebuild complete."
        else
            echo "    WARNING: Rebuild failed. Run 'make build' manually."
        fi
    fi
    unset _repo _build_commit _head
fi
BASHRC
    # Replace placeholder with actual repo root path
    sed -i "s|REPO_ROOT_PLACEHOLDER|${REPO_ROOT}|g" "${HOME}/.bashrc"
fi

echo ">>> Dev environment ready."
echo "    Test workspace: ~/test-workspace/"
echo "    Run: cd ~/test-workspace && complyctl get"
