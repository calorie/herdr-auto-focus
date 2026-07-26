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
		outcome, wait, released, err := worker.processNext(ctx, workerLock)
		if err != nil {
			return err
		}
		if released {
			return nil
		}
		if outcome != processWaiting {
			continue
		}
		if err := waitForPoll(ctx, wait); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

func (worker *Worker) processNext(
	ctx context.Context,
	workerLock *fileLock,
) (outcome processOutcome, wait time.Duration, released bool, err error) {
	outcome = processChanged
	err = worker.store.withLocked(func(state *State) (bool, error) {
		pending, ok := selectPending(state.Pending)
		if !ok {
			if err := workerLock.Unlock(); err != nil {
				return false, err
			}
			released = true
			return false, nil
		}

		sample, err := worker.input.Sample(ctx)
		if err != nil {
			return false, err
		}
		wait = focusDelay(
			sample,
			state.LastAutoFocusAtUnixNano,
			worker.idleThreshold,
		)
		if wait > 0 {
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
	return outcome, wait, released, err
}

func focusDelay(
	sample InputSample,
	lastAutoFocusAtUnixNano int64,
	idleThreshold time.Duration,
) time.Duration {
	if sample.Idle < idleThreshold {
		return idleThreshold - sample.Idle
	}
	if lastAutoFocusAtUnixNano != 0 &&
		sample.LastInputAt.UnixNano() <= lastAutoFocusAtUnixNano {
		return idleThreshold
	}
	return 0
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
