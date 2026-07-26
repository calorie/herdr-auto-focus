package autofocus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type AgentInfo struct {
	PaneID string `json:"pane_id"`
	Status string `json:"agent_status"`
}

type HerdrClient interface {
	GetAgent(context.Context, string) (AgentInfo, error)
	CurrentPane(context.Context) (string, error)
	FocusAgent(context.Context, string) error
}

type CLIClient struct {
	path string
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (err *APIError) Error() string {
	return fmt.Sprintf("Herdr API error %s: %s", err.Code, err.Message)
}

func isAgentNotFound(err error) bool {
	var apiError *APIError
	return errors.As(err, &apiError) && apiError.Code == "agent_not_found"
}

func (client CLIClient) GetAgent(ctx context.Context, target string) (AgentInfo, error) {
	var result struct {
		Agent AgentInfo `json:"agent"`
	}
	if err := client.call(ctx, &result, "agent", "get", target); err != nil {
		return AgentInfo{}, err
	}
	if result.Agent.PaneID == "" {
		return AgentInfo{}, errors.New("agent get response did not include pane_id")
	}
	return result.Agent, nil
}

func (client CLIClient) CurrentPane(ctx context.Context) (string, error) {
	var result struct {
		Pane struct {
			PaneID string `json:"pane_id"`
		} `json:"pane"`
	}
	if err := client.call(ctx, &result, "pane", "current"); err != nil {
		return "", err
	}
	if result.Pane.PaneID == "" {
		return "", errors.New("pane current response did not include pane_id")
	}
	return result.Pane.PaneID, nil
}

func (client CLIClient) FocusAgent(ctx context.Context, target string) error {
	var result struct {
		Agent AgentInfo `json:"agent"`
	}
	return client.call(ctx, &result, "agent", "focus", target)
}

func (client CLIClient) call(ctx context.Context, result any, args ...string) error {
	command := exec.CommandContext(ctx, client.path, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if apiError := decodeAPIError(stdout.Bytes(), stderr.Bytes()); apiError != nil {
			return apiError
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			return fmt.Errorf("run Herdr command: %w", err)
		}
		return fmt.Errorf("run Herdr command: %w: %s", err, message)
	}

	var response struct {
		Result json.RawMessage `json:"result"`
		Error  *APIError       `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return fmt.Errorf("decode Herdr response: %w", err)
	}
	if response.Error != nil {
		return response.Error
	}
	if len(response.Result) == 0 {
		return errors.New("Herdr response did not include result")
	}
	if err := json.Unmarshal(response.Result, result); err != nil {
		return fmt.Errorf("decode Herdr result: %w", err)
	}
	return nil
}

func decodeAPIError(values ...[]byte) error {
	for _, value := range values {
		var response struct {
			Error *APIError `json:"error"`
		}
		if json.Unmarshal(value, &response) == nil && response.Error != nil {
			return response.Error
		}
	}
	return nil
}
