#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

REGISTRY="${REGISTRY_URL:-zot:5000}"

echo "Waiting for registry at ${REGISTRY}..."
for i in $(seq 1 30); do
	if oras repo list --plain-http "${REGISTRY}" >/dev/null 2>&1; then
		echo "Registry is ready."
		break
	fi
	if [[ "$i" -eq 30 ]]; then
		echo "ERROR: Registry not ready after 30 attempts."
		exit 1
	fi
	sleep 1
done

echo "Pushing Gemara policy bundle..."
cd /testdata
oras push --plain-http "${REGISTRY}/policies/acceptance-test:v1.0.0" \
	catalog.yaml:application/vnd.gemara.catalog.v1+yaml \
	policy.yaml:application/vnd.gemara.policy.v1+yaml

echo "Tagging latest..."
oras tag --plain-http "${REGISTRY}/policies/acceptance-test:v1.0.0" latest

echo "Verifying push..."
tags=$(oras repo tags --plain-http "${REGISTRY}/policies/acceptance-test")
echo "Tags: ${tags}"

if ! echo "${tags}" | grep -q "v1.0.0"; then
	echo "ERROR: v1.0.0 tag not found after push."
	exit 1
fi
if ! echo "${tags}" | grep -q "latest"; then
	echo "ERROR: latest tag not found after tagging."
	exit 1
fi

echo "Seed complete."
