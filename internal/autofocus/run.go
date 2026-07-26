package autofocus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	pluginEventName = "pane.agent_status_changed"
	pollInterval    = time.Second
)

func Run(ctx context.Context) error {
	eventName, err := requiredEnv("HERDR_PLUGIN_EVENT")
	if err != nil {
		return err
	}
	if eventName != pluginEventName {
		return fmt.Errorf("unexpected HERDR_PLUGIN_EVENT %q", eventName)
	}
	eventJSON, err := requiredEnv("HERDR_PLUGIN_EVENT_JSON")
	if err != nil {
		return err
	}
	event, err := parseEvent(eventJSON)
	if err != nil {
		return err
	}
	configDir, err := requiredDirectoryEnv("HERDR_PLUGIN_CONFIG_DIR")
	if err != nil {
		return err
	}
	stateDir, err := requiredDirectoryEnv("HERDR_PLUGIN_STATE_DIR")
	if err != nil {
		return err
	}
	herdrPath, err := requiredEnv("HERDR_BIN_PATH")
	if err != nil {
		return err
	}
	config, err := loadConfig(configDir)
	if err != nil {
		return err
	}
	store, err := newStore(stateDir)
	if err != nil {
		return err
	}
	attention, err := store.applyEvent(event)
	if err != nil {
		return err
	}
	if !attention {
		return nil
	}

	workerLock, acquired, err := acquireFileLock(
		filepath.Join(stateDir, "worker.lock"),
		true,
	)
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}

	worker := Worker{
		store:         store,
		herdr:         CLIClient{path: herdrPath},
		input:         HIDIdleSource{now: time.Now},
		clock:         realClock{},
		idleThreshold: config.IdleDuration,
		pollInterval:  pollInterval,
	}
	return worker.run(ctx, workerLock)
}

func requiredEnv(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func requiredDirectoryEnv(name string) (string, error) {
	value, err := requiredEnv(name)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(value)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", name, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s must point to a directory", name)
	}
	return value, nil
}

func waitForPoll(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil
		}
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
