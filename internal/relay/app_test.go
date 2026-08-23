package relay

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"testing"
	"time"
)

type fakeForum struct {
	pages map[int]NotificationPage
	err   error
	calls []int
}

func (f *fakeForum) FetchPage(_ context.Context, page int) (NotificationPage, error) {
	f.calls = append(f.calls, page)
	if f.err != nil {
		return NotificationPage{}, f.err
	}
	return f.pages[page], nil
}

type fakeSender struct {
	messages []OutboundMessage
	fail     bool
}

func (s *fakeSender) Send(_ context.Context, message OutboundMessage) error {
	if s.fail {
		return errors.New("send failed")
	}
	s.messages = append(s.messages, message)
	return nil
}

type memoryStore struct {
	state  State
	exists bool
	err    error
}

func (s *memoryStore) Load() (State, bool, error) { return s.state, s.exists, s.err }
func (s *memoryStore) Save(state State) error {
	if s.err != nil {
		return s.err
	}
	s.state, s.exists = state, true
	return nil
}

func note(id, kind string) Notification {
	return Notification{ID: id, Kind: kind, Actor: "user", Content: "body " + id, TargetURL: "https://sb.sb/t/1/?reply_id=" + id, CreatedAt: time.Now()}
}

func testApp(f Forum, sender Sender, store StateStore) *App {
	return NewApp(f, sender, store, AppOptions{PollInterval: time.Minute, MaxPages: 10, MaxSeen: 2048, Logger: log.New(io.Discard, "", 0)})
}

func TestFirstPollSendsOnlyLatestAndRestartDoesNotDuplicate(t *testing.T) {
	forum := &fakeForum{pages: map[int]NotificationPage{1: {Notifications: []Notification{note("new", "回复"), note("old-2", "提及"), note("old-1", "回复")}}}}
	store := &memoryStore{}
	sender := &fakeSender{}
	if err := testApp(forum, sender, store).Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0].PlainText, "body new") {
		t.Fatalf("unexpected first-run messages: %#v", sender.messages)
	}
	if len(store.state.Seen) != 3 {
		t.Fatalf("baseline was not persisted: %#v", store.state.Seen)
	}
	restartedSender := &fakeSender{}
	if err := testApp(forum, restartedSender, store).Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(restartedSender.messages) != 0 {
		t.Fatalf("restart duplicated notifications: %#v", restartedSender.messages)
	}
}

func TestRegularPollCollectsAcrossPagesOldestFirst(t *testing.T) {
	forum := &fakeForum{pages: map[int]NotificationPage{
		1: {Notifications: []Notification{note("a", "回复"), note("b", "回复")}, HasNext: true},
		2: {Notifications: []Notification{note("c", "提及"), note("known", "回复")}},
	}}
	store := &memoryStore{exists: true, state: State{Version: stateVersion, Seen: []string{"known"}}}
	sender := &fakeSender{}
	if err := testApp(forum, sender, store).Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 3 || !strings.Contains(sender.messages[0].PlainText, "body c") || !strings.Contains(sender.messages[2].PlainText, "body a") {
		t.Fatalf("wrong delivery order: %#v", sender.messages)
	}
	if len(forum.calls) != 2 {
		t.Fatalf("expected two pages, got calls %#v", forum.calls)
	}
}

func TestSendFailureDoesNotMarkNotificationSeen(t *testing.T) {
	forum := &fakeForum{pages: map[int]NotificationPage{1: {Notifications: []Notification{note("new", "回复"), note("known", "回复")}}}}
	store := &memoryStore{exists: true, state: State{Version: stateVersion, Seen: []string{"known"}}}
	err := testApp(forum, &fakeSender{fail: true}, store).Poll(context.Background())
	if err == nil {
		t.Fatal("expected send failure")
	}
	if len(store.state.Seen) != 1 || store.state.Seen[0] != "known" {
		t.Fatalf("failed notification was marked seen: %#v", store.state.Seen)
	}
}

func TestPartialShoutrrrSuccessPersistsAndDoesNotResend(t *testing.T) {
	forum := &fakeForum{pages: map[int]NotificationPage{1: {Notifications: []Notification{note("new", "回复"), note("known", "回复")}}}}
	store := &memoryStore{exists: true, state: State{Version: stateVersion, Seen: []string{"known"}}}
	sender := testShoutrrrSender(&bytes.Buffer{},
		shoutrrrDestination{index: 1, scheme: "telegram", router: &fakeDestinationRouter{errs: []error{errors.New("failed")}}},
		shoutrrrDestination{index: 2, scheme: "logger", router: &fakeDestinationRouter{errs: []error{nil}}},
	)
	if err := testApp(forum, sender, store).Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.state.Seen) != 2 || store.state.Seen[0] != "known" || store.state.Seen[1] != "new" {
		t.Fatalf("partial success was not persisted: %#v", store.state.Seen)
	}
	if err := testApp(forum, sender, store).Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.state.Seen) != 2 {
		t.Fatalf("notification was re-added after restart: %#v", store.state.Seen)
	}
}

func TestStateLoadFailureIsFatal(t *testing.T) {
	app := testApp(&fakeForum{}, &fakeSender{}, &memoryStore{err: errors.New("corrupt")})
	err := app.Poll(context.Background())
	var fatal fatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("expected fatal state error, got %v", err)
	}
}
