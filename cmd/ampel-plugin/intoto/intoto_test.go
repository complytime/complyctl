package intoto

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnwrapDSSE_Envelope(t *testing.T) {
	inner := []byte(`{"_type":"https://in-toto.io/Statement/v1"}`)
	envelope := Envelope{
		PayloadType: "application/vnd.in-toto+json",
		Payload:     base64.RawURLEncoding.EncodeToString(inner),
	}
	raw, _ := json.Marshal(envelope)

	result, err := UnwrapDSSE(raw)
	require.NoError(t, err)
	require.Equal(t, inner, result)
}

func TestUnwrapDSSE_StdEncoding(t *testing.T) {
	inner := []byte(`{"_type":"https://in-toto.io/Statement/v1"}`)
	envelope := Envelope{
		PayloadType: "application/vnd.in-toto+json",
		Payload:     base64.StdEncoding.EncodeToString(inner),
	}
	raw, _ := json.Marshal(envelope)

	result, err := UnwrapDSSE(raw)
	require.NoError(t, err)
	require.Equal(t, inner, result)
}

func TestUnwrapDSSE_NotEnvelope(t *testing.T) {
	raw := []byte(`{"_type":"https://in-toto.io/Statement/v1"}`)

	result, err := UnwrapDSSE(raw)
	require.NoError(t, err)
	require.Equal(t, raw, result)
}

func TestUnwrapDSSE_InvalidPayload(t *testing.T) {
	envelope := Envelope{
		PayloadType: "application/vnd.in-toto+json",
		Payload:     "!!!invalid-base64!!!",
	}
	raw, _ := json.Marshal(envelope)

	_, err := UnwrapDSSE(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "decoding DSSE payload")
}
