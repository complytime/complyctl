// SPDX-License-Identifier: Apache-2.0

package codec

// BaseConverter maps between a wrapper type and its base generated type.
type BaseConverter[Base any] interface {
	ToBase() Base
	FromBase(*Base)
}

// MarshalBaseYAML marshals a BaseConverter by serializing its base type
// to YAML bytes.
func MarshalBaseYAML[B any](c BaseConverter[B]) ([]byte, error) {
	return MarshalYAML(c.ToBase())
}

// UnmarshalBaseYAML unmarshals YAML into a BaseConverter through its base type.
func UnmarshalBaseYAML[B any](data []byte, c BaseConverter[B]) error {
	var base B
	if err := UnmarshalYAML(data, &base); err != nil {
		return err
	}
	c.FromBase(&base)
	return nil
}
