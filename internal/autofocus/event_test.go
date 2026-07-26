package autofocus

import "testing"

func TestParseEvent(t *testing.T) {
	event, err := parseEvent(`{
		"event": "pane_agent_status_changed",
		"data": {
			"type": "pane_agent_status_changed",
			"pane_id": "w1:p2",
			"workspace_id": "w1",
			"agent_status": "blocked",
			"agent": "codex",
			"title": "Implement feature",
			"display_agent": "Codex",
			"state_labels": {
				"blocked": "Input required"
			}
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if event.PaneID != "w1:p2" || event.Status != "blocked" {
		t.Fatalf("parseEvent() = %#v", event)
	}
}

func TestParseEventRejectsInvalidEvent(t *testing.T) {
	tests := []string{
		`{}`,
		`{"event":"pane_focused","data":{"type":"pane_focused","pane_id":"w1:p1","agent_status":"done"}}`,
		`{"event":"pane_agent_status_changed","data":{"type":"pane_agent_status_changed","pane_id":"","agent_status":"done"}}`,
		`{"event":"pane_agent_status_changed","data":{"type":"pane_agent_status_changed","pane_id":"w1:p1","agent_status":"paused"}}`,
	}
	for _, value := range tests {
		if _, err := parseEvent(value); err == nil {
			t.Fatalf("parseEvent(%q) error = nil, want error", value)
		}
	}
}
