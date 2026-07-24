// SPDX-License-Identifier: Apache-2.0

package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatStepIdentity_WithComplypackRef(t *testing.T) {
	result := formatStepIdentity("registry.example.com/complypacks/opa@sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", "kubernetes.run_as_nonroot")
	assert.Equal(t, "registry.example.com/complypacks/opa@sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2#kubernetes.run_as_nonroot", result)
}

func TestFormatStepIdentity_WithoutComplypackRef(t *testing.T) {
	result := formatStepIdentity("", "my-check")
	assert.Equal(t, "my-check", result)
}

func TestFormatStepIdentity_EmptyStepName(t *testing.T) {
	result := formatStepIdentity("registry.example.com/complypacks/opa@sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", "")
	assert.Equal(t, "", result)
}

func TestFormatStepIdentity_BothEmpty(t *testing.T) {
	result := formatStepIdentity("", "")
	assert.Equal(t, "", result)
}
