#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# createtree-init.sh provisions the CTFE (Certificate Transparency Front End)
# for the acceptance verification stack. It runs once as a compose init service
# and exits 0 on success so ctfe (and Fulcio) can start.
#
# Steps:
#   1. Wait for the trillian-log-server admin gRPC endpoint.
#   2. Load the CTFE issuance-chain schema into MySQL (idempotent).
#   3. Provision a Trillian tree and capture its numeric tree id.
#   4. Render ct_server.cfg from the template, substituting the tree id, into
#      the shared /ctfe-config volume alongside copies of the committed Fulcio
#      root and CTFE private key that ct_server reads.
#
# Inputs (mounted read-only):
#   /input/ct_server.cfg.template  config template with @TREE_ID@
#   /input/ctfe-schema.sql         CTFE MySQL schema (idempotent)
#   /pki/root.pem                  Fulcio fileca root (shared trust anchor)
#   /pki/privkey.pem               CTFE EC signing key (legacy-PEM-encrypted,
#                                  passphrase "ctfe"; see ct_server.cfg.template)
# Output (shared volume, read by ctfe):
#   /ctfe-config/ct_server.cfg
#   /ctfe-config/root.pem
#   /ctfe-config/privkey.pem
set -euo pipefail

TRILLIAN_ADMIN="${TRILLIAN_ADMIN:-trillian-log-server:8090}"
MYSQL_HOST="${MYSQL_HOST:-mysql}"
MYSQL_USER="${MYSQL_USER:-test}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-zaphod}"
MYSQL_DATABASE="${MYSQL_DATABASE:-test}"
INPUT_DIR="${INPUT_DIR:-/input}"
# PKI_DIR is separate from INPUT_DIR so that ct_server.cfg.template and
# ctfe-schema.sql can be file bind-mounts at /input while the pki-material
# volume is mounted at /pki — Docker does not allow file bind-mounts inside
# a read-only volume mount at the same path.
PKI_DIR="${PKI_DIR:-/pki}"
CONFIG_DIR="${CONFIG_DIR:-/ctfe-config}"

log() { printf '[createtree-init] %s\n' "$*"; }

# 1. Wait for the Trillian admin gRPC port to accept connections.
admin_host="${TRILLIAN_ADMIN%%:*}"
admin_port="${TRILLIAN_ADMIN##*:}"
log "waiting for Trillian admin at ${TRILLIAN_ADMIN}"
for _ in $(seq 1 60); do
	if nc -z "${admin_host}" "${admin_port}" 2>/dev/null; then
		break
	fi
	sleep 2
done
if ! nc -z "${admin_host}" "${admin_port}" 2>/dev/null; then
	log "ERROR: Trillian admin ${TRILLIAN_ADMIN} not reachable"
	exit 1
fi

# 2. Load the CTFE issuance-chain schema (idempotent: CREATE TABLE IF NOT EXISTS).
log "loading CTFE schema into ${MYSQL_HOST}/${MYSQL_DATABASE}"
for _ in $(seq 1 30); do
	if mysql -h "${MYSQL_HOST}" -u "${MYSQL_USER}" "-p${MYSQL_PASSWORD}" \
		"${MYSQL_DATABASE}" <"${INPUT_DIR}/ctfe-schema.sql" 2>/dev/null; then
		break
	fi
	sleep 2
done

# 3. Provision a Trillian tree for the CT log. createtree prints the tree id.
log "provisioning Trillian tree via createtree"
tree_id=""
for _ in $(seq 1 30); do
	if tree_id="$(createtree --admin_server="${TRILLIAN_ADMIN}" 2>/dev/null)"; then
		if [[ -n "${tree_id}" ]]; then
			break
		fi
	fi
	sleep 2
done
if [[ -z "${tree_id}" ]]; then
	log "ERROR: createtree did not return a tree id"
	exit 1
fi
log "provisioned tree id ${tree_id}"

# 4. Render the config. Only @TREE_ID@ (numeric) is substituted; ct_server
#    derives its public key from privkey.pem, so no DER escaping is needed.
#    awk with index()/substr() performs a purely literal replacement.
mkdir -p "${CONFIG_DIR}"
awk -v tree_id="${tree_id}" '
  {
    line = $0
    p = index(line, "@TREE_ID@")
    if (p > 0) {
      line = substr(line, 1, p - 1) tree_id substr(line, p + length("@TREE_ID@"))
    }
    print line
  }
' "${INPUT_DIR}/ct_server.cfg.template" >"${CONFIG_DIR}/ct_server.cfg"

if ! grep -q "log_id: ${tree_id}" "${CONFIG_DIR}/ct_server.cfg"; then
	log "ERROR: rendered ct_server.cfg missing tree id ${tree_id}"
	exit 1
fi

# ct_server reads the root and private key from the shared config volume.
cp "${PKI_DIR}/root.pem" "${CONFIG_DIR}/root.pem"
cp "${PKI_DIR}/privkey.pem" "${CONFIG_DIR}/privkey.pem"

log "wrote ${CONFIG_DIR}/ct_server.cfg (tree id ${tree_id})"
log "done"
