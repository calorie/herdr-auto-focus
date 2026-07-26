package autofocus

import (
	"errors"
	"testing"
)

func TestDecodeAPIError(t *testing.T) {
	err := decodeAPIError(
		nil,
		[]byte(`{"error":{"code":"agent_not_found","message":"missing"}}`),
	)
	if err == nil {
		t.Fatal("decodeAPIError() = nil, want error")
	}
	if !isAgentNotFound(err) {
		t.Fatalf("isAgentNotFound(%v) = false", err)
	}

	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.Message != "missing" {
		t.Fatalf("decodeAPIError() = %#v", err)
	}
}
