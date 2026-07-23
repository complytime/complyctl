// SPDX-License-Identifier: Apache-2.0

package complypack

import "errors"

var (
	// ErrInvalidConfig is returned when the Config has missing or invalid required fields.
	ErrInvalidConfig = errors.New("complypack: invalid config")

	// ErrEmptyContent is returned when the content reader provides zero bytes.
	ErrEmptyContent = errors.New("complypack: content must not be empty")

	// ErrContentTooLarge is returned when content exceeds maximum size.
	ErrContentTooLarge = errors.New("complypack: content exceeds maximum size")

	// ErrInvalidMediaType is returned when an OCI layer has an unexpected media type.
	ErrInvalidMediaType = errors.New("complypack: invalid media type")

	// ErrNoContentLayer is returned when unpacking finds no content layer.
	ErrNoContentLayer = errors.New("complypack: no content layer found")
)
