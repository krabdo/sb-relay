package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTelegramClientRetries429AndDisablesPreview(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var payload struct {
			ChatID             string `json:"chat_id"`
			ParseMode          string `json:"parse_mode"`
			LinkPreviewOptions struct {
				IsDisabled bool `json:"is_disabled"`
			} `json:"link_preview_options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.ChatID != "123" || payload.ParseMode != "HTML" || !payload.LinkPreviewOptions.IsDisabled {
			t.Fatalf("unexpected payload: %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"description":"retry","parameters":{"retry_after":0}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := &TelegramClient{
		endpoint: server.URL,
		chatID:   "123",
		client:   server.Client(),
		sleep:    func(context.Context, time.Duration) error { return nil },
	}
	if err := client.Send(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("expected retry, got %d attempts", attempts)
	}
}
