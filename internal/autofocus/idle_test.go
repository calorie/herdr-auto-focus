package autofocus

import (
	"testing"
	"time"
)

func TestParseHIDIdleTime(t *testing.T) {
	idle, err := parseHIDIdleTime([]byte(`
		{
		  "HIDIdleTime" = 5000000000
		}
	`))
	if err != nil {
		t.Fatal(err)
	}
	if idle != 5*time.Second {
		t.Fatalf("parseHIDIdleTime() = %s, want 5s", idle)
	}
}

func TestParseHIDIdleTimeRejectsMissingOrDuplicateValues(t *testing.T) {
	for _, output := range []string{
		`{}`,
		`"HIDIdleTime" = 1 "HIDIdleTime" = 2`,
		`"HIDIdleTime" = invalid`,
	} {
		if _, err := parseHIDIdleTime([]byte(output)); err == nil {
			t.Fatalf("parseHIDIdleTime(%q) error = nil, want error", output)
		}
	}
}
