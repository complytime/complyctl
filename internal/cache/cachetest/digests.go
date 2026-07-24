// SPDX-License-Identifier: Apache-2.0

package cachetest

// Canonical test digests for use in unit tests across the codebase.
// These are valid OCI digest strings (sha256 with 64 hex chars) that
// satisfy cache.ValidateDigest. Using shared constants avoids
// repetition and ensures consistency across test files.
const (
	// DigestA is a general-purpose test digest (maps to sha256 of "test").
	DigestA = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	// DigestB is a second distinct test digest.
	DigestB = "sha256:3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d"
	// DigestC is a third distinct test digest.
	DigestC = "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	// DigestD is a fourth distinct test digest.
	DigestD = "sha256:82a920ed89b44f30bd4e09e0c18bc4f2ef3d4274f3e6d5f9c68b14b1e3e5dda6"
	// DigestE is a fifth distinct test digest.
	DigestE = "sha256:a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890a1b2c3d4e5f67890"
	// DigestF is a sixth distinct test digest.
	DigestF = "sha256:b1c2d3e4f5a67890b1c2d3e4f5a67890b1c2d3e4f5a67890b1c2d3e4f5a67890"
)
