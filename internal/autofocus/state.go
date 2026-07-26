package autofocus

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Pending struct {
	PaneID   string `json:"pane_id"`
	Status   string `json:"status"`
	Sequence uint64 `json:"sequence"`
}

type State struct {
	NextSequence            uint64             `json:"next_sequence"`
	Pending                 map[string]Pending `json:"pending"`
	LastAutoFocusAtUnixNano int64              `json:"last_auto_focus_at_unix_nano,omitempty"`
}

type Store struct {
	dir       string
	statePath string
	lockPath  string
}

func newStore(dir string) (*Store, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("inspect state directory: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("state path must point to a directory")
	}
	return &Store{
		dir:       dir,
		statePath: filepath.Join(dir, "state.json"),
		lockPath:  filepath.Join(dir, "state.lock"),
	}, nil
}

func (store *Store) applyEvent(event Event) (bool, error) {
	attention := isAttentionStatus(event.Status)
	err := store.withLocked(func(state *State) (bool, error) {
		if attention {
			state.NextSequence++
			state.Pending[event.PaneID] = Pending{
				PaneID:   event.PaneID,
				Status:   event.Status,
				Sequence: state.NextSequence,
			}
			return true, nil
		}
		if _, ok := state.Pending[event.PaneID]; ok {
			delete(state.Pending, event.PaneID)
			return true, nil
		}
		return false, nil
	})
	return attention, err
}

func (store *Store) withLocked(
	callback func(*State) (bool, error),
) (err error) {
	lock, acquired, err := acquireFileLock(store.lockPath, false)
	if err != nil {
		return err
	}
	if !acquired {
		return errors.New("state lock was not acquired")
	}
	defer func() {
		err = errors.Join(err, lock.Unlock())
	}()

	state, err := store.loadUnlocked()
	if err != nil {
		return err
	}
	changed, err := callback(&state)
	if err != nil {
		return err
	}
	if changed {
		return store.saveUnlocked(state)
	}
	return nil
}

func (store *Store) loadUnlocked() (State, error) {
	file, err := os.Open(store.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return newState(), nil
	}
	if err != nil {
		return State{}, fmt.Errorf("open state: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var state State
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("decode state: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return State{}, fmt.Errorf("decode state: %w", err)
	}
	if state.Pending == nil {
		state.Pending = make(map[string]Pending)
	}
	if err := validateState(state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (store *Store) saveUnlocked(state State) (err error) {
	temp, err := os.CreateTemp(store.dir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("create state file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		if tempPath == "" {
			return
		}
		removeErr := os.Remove(tempPath)
		if !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()

	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		temp.Close()
		return fmt.Errorf("encode state: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close state: %w", err)
	}
	if err := os.Rename(tempPath, store.statePath); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	tempPath = ""
	return nil
}

func newState() State {
	return State{Pending: make(map[string]Pending)}
}

func validateState(state State) error {
	for key, pending := range state.Pending {
		if key == "" || pending.PaneID != key {
			return errors.New("state contains an invalid pending pane")
		}
		if !isAttentionStatus(pending.Status) {
			return errors.New("state contains an invalid pending status")
		}
		if pending.Sequence == 0 || pending.Sequence > state.NextSequence {
			return errors.New("state contains an invalid pending sequence")
		}
	}
	return nil
}
