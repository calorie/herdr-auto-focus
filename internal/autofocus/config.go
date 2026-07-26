package autofocus

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultIdleSeconds = 5
	maxIdleSeconds     = 3600
)

type Config struct {
	IdleDuration time.Duration
}

type configFile struct {
	IdleSeconds *int `json:"idle_seconds"`
}

func loadConfig(dir string) (Config, error) {
	config := Config{IdleDuration: defaultIdleSeconds * time.Second}
	path := filepath.Join(dir, "config.json")
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var raw configFile
	if err := decoder.Decode(&raw); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if raw.IdleSeconds != nil {
		if *raw.IdleSeconds < 1 || *raw.IdleSeconds > maxIdleSeconds {
			return Config{}, fmt.Errorf(
				"idle_seconds must be between 1 and %d",
				maxIdleSeconds,
			)
		}
		config.IdleDuration = time.Duration(*raw.IdleSeconds) * time.Second
	}
	return config, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON values are not allowed")
}
