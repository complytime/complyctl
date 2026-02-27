package intoto

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Envelope represents a DSSE signed envelope.
type Envelope struct {
	PayloadType string `json:"payloadType"`
	Payload     string `json:"payload"`
}

// UnwrapDSSE checks if raw JSON is a DSSE envelope and, if so, decodes and
// returns the inner payload. If the data is not a DSSE envelope, it is
// returned unchanged.
func UnwrapDSSE(raw []byte) ([]byte, error) {
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.PayloadType != "" && envelope.Payload != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(envelope.Payload)
		if err != nil {
			decoded, err = base64.StdEncoding.DecodeString(envelope.Payload)
			if err != nil {
				return nil, fmt.Errorf("decoding DSSE payload: %w", err)
			}
		}
		return decoded, nil
	}
	return raw, nil
}
