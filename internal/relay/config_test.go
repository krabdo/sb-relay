package relay

import (
	"strings"
	"testing"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("SB_USER_ID", "42")
	t.Setenv("SB_COOKIE", "session=dummy")
	t.Setenv("TELEGRAM_BOT_TOKEN", "123:dummy")
	t.Setenv("TELEGRAM_CHAT_ID", "-100123")
	t.Setenv("POLL_INTERVAL", "30s")
	t.Setenv("STATE_FILE", "test-state.json")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UserID != "42" || cfg.PollInterval.String() != "30s" || cfg.StateFile != "test-state.json" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestConfigRejectsCookieHeaderPrefix(t *testing.T) {
	t.Setenv("SB_USER_ID", "42")
	t.Setenv("SB_COOKIE", "Cookie: session=dummy")
	t.Setenv("TELEGRAM_BOT_TOKEN", "123:dummy")
	t.Setenv("TELEGRAM_CHAT_ID", "123")
	_, err := ConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "without the Cookie: prefix") {
		t.Fatalf("unexpected error: %v", err)
	}
}
