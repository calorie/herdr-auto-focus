package autofocus

import (
	"context"
	"errors"
	"time"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

type Worker struct {
	store         *Store
	herdr         HerdrClient
	input         InputSource
	clock         Clock
	idleThreshold time.Duration
	pollInterval  time.Duration
}

type processOutcome int

const (
	processChanged processOutcome = iota
	processWaiting
	processRemoved
	processFocused
)

func (worker *Worker) run(ctx context.Context, workerLock *fileLock) (err error) {
	defer func() {
		err = errors.Join(err, workerLock.Unlock())
	}()

	for {
		pending, released, err := worker.nextOrRelease(workerLock)
		if err != nil {
			return err
		}
		if released {
			return nil
		}
		outcome, err := worker.process(ctx, pending)
		if err != nil {
			return err
		}
		if outcome != processWaiting {
			continue
		}
		if err := waitForPoll(ctx, worker.pollInterval); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

func (worker *Worker) nextOrRelease(
	workerLock *fileLock,
) (pending Pending, released bool, err error) {
	err = worker.store.withLocked(func(state *State) (bool, error) {
		selected, ok := selectPending(state.Pending)
		if ok {
			pending = selected
			return false, nil
		}
		if err := workerLock.Unlock(); err != nil {
			return false, err
		}
		released = true
		return false, nil
	})
	return pending, released, err
}

func (worker *Worker) process(
	ctx context.Context,
	expected Pending,
) (outcome processOutcome, err error) {
	outcome = processChanged
	err = worker.store.withLocked(func(state *State) (bool, error) {
		pending, ok := state.Pending[expected.PaneID]
		if !ok || pending.Sequence != expected.Sequence {
			return false, nil
		}

		sample, err := worker.input.Sample(ctx)
		if err != nil {
			return false, err
		}
		if !canFocus(sample, state.LastAutoFocusAtUnixNano, worker.idleThreshold) {
			outcome = processWaiting
			return false, nil
		}

		agent, err := worker.herdr.GetAgent(ctx, pending.PaneID)
		if isAgentNotFound(err) {
			delete(state.Pending, pending.PaneID)
			outcome = processRemoved
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if !isAttentionStatus(agent.Status) {
			delete(state.Pending, pending.PaneID)
			outcome = processRemoved
			return true, nil
		}
		if agent.Focused {
			delete(state.Pending, pending.PaneID)
			outcome = processRemoved
			return true, nil
		}

		if err := worker.herdr.FocusAgent(ctx, pending.PaneID); err != nil {
			return false, err
		}

		delete(state.Pending, pending.PaneID)
		state.LastAutoFocusAtUnixNano = worker.clock.Now().UnixNano()
		outcome = processFocused
		return true, nil
	})
	return outcome, err
}

func canFocus(
	sample InputSample,
	lastAutoFocusAtUnixNano int64,
	idleThreshold time.Duration,
) bool {
	if sample.Idle < idleThreshold {
		return false
	}
	if lastAutoFocusAtUnixNano == 0 {
		return true
	}
	return sample.LastInputAt.UnixNano() > lastAutoFocusAtUnixNano
}

func selectPending(pending map[string]Pending) (Pending, bool) {
	var selected Pending
	found := false
	for _, candidate := range pending {
		if !found || pendingBefore(candidate, selected) {
			selected = candidate
			found = true
		}
	}
	return selected, found
}

func pendingBefore(left, right Pending) bool {
	leftPriority := 1
	if left.Status == "blocked" {
		leftPriority = 0
	}
	rightPriority := 1
	if right.Status == "blocked" {
		rightPriority = 0
	}
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	return left.Sequence < right.Sequence
}
