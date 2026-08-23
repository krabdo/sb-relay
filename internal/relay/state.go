package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const stateVersion = 1

type State struct {
	Version int      `json:"version"`
	Seen    []string `json:"seen"`
}

type StateStore interface {
	Load() (State, bool, error)
	Save(State) error
}

type FileStateStore struct {
	path string
}

func NewFileStateStore(path string) *FileStateStore {
	return &FileStateStore{path: path}
}

func (s *FileStateStore) Load() (State, bool, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return State{Version: stateVersion}, false, nil
	}
	if err != nil {
		return State{}, false, fmt.Errorf("read state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, false, fmt.Errorf("decode state: %w", err)
	}
	if state.Version != stateVersion {
		return State{}, false, fmt.Errorf("unsupported state version %d", state.Version)
	}
	for i, id := range state.Seen {
		if id == "" {
			return State{}, false, fmt.Errorf("state contains an empty notification ID at index %d", i)
		}
	}
	return state, true, nil
}

func (s *FileStateStore) Save(state State) error {
	state.Version = stateVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary state: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("protect temporary state: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temporary state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary state: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace state atomically: %w", err)
	}
	return nil
}

type seenSet struct {
	order []string
	set   map[string]struct{}
	max   int
}

func newSeenSet(ids []string, max int) *seenSet {
	s := &seenSet{set: make(map[string]struct{}, len(ids)), max: max}
	start := 0
	if len(ids) > max {
		start = len(ids) - max
	}
	for _, id := range ids[start:] {
		if _, exists := s.set[id]; exists {
			continue
		}
		s.order = append(s.order, id)
		s.set[id] = struct{}{}
	}
	return s
}

func (s *seenSet) Has(id string) bool {
	_, ok := s.set[id]
	return ok
}

func (s *seenSet) Add(id string) {
	if s.Has(id) {
		return
	}
	s.order = append(s.order, id)
	s.set[id] = struct{}{}
	for len(s.order) > s.max {
		delete(s.set, s.order[0])
		s.order = s.order[1:]
	}
}

func (s *seenSet) State() State {
	return State{Version: stateVersion, Seen: append([]string(nil), s.order...)}
}
