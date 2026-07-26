package autofocus

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigDefaultsToFiveSeconds(t *testing.T) {
	config, err := loadConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if config.IdleDuration != 5*time.Second {
		t.Fatalf("IdleDuration = %s, want 5s", config.IdleDuration)
	}
}

func TestLoadConfigReadsIdleSeconds(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "config.json"),
		[]byte(`{"idle_seconds": 12}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	config, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if config.IdleDuration != 12*time.Second {
		t.Fatalf("IdleDuration = %s, want 12s", config.IdleDuration)
	}
}

func TestLoadConfigRejectsInvalidContent(t *testing.T) {
	tests := []string{
		`{"idle_seconds": 0}`,
		`{"idle_seconds": 3601}`,
		`{"idle_seconds": "5"}`,
		`{"unknown": true}`,
		`{} {}`,
	}
	for _, content := range tests {
		t.Run(content, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(
				filepath.Join(dir, "config.json"),
				[]byte(content),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			if _, err := loadConfig(dir); err == nil {
				t.Fatal("loadConfig() error = nil, want error")
			}
		})
	}
}
