// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/goccy/go-yaml"
)

// DecodeYAML decodes YAML from a reader into the target.
func DecodeYAML(reader io.Reader, target interface{}) error {
	if err := yaml.NewDecoder(reader).Decode(target); err != nil {
		return fmt.Errorf("error decoding YAML: %w", err)
	}
	return nil
}

// DecodeJSON decodes JSON from a reader into the target.
// Unknown fields in the input are rejected.
func DecodeJSON(reader io.Reader, target interface{}) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("error decoding JSON: %w", err)
	}
	return nil
}

// MarshalYAML marshals an object to YAML bytes.
func MarshalYAML(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

// UnmarshalYAML unmarshals YAML bytes into the provided target.
func UnmarshalYAML(data []byte, target interface{}) error {
	return yaml.Unmarshal(data, target)
}
