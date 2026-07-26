package autofocus

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGetAgentReadsFocusedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "herdr")
	script := `#!/bin/sh
printf '%s\n' '{"result":{"agent":{"pane_id":"w1:p2","agent_status":"blocked","focused":true}}}'
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	agent, err := (CLIClient{path: path}).GetAgent(
		context.Background(),
		"w1:p2",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !agent.Focused {
		t.Fatalf("GetAgent() = %#v, want focused agent", agent)
	}
}

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
