#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

# verify-sut-entrypoint.sh — wait for sign-seed artifacts before
# running verification tests.  docker compose enforces the
# depends_on service_completed_successfully condition, but
# podman-compose ignores it.  This wrapper provides defence-in-depth
# so local runs succeed under either compose implementation.
#
# The gate is the completion sentinel that sign-seed writes as its
# very last action, not the individual artifact files.  Under
# podman-compose the shared volume can carry stale cosign.pub /
# wrong.pub / trusted_root.json from a prior run, which would satisfy
# a file-only gate "after 0s" and let the tests race a half-seeded
# registry.  The Makefile removes the volume (down -v) before every
# run, so the sentinel is guaranteed absent until this run's sign-seed
# finishes.  It is listed last so its presence proves every artifact
# written before it is also present.

SHARED_DIR="/shared"
MAX_WAIT=120
INTERVAL=2

required_files=(
	"${SHARED_DIR}/cosign.pub"
	"${SHARED_DIR}/wrong.pub"
	"${SHARED_DIR}/trusted_root.json"
	"${SHARED_DIR}/.sign-seed-complete"
)

echo "Waiting for sign-seed artifacts in ${SHARED_DIR}..."
elapsed=0
while true; do
	all_present=true
	for f in "${required_files[@]}"; do
		if [[ ! -f "$f" ]]; then
			all_present=false
			break
		fi
	done

	if $all_present; then
		echo "All sign-seed artifacts present after ${elapsed}s."
		break
	fi

	if [[ $elapsed -ge $MAX_WAIT ]]; then
		echo "ERROR: sign-seed artifacts not found after ${MAX_WAIT}s." >&2
		for f in "${required_files[@]}"; do
			[[ -f "$f" ]] || echo "  missing: $f" >&2
		done
		exit 1
	fi

	sleep "$INTERVAL"
	elapsed=$((elapsed + INTERVAL))
done

exec /usr/local/bin/acceptance.test "$@"
