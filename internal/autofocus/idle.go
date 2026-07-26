package autofocus

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

var hidIdleTimePattern = regexp.MustCompile(`"HIDIdleTime"\s*=\s*([0-9]+)`)

type InputSample struct {
	Idle        time.Duration
	LastInputAt time.Time
}

type InputSource interface {
	Sample(context.Context) (InputSample, error)
}

type HIDIdleSource struct {
	now func() time.Time
}

func (source HIDIdleSource) Sample(ctx context.Context) (InputSample, error) {
	output, err := exec.CommandContext(
		ctx,
		"/usr/sbin/ioreg",
		"-r",
		"-c",
		"IOHIDSystem",
		"-d",
		"1",
	).Output()
	if err != nil {
		return InputSample{}, fmt.Errorf("read HIDIdleTime: %w", err)
	}
	idle, err := parseHIDIdleTime(output)
	if err != nil {
		return InputSample{}, err
	}
	now := source.now()
	return InputSample{
		Idle:        idle,
		LastInputAt: now.Add(-idle),
	}, nil
}

func parseHIDIdleTime(output []byte) (time.Duration, error) {
	matches := hidIdleTimePattern.FindAllSubmatch(output, -1)
	if len(matches) != 1 {
		return 0, errors.New("HIDIdleTime was not found exactly once")
	}
	nanoseconds, err := strconv.ParseInt(string(matches[0][1]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse HIDIdleTime: %w", err)
	}
	return time.Duration(nanoseconds), nil
}
