package relay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStateStoreRoundTripAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	store := NewFileStateStore(path)
	if _, exists, err := store.Load(); err != nil || exists {
		t.Fatalf("missing state: exists=%v err=%v", exists, err)
	}
	if err := store.Save(State{Seen: []string{"one", "two"}}); err != nil {
		t.Fatal(err)
	}
	state, exists, err := store.Load()
	if err != nil || !exists || len(state.Seen) != 2 || state.Seen[1] != "two" {
		t.Fatalf("round trip failed: %+v exists=%v err=%v", state, exists, err)
	}
	if err := store.Save(State{Seen: []string{"three"}}); err != nil {
		t.Fatalf("atomic replacement failed: %v", err)
	}
	state, _, err = store.Load()
	if err != nil || len(state.Seen) != 1 || state.Seen[0] != "three" {
		t.Fatalf("replacement round trip failed: %+v err=%v", state, err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Load(); err == nil {
		t.Fatal("corrupt state must fail")
	}
}

func TestSeenSetRetention(t *testing.T) {
	seen := newSeenSet(nil, 2)
	seen.Add("one")
	seen.Add("two")
	seen.Add("three")
	if seen.Has("one") || !seen.Has("two") || !seen.Has("three") {
		t.Fatalf("unexpected retained IDs: %#v", seen.order)
	}
}
