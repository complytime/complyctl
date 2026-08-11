#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

# generate-pki.sh — generate test PKI material at runtime into PKI_DIR.
# Produces a Fulcio CA keypair (root.key + root.pem) and a CTFE keypair
# (privkey.pem + ctfe.pub). All credentials are test-only values with no
# security value outside this acceptance-test environment.

PKI_DIR="${PKI_DIR:-/pki}"

mkdir -p "${PKI_DIR}"
umask 0077

# --- Fulcio CA key (EC P-256, AES-256-CBC, passphrase: fulcio) ---------------
echo "Generating Fulcio CA private key..."
openssl ecparam -name prime256v1 -genkey -noout |
	openssl ec -aes256 -passout pass:fulcio -out "${PKI_DIR}/root.key"
# 0644: Fulcio and other services run as nonroot in distroless images.
# The key is AES-256-CBC encrypted; world-readable is acceptable for test-only material.
chmod 0644 "${PKI_DIR}/root.key"
echo "  wrote ${PKI_DIR}/root.key"

# --- Fulcio CA certificate (self-signed, 10-year validity) --------------------
echo "Generating Fulcio CA certificate..."
openssl req -new -x509 \
	-key "${PKI_DIR}/root.key" \
	-passin pass:fulcio \
	-out "${PKI_DIR}/root.pem" \
	-days 3650 \
	-subj "/CN=Test Fulcio CA/O=complyctl-test/C=US" \
	-addext "basicConstraints=critical,CA:true" \
	-addext "keyUsage=critical,keyCertSign,cRLSign"
chmod 0644 "${PKI_DIR}/root.pem"
echo "  wrote ${PKI_DIR}/root.pem"

# --- CTFE private key (EC P-256, legacy AES-256-CBC, passphrase: ctfe) -------
echo "Generating CTFE private key..."
openssl ecparam -name prime256v1 -genkey -noout |
	openssl ec -aes256 -passout pass:ctfe -out "${PKI_DIR}/privkey.pem"
chmod 0644 "${PKI_DIR}/privkey.pem"
echo "  wrote ${PKI_DIR}/privkey.pem"

# --- CTFE public key (PEM PKIX/SubjectPublicKeyInfo format) -------------------
echo "Deriving CTFE public key..."
openssl ec -in "${PKI_DIR}/privkey.pem" -passin pass:ctfe \
	-pubout -out "${PKI_DIR}/ctfe.pub"
chmod 0644 "${PKI_DIR}/ctfe.pub"
echo "  wrote ${PKI_DIR}/ctfe.pub"

echo "PKI material generation complete in ${PKI_DIR}"
