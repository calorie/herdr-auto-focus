package autofocus

import "testing"

func TestApplyEventQueuesAndRemovesPane(t *testing.T) {
	store := testStore(t)

	attention, err := store.applyEvent(Event{PaneID: "w1:p1", Status: "done"})
	if err != nil {
		t.Fatal(err)
	}
	if !attention {
		t.Fatal("applyEvent() attention = false, want true")
	}
	state := readState(t, store)
	if state.Pending["w1:p1"].Status != "done" {
		t.Fatalf("pending = %#v", state.Pending)
	}

	attention, err = store.applyEvent(Event{PaneID: "w1:p1", Status: "idle"})
	if err != nil {
		t.Fatal(err)
	}
	if attention {
		t.Fatal("applyEvent() attention = true, want false")
	}
	if state := readState(t, store); len(state.Pending) != 0 {
		t.Fatalf("pending = %#v, want empty", state.Pending)
	}
}
