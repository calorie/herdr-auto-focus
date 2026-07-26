package autofocus

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const agentStatusChangedEvent = "pane_agent_status_changed"

type Event struct {
	PaneID string
	Status string
}

type eventEnvelope struct {
	Event string    `json:"event"`
	Data  eventData `json:"data"`
}

type eventData struct {
	Type        string `json:"type"`
	PaneID      string `json:"pane_id"`
	AgentStatus string `json:"agent_status"`
}

func parseEvent(value string) (Event, error) {
	decoder := json.NewDecoder(strings.NewReader(value))
	var envelope eventEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return Event{}, fmt.Errorf("decode HERDR_PLUGIN_EVENT_JSON: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Event{}, fmt.Errorf("decode HERDR_PLUGIN_EVENT_JSON: %w", err)
	}
	if envelope.Event != agentStatusChangedEvent ||
		envelope.Data.Type != agentStatusChangedEvent {
		return Event{}, errors.New("unexpected Herdr plugin event")
	}
	if strings.TrimSpace(envelope.Data.PaneID) == "" {
		return Event{}, errors.New("event pane_id is required")
	}
	switch envelope.Data.AgentStatus {
	case "idle", "working", "blocked", "done", "unknown":
	default:
		return Event{}, fmt.Errorf(
			"unsupported agent_status %q",
			envelope.Data.AgentStatus,
		)
	}
	return Event{
		PaneID: envelope.Data.PaneID,
		Status: envelope.Data.AgentStatus,
	}, nil
}

func isAttentionStatus(status string) bool {
	return status == "blocked" || status == "done"
}
