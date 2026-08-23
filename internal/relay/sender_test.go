package relay

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/url"
	"strings"
	"testing"

	"github.com/containrrr/shoutrrr/pkg/types"
)

type fakeDestinationRouter struct {
	errs    []error
	body    string
	params  types.Params
	started chan struct{}
	block   chan struct{}
}

func (f *fakeDestinationRouter) Send(body string, params *types.Params) []error {
	f.body = body
	f.params = *params
	if f.started != nil {
		close(f.started)
	}
	if f.block != nil {
		<-f.block
	}
	return f.errs
}

func testShoutrrrSender(buffer *bytes.Buffer, destinations ...shoutrrrDestination) *ShoutrrrSender {
	return &ShoutrrrSender{destinations: destinations, logger: log.New(buffer, "", 0)}
}

func TestNormalizeTelegramForcesHTMLAndDisablesPreview(t *testing.T) {
	scheme, normalized, err := normalizeDestinationURL("telegram://123:secret@telegram?chats=42&preview=Yes&parseMode=None")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		t.Fatal(err)
	}
	if scheme != "telegram" || parsed.Query().Get("parsemode") != "HTML" || parsed.Query().Get("preview") != "No" {
		t.Fatalf("unexpected normalized URL: %s", normalized)
	}
	if parsed.Query().Get("parseMode") != "" {
		t.Fatalf("conflicting parse mode survived normalization: %s", normalized)
	}
}

func TestNewShoutrrrSenderRejectsInvalidWithoutLeakingCredentials(t *testing.T) {
	secret := "do-not-log-this"
	_, err := NewShoutrrrSender([]string{"telegram://123:" + secret + "@telegram"}, defaultHTTPTimeout, nil)
	if err == nil {
		t.Fatal("expected invalid token error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("credential leaked: %v", err)
	}
	_, err = NewShoutrrrSender([]string{"unknown://user:" + secret + "@host"}, defaultHTTPTimeout, nil)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unexpected unknown scheme error: %v", err)
	}
}

func TestNewShoutrrrSenderAcceptsTelegram(t *testing.T) {
	sender, err := NewShoutrrrSender(
		[]string{"telegram://12345:mock-token@telegram?chats=channel-1&parsemode=None&preview=Yes"},
		defaultHTTPTimeout,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.destinations) != 1 || sender.destinations[0].scheme != "telegram" {
		t.Fatalf("unexpected destinations: %+v", sender.destinations)
	}
}

func TestNormalizeDestinationRejectsMissingScheme(t *testing.T) {
	if _, _, err := normalizeDestinationURL("not-a-service-url"); err == nil {
		t.Fatal("expected invalid URL error")
	}
}

func TestShoutrrrSenderSingleSuccess(t *testing.T) {
	target := &fakeDestinationRouter{errs: []error{nil}}
	sender := testShoutrrrSender(&bytes.Buffer{},
		shoutrrrDestination{index: 1, scheme: "logger", router: target},
	)
	if err := sender.Send(context.Background(), OutboundMessage{Title: "title", PlainText: "plain"}); err != nil {
		t.Fatal(err)
	}
	if target.body != "plain" {
		t.Fatalf("unexpected body: %q", target.body)
	}
}

func TestShoutrrrSenderAnySuccessAndMessageVariants(t *testing.T) {
	telegram := &fakeDestinationRouter{errs: []error{errors.New("secret-bearing upstream error")}}
	webhook := &fakeDestinationRouter{errs: []error{nil}}
	var output bytes.Buffer
	sender := testShoutrrrSender(&output,
		shoutrrrDestination{index: 1, scheme: "telegram", router: telegram},
		shoutrrrDestination{index: 2, scheme: "webhook", router: webhook},
	)
	message := OutboundMessage{Title: "title", PlainText: "plain", TelegramHTML: "<b>html</b>"}
	if err := sender.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if telegram.body != message.TelegramHTML || webhook.body != message.PlainText {
		t.Fatalf("wrong bodies: telegram=%q webhook=%q", telegram.body, webhook.body)
	}
	if _, exists := telegram.params[types.TitleKey]; exists {
		t.Fatal("Telegram unexpectedly received title param")
	}
	if title, _ := webhook.params.Title(); title != message.Title {
		t.Fatalf("wrong title: %q", title)
	}
	if strings.Contains(output.String(), "secret-bearing") || !strings.Contains(output.String(), "#1 (telegram)") {
		t.Fatalf("error was not safely logged: %q", output.String())
	}
}

func TestShoutrrrSenderAllFail(t *testing.T) {
	var output bytes.Buffer
	sender := testShoutrrrSender(&output,
		shoutrrrDestination{index: 1, scheme: "logger", router: &fakeDestinationRouter{errs: []error{errors.New("failed")}}},
		shoutrrrDestination{index: 2, scheme: "webhook", router: &fakeDestinationRouter{errs: []error{errors.New("failed")}}},
	)
	if err := sender.Send(context.Background(), OutboundMessage{PlainText: "test"}); err == nil || !strings.Contains(err.Error(), "all 2") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShoutrrrSenderContextCancellation(t *testing.T) {
	started := make(chan struct{})
	block := make(chan struct{})
	sender := testShoutrrrSender(&bytes.Buffer{}, shoutrrrDestination{
		index: 1, scheme: "logger", router: &fakeDestinationRouter{started: started, block: block},
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sender.Send(ctx, OutboundMessage{PlainText: "test"}) }()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	close(block)
}
