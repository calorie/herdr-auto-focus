package autofocus

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeHerdrClient struct {
	agent    AgentInfo
	agentErr error
	focusErr error
	focused  []string
	getCalls int
}

func (client *fakeHerdrClient) GetAgent(
	context.Context,
	string,
) (AgentInfo, error) {
	client.getCalls++
	return client.agent, client.agentErr
}

func (client *fakeHerdrClient) FocusAgent(
	_ context.Context,
	target string,
) error {
	if client.focusErr != nil {
		return client.focusErr
	}
	client.focused = append(client.focused, target)
	return nil
}

type fakeInputSource struct {
	sample InputSample
	err    error
}

func (source fakeInputSource) Sample(context.Context) (InputSample, error) {
	return source.sample, source.err
}

type fakeClock struct {
	now time.Time
}

func (clock fakeClock) Now() time.Time {
	return clock.now
}

func TestSelectPendingPrioritizesBlockedThenSequence(t *testing.T) {
	pending, ok := selectPending(map[string]Pending{
		"w1:p1": {PaneID: "w1:p1", Status: "done", Sequence: 1},
		"w1:p2": {PaneID: "w1:p2", Status: "blocked", Sequence: 3},
		"w1:p3": {PaneID: "w1:p3", Status: "blocked", Sequence: 2},
	})
	if !ok {
		t.Fatal("selectPending() ok = false")
	}
	if pending.PaneID != "w1:p3" {
		t.Fatalf("selectPending() = %#v, want w1:p3", pending)
	}
}

func TestProcessWaitsUntilFiveSecondsIdle(t *testing.T) {
	now := time.Unix(100, 0)
	worker, pending, client := testWorker(t, now, InputSample{
		Idle:        4*time.Second + 999*time.Millisecond,
		LastInputAt: now.Add(-4*time.Second - 999*time.Millisecond),
	})

	outcome, err := worker.process(context.Background(), pending)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != processWaiting {
		t.Fatalf("process() outcome = %v, want waiting", outcome)
	}
	if len(client.focused) != 0 {
		t.Fatalf("focused = %#v, want empty", client.focused)
	}
	if client.getCalls != 0 {
		t.Fatalf(
			"GetAgent calls = %d, want no calls while input is active",
			client.getCalls,
		)
	}
}

func TestProcessFocusesAttentionPaneAfterFiveSecondsIdle(t *testing.T) {
	now := time.Unix(100, 0)
	worker, pending, client := testWorker(t, now, InputSample{
		Idle:        5 * time.Second,
		LastInputAt: now.Add(-5 * time.Second),
	})

	outcome, err := worker.process(context.Background(), pending)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != processFocused {
		t.Fatalf("process() outcome = %v, want focused", outcome)
	}
	if len(client.focused) != 1 || client.focused[0] != pending.PaneID {
		t.Fatalf("focused = %#v", client.focused)
	}
	state := readState(t, worker.store)
	if len(state.Pending) != 0 {
		t.Fatalf("pending = %#v, want empty", state.Pending)
	}
	if state.LastAutoFocusAtUnixNano != now.UnixNano() {
		t.Fatalf(
			"LastAutoFocusAtUnixNano = %d, want %d",
			state.LastAutoFocusAtUnixNano,
			now.UnixNano(),
		)
	}
}

func TestProcessRequiresNewInputAfterAutomaticFocus(t *testing.T) {
	now := time.Unix(100, 0)
	worker, pending, client := testWorker(t, now, InputSample{
		Idle:        10 * time.Second,
		LastInputAt: now.Add(-10 * time.Second),
	})
	if err := worker.store.withLocked(func(state *State) (bool, error) {
		state.LastAutoFocusAtUnixNano = now.Add(-5 * time.Second).UnixNano()
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}

	outcome, err := worker.process(context.Background(), pending)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != processWaiting {
		t.Fatalf("process() outcome = %v, want waiting", outcome)
	}
	if len(client.focused) != 0 {
		t.Fatalf("focused = %#v, want empty", client.focused)
	}

	worker.input = fakeInputSource{sample: InputSample{
		Idle:        5 * time.Second,
		LastInputAt: now.Add(-4 * time.Second),
	}}
	outcome, err = worker.process(context.Background(), pending)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != processFocused {
		t.Fatalf("process() outcome = %v, want focused", outcome)
	}
}

func TestProcessRemovesSettledOrFocusedPane(t *testing.T) {
	tests := []struct {
		name  string
		agent AgentInfo
	}{
		{
			name:  "settled",
			agent: AgentInfo{PaneID: "w1:p2", Status: "idle"},
		},
		{
			name: "current",
			agent: AgentInfo{
				PaneID:  "w1:p2",
				Status:  "blocked",
				Focused: true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := time.Unix(100, 0)
			worker, pending, client := testWorker(t, now, InputSample{
				Idle:        5 * time.Second,
				LastInputAt: now.Add(-5 * time.Second),
			})
			client.agent = test.agent

			outcome, err := worker.process(context.Background(), pending)
			if err != nil {
				t.Fatal(err)
			}
			if outcome != processRemoved {
				t.Fatalf("process() outcome = %v, want removed", outcome)
			}
			if len(readState(t, worker.store).Pending) != 0 {
				t.Fatal("pending notification was not removed")
			}
		})
	}
}

func TestProcessKeepsPendingNotificationOnFailure(t *testing.T) {
	now := time.Unix(100, 0)
	worker, pending, client := testWorker(t, now, InputSample{
		Idle:        5 * time.Second,
		LastInputAt: now.Add(-5 * time.Second),
	})
	client.focusErr = errors.New("focus failed")

	if _, err := worker.process(context.Background(), pending); err == nil {
		t.Fatal("process() error = nil, want error")
	}
	if len(readState(t, worker.store).Pending) != 1 {
		t.Fatal("pending notification was removed after failure")
	}
}

func testWorker(
	t *testing.T,
	now time.Time,
	sample InputSample,
) (*Worker, Pending, *fakeHerdrClient) {
	t.Helper()
	store := testStore(t)
	if _, err := store.applyEvent(Event{PaneID: "w1:p2", Status: "blocked"}); err != nil {
		t.Fatal(err)
	}
	pending := readState(t, store).Pending["w1:p2"]
	client := &fakeHerdrClient{
		agent: AgentInfo{PaneID: "w1:p2", Status: "blocked"},
	}
	return &Worker{
		store:         store,
		herdr:         client,
		input:         fakeInputSource{sample: sample},
		clock:         fakeClock{now: now},
		idleThreshold: 5 * time.Second,
		pollInterval:  time.Millisecond,
	}, pending, client
}

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func readState(t *testing.T, store *Store) State {
	t.Helper()
	var snapshot State
	if err := store.withLocked(func(state *State) (bool, error) {
		snapshot = *state
		snapshot.Pending = make(map[string]Pending, len(state.Pending))
		for key, pending := range state.Pending {
			snapshot.Pending[key] = pending
		}
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}
	return snapshot
}
