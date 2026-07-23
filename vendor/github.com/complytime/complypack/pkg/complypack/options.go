// SPDX-License-Identifier: Apache-2.0

package complypack

// PackOption configures optional Pack behavior.
type PackOption func(*packOptions)

// packOptions holds internal state for Pack options.
type packOptions struct {
	// Annotations for OCI manifest
	annotations map[string]string
}

// WithAnnotations adds OCI manifest annotations.
// Common annotations: org.opencontainers.image.created, org.opencontainers.image.authors
func WithAnnotations(annotations map[string]string) PackOption {
	return func(o *packOptions) {
		if o.annotations == nil {
			o.annotations = make(map[string]string)
		}
		for k, v := range annotations {
			o.annotations[k] = v
		}
	}
}
