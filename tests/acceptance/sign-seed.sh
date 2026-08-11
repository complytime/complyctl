#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

# sign-seed.sh — push Gemara policy artifacts to zot and sign them
# with cosign (keyed and keyless) for verification acceptance tests.
# All credentials in this script are test-only mock values with no
# security value.
#
# Keyless signing is performed against the private Sigstore stack
# (dex/fulcio/rekor) and a trusted_root.json is emitted to the shared
# volume for the verify-sut container to consume.

REGISTRY="${REGISTRY_URL:-zot:5000}"

# --- idempotency guard ------------------------------------------------------
# podman-compose 1.6.0 can invoke this entrypoint more than once for the same
# container (a `podman start` on the already-exited container re-runs the
# entrypoint). This script is destructive at its start (it clears the shared
# outputs it regenerates) and cosign's key generation is interactive on a
# pre-existing key, so a naive second run would delete run #1's good artifacts
# and then abort on cosign's "File cosign.key already exists" prompt — leaving
# verify-sut with no artifacts. Guard against that: if the completion sentinel
# is already present, a prior invocation in THIS run finished successfully, so
# exit 0 without touching anything. This is safe against the historical
# stale-sentinel race because the Makefile removes the named shared volume
# before every run, so any sentinel seen here was written by this run.
if [[ -f /shared/.sign-seed-complete ]]; then
	echo "Sign-seed already complete for this run (sentinel present); skipping."
	exit 0
fi

# --- helpers ----------------------------------------------------------------

retry() {
	local max_attempts=3
	local delay=2
	local attempt=1
	local cmd=("$@")

	# Progress messages go to stderr so callers can safely capture the
	# wrapped command's stdout, e.g. `id_token="$(retry curl ... | jq ...)"`.
	while [[ ${attempt} -le ${max_attempts} ]]; do
		echo "  attempt ${attempt}/${max_attempts}: ${cmd[*]}" >&2
		if "${cmd[@]}"; then
			return 0
		fi
		if [[ ${attempt} -eq ${max_attempts} ]]; then
			echo "ERROR: command failed after ${max_attempts} attempts: ${cmd[*]}" >&2
			return 1
		fi
		echo "  retrying in ${delay}s..." >&2
		sleep "${delay}"
		attempt=$((attempt + 1))
	done
}

verify_push() {
	local repo="$1"
	local tag="$2"
	local tags
	tags=$(oras repo tags --plain-http "${REGISTRY}/${repo}")
	if ! echo "${tags}" | grep -q "${tag}"; then
		echo "ERROR: tag ${tag} not found in ${repo} after push." >&2
		exit 1
	fi
	echo "  verified: ${repo}:${tag}"
}

# verify_signature REPO — assert cosign persisted the sha256-<digest>.sig
# tag for the given repo's :v1.0.0 manifest.  cosign's retry wrapper can
# report success even when the signature manifest did not land in the
# registry; without this check a silent push failure surfaces later as a
# confusing MANIFEST_UNKNOWN from the verifier instead of a loud sign-seed
# abort here.
verify_signature() {
	local repo="$1"
	local digest sig_tag tags
	digest=$(oras manifest fetch --plain-http --descriptor \
		"${REGISTRY}/${repo}:v1.0.0" | jq -r '.digest')
	if [[ -z "${digest}" || "${digest}" == "null" ]]; then
		echo "ERROR: could not resolve manifest digest for ${repo}." >&2
		exit 1
	fi
	sig_tag="sha256-${digest#sha256:}.sig"
	tags=$(oras repo tags --plain-http "${REGISTRY}/${repo}")
	if ! echo "${tags}" | grep -q "${sig_tag}"; then
		echo "ERROR: cosign signature ${sig_tag} not found in ${repo}" \
			"after signing." >&2
		exit 1
	fi
	echo "  signature verified: ${repo} -> ${sig_tag}"
}

# manifest_digest REPO — print the registry-stored digest of REPO:v1.0.0.
# cosign must sign by this digest (not the :v1.0.0 tag): oras stamps a
# wall-clock org.opencontainers.image.created annotation on every push, and
# signing by tag makes cosign resolve/re-serialize a manifest whose digest can
# diverge from the stored bytes, so the sha256-<digest>.sig tag it writes no
# longer matches the digest the verifier resolves — surfacing as MANIFEST_UNKNOWN.
manifest_digest() {
	local repo="$1"
	local digest
	digest=$(oras manifest fetch --plain-http --descriptor \
		"${REGISTRY}/${repo}:v1.0.0" | jq -r '.digest')
	if [[ -z "${digest}" || "${digest}" == "null" ]]; then
		echo "ERROR: could not resolve manifest digest for ${repo}." >&2
		exit 1
	fi
	echo "${digest}"
}

# wait_http URL DESCRIPTION — poll an HTTP endpoint until it responds 2xx/3xx.
# podman-compose ignores `depends_on: condition`, so poll defensively.
wait_http() {
	local url="$1"
	local desc="$2"
	local max=60
	local i=0
	echo "Waiting for ${desc} at ${url}..."
	while [[ "${i}" -lt "${max}" ]]; do
		if curl -fsS -o /dev/null "${url}"; then
			echo "${desc} is ready."
			return 0
		fi
		i=$((i + 1))
		sleep 2
	done
	echo "ERROR: ${desc} not ready at ${url} after $((max * 2))s" >&2
	return 1
}

# --- clear stale shared artifacts -------------------------------------------
# The shared volume can survive a prior run: podman-compose 1.6.0 does not
# reliably remove named volumes on `down -v`, so a previous run's
# .sign-seed-complete sentinel (and public keys / trusted root) may still be
# present when this container starts. verify-sut gates on that sentinel, so a
# stale one lets the test suite start before this run has finished signing,
# producing MANIFEST_UNKNOWN for the not-yet-pushed .sig artifacts. Remove the
# sentinel FIRST (before any producer output) so the gate is only satisfied by
# this run, then clear the remaining outputs we regenerate below.
echo "Clearing stale shared artifacts from /shared..."
rm -f /shared/.sign-seed-complete
rm -f /shared/cosign.pub /shared/wrong.pub /shared/trusted_root.json

# --- wait for registry ------------------------------------------------------

echo "Waiting for registry at ${REGISTRY}..."
for i in $(seq 1 30); do
	if oras repo list --plain-http "${REGISTRY}" >/dev/null 2>&1; then
		echo "Registry is ready."
		break
	fi
	if [[ "$i" -eq 30 ]]; then
		echo "ERROR: Registry not ready after 30 attempts." >&2
		exit 1
	fi
	sleep 1
done

# --- push artifacts to repository paths --------------------------------------

cd /testdata

for repo in policies/verify-keyed policies/verify-unsigned policies/verify-keyless; do
	echo "Pushing Gemara policy bundle to ${repo}..."
	oras push --plain-http "${REGISTRY}/${repo}:v1.0.0" \
		catalog.yaml:application/vnd.gemara.catalog.v1+yaml \
		policy.yaml:application/vnd.gemara.policy.v1+yaml

	oras tag --plain-http "${REGISTRY}/${repo}:v1.0.0" latest
	verify_push "${repo}" "v1.0.0"
	verify_push "${repo}" "latest"
done

echo "All artifacts pushed."

# --- keyed signing -----------------------------------------------------------

echo "Generating cosign keypair for keyed signing..."
cd /tmp
export COSIGN_PASSWORD=""
# Remove any key files left by a previous entrypoint invocation: cosign's
# generate-key-pair prompts interactively (and aborts under -euo pipefail) if
# cosign.key already exists.
rm -f /tmp/cosign.key /tmp/cosign.pub
cosign generate-key-pair

echo "Signing policies/verify-keyed with cosign (keyed)..."
keyed_digest="$(manifest_digest policies/verify-keyed)"
retry cosign sign \
	--key /tmp/cosign.key \
	--tlog-upload=false \
	--allow-http-registry \
	--yes \
	"${REGISTRY}/policies/verify-keyed@${keyed_digest}"

verify_signature "policies/verify-keyed"

echo "Copying public key to shared volume..."
cp /tmp/cosign.pub /shared/cosign.pub
# verify-sut runs as an unprivileged user and mounts /shared read-only, so the
# artifacts must be world-readable (this container runs as root).
chmod 0644 /shared/cosign.pub

# --- wrong keypair for negative tests ----------------------------------------

echo "Generating wrong keypair for negative test cases..."
cd /tmp
mkdir -p wrong-key
cd wrong-key
rm -f /tmp/wrong-key/cosign.key /tmp/wrong-key/cosign.pub
cosign generate-key-pair

cp /tmp/wrong-key/cosign.pub /shared/wrong.pub
chmod 0644 /shared/wrong.pub
# Private key stays in /tmp — not written to shared volume.

echo "Wrong public key written to /shared/wrong.pub."

# --- keyless signing against the private Sigstore ----------------------------

DEX_ISSUER="http://dex-idp:5556/dex"
FULCIO_URL="http://fulcio:5555"
REKOR_URL="http://rekor-server:3000"
CTFE_URL="http://ctfe:6962/testlog"

# The Sigstore images (fulcio, rekor-server, ctfe) are distroless and ship no
# curl, so they have no compose healthcheck; readiness is polled here instead.
# Wait for the CT log before signing so Fulcio can embed a Signed Certificate
# Timestamp in the issued cert (sigstore-go verification requires >=1 SCT).
wait_http "${DEX_ISSUER}/healthz" "Dex"
wait_http "${CTFE_URL}/ct/v1/get-sth" "CTFE"
wait_http "${FULCIO_URL}/api/v1/rootCert" "Fulcio"
wait_http "${REKOR_URL}/ping" "Rekor"

echo "Requesting OIDC token from Dex..."
id_token="$(retry curl -fsS \
	-d 'grant_type=password' \
	-d 'username=admin@example.com' \
	-d 'password=password' \
	-d 'scope=openid email' \
	-d 'client_id=complyctl-test' \
	-d 'client_secret=test-secret' \
	"${DEX_ISSUER}/token" | jq -r '.id_token')"

if [[ -z "${id_token}" || "${id_token}" == "null" ]]; then
	echo "ERROR: failed to obtain OIDC id_token from Dex" >&2
	exit 1
fi

echo "Keyless-signing policies/verify-keyless..."
# After Fulcio issues the cert, cosign verifies the embedded SCT against a known
# CT-log public key. Our private CTFE is not in cosign's TUF root, so point
# cosign at the generated CTFE public key to verify the SCT it signed.
export SIGSTORE_CT_LOG_PUBLIC_KEY_FILE="/pki/ctfe.pub"
keyless_digest="$(manifest_digest policies/verify-keyless)"
retry cosign sign \
	--fulcio-url="${FULCIO_URL}" \
	--rekor-url="${REKOR_URL}" \
	--identity-token="${id_token}" \
	--allow-http-registry \
	--yes \
	"${REGISTRY}/policies/verify-keyless@${keyless_digest}"

verify_signature "policies/verify-keyless"

# --- trusted-root generation -------------------------------------------------

echo "Generating trusted_root.json..."
tmp_root="$(mktemp /shared/trusted_root.json.XXXXXX)"

# cosign is pinned to v2.4.3 in Dockerfile.sign-seed, which ships
# `trusted-root create`. In v2.4.3 that subcommand accepts only file inputs
# (--certificate-chain / --ctfe-key / --rekor-key), NOT --fulcio-url/--rekor-url,
# so we assemble the trust root from the generated Fulcio root, the generated
# CTFE public key, and the Rekor public key fetched at runtime. Fail closed if
# a future pin drops the subcommand.
if ! cosign trusted-root create --help >/dev/null 2>&1; then
	echo "ERROR: cosign lacks 'trusted-root create' (pinned v2.4.3 must provide it)" >&2
	rm -f "${tmp_root}"
	exit 1
fi

# Rekor's public key is served over its API; the generated Fulcio root and CTFE
# public key are mounted from the pki-material volume at /pki/.
rekor_key="$(mktemp /tmp/rekor-pub.XXXXXX.pem)"
retry curl -fsS "${REKOR_URL}/api/v1/log/publicKey" -o "${rekor_key}"

cosign trusted-root create \
	--certificate-chain=/pki/root.pem \
	--ctfe-key=/pki/ctfe.pub \
	--rekor-key="${rekor_key}" \
	--out="${tmp_root}"

rm -f "${rekor_key}"

# mktemp creates the file 0600 (root-only); verify-sut runs unprivileged and
# mounts /shared read-only, so make it world-readable before publishing.
chmod 0644 "${tmp_root}"

# Atomic publish so verify-sut never reads a half-written file.
mv -f "${tmp_root}" /shared/trusted_root.json
echo "trusted_root.json written."

# --- done --------------------------------------------------------------------

# Completion sentinel — written last, after every artifact is signed and
# published, so verify-sut-entrypoint.sh can gate on it rather than on the
# individual artifact files (which can persist as stale copies on a reused
# shared volume under podman-compose).  world-readable for the unprivileged
# verify-sut container that mounts /shared read-only.
touch /shared/.sign-seed-complete
chmod 0644 /shared/.sign-seed-complete

echo "Sign-seed complete."
