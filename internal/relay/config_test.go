package relay

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("SB_USER_ID", "42")
	t.Setenv("SB_COOKIE", "session=dummy")
	t.Setenv("SHOUTRRR_URLS", "logger:// generic+https://example.invalid/hook")
	t.Setenv("SHOUTRRR_URLS_FILE", "")
	t.Setenv("POLL_INTERVAL", "30s")
	t.Setenv("STATE_FILE", "test-state.json")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UserID != "42" || cfg.PollInterval.String() != "30s" || cfg.StateFile != "test-state.json" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if !reflect.DeepEqual(cfg.ShoutrrrURLs, []string{"logger://", "generic+https://example.invalid/hook"}) {
		t.Fatalf("unexpected URLs: %#v", cfg.ShoutrrrURLs)
	}
}

func TestConfigRejectsCookieHeaderPrefix(t *testing.T) {
	t.Setenv("SB_USER_ID", "42")
	t.Setenv("SB_COOKIE", "Cookie: session=dummy")
	t.Setenv("SHOUTRRR_URLS", "logger://")
	t.Setenv("SHOUTRRR_URLS_FILE", "")
	_, err := ConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "without the Cookie: prefix") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigReadsURLFileAndIgnoresBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "destinations")
	if err := os.WriteFile(path, []byte("\nlogger://\r\n  generic+https://example.invalid/hook  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SB_USER_ID", "42")
	t.Setenv("SB_COOKIE", "session=dummy")
	t.Setenv("SHOUTRRR_URLS", "")
	t.Setenv("SHOUTRRR_URLS_FILE", path)
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.ShoutrrrURLs, []string{"logger://", "generic+https://example.invalid/hook"}) {
		t.Fatalf("unexpected URLs: %#v", cfg.ShoutrrrURLs)
	}
}

func TestConfigRejectsMutualAndDuplicateURLs(t *testing.T) {
	if _, err := loadShoutrrrURLs("logger://", "destinations"); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("unexpected mutual exclusion error: %v", err)
	}
	if _, err := loadShoutrrrURLs("logger:// logger://", ""); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("unexpected duplicate error: %v", err)
	}
}

func TestLegacyTelegramVariablesDoNotSatisfyConfig(t *testing.T) {
	t.Setenv("SB_USER_ID", "42")
	t.Setenv("SB_COOKIE", "session=dummy")
	t.Setenv("SHOUTRRR_URLS", "")
	t.Setenv("SHOUTRRR_URLS_FILE", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "123:legacy")
	t.Setenv("TELEGRAM_CHAT_ID", "42")
	_, err := ConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "SHOUTRRR_URLS") {
		t.Fatalf("unexpected error: %v", err)
	}
}
