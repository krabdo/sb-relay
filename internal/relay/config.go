package relay

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	defaultPollInterval = time.Minute
	defaultHTTPTimeout  = 20 * time.Second
	defaultStateFile    = "/data/state.json"
)

var numericUserID = regexp.MustCompile(`^[0-9]+$`)

type Config struct {
	UserID           string
	Cookie           string
	TelegramBotToken string
	TelegramChatID   string
	PollInterval     time.Duration
	HTTPTimeout      time.Duration
	StateFile        string
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		UserID:           strings.TrimSpace(os.Getenv("SB_USER_ID")),
		Cookie:           strings.TrimSpace(os.Getenv("SB_COOKIE")),
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatID:   strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
		PollInterval:     defaultPollInterval,
		HTTPTimeout:      defaultHTTPTimeout,
		StateFile:        strings.TrimSpace(os.Getenv("STATE_FILE")),
	}
	if cfg.StateFile == "" {
		cfg.StateFile = defaultStateFile
	}
	if value := strings.TrimSpace(os.Getenv("POLL_INTERVAL")); value != "" {
		d, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("POLL_INTERVAL must be a Go duration: %w", err)
		}
		cfg.PollInterval = d
	}
	if cfg.UserID == "" || !numericUserID.MatchString(cfg.UserID) {
		return Config{}, fmt.Errorf("SB_USER_ID must contain only digits")
	}
	if cfg.Cookie == "" {
		return Config{}, fmt.Errorf("SB_COOKIE is required")
	}
	if strings.HasPrefix(strings.ToLower(cfg.Cookie), "cookie:") {
		return Config{}, fmt.Errorf("SB_COOKIE must contain only the header value, without the Cookie: prefix")
	}
	if cfg.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.TelegramChatID == "" {
		return Config{}, fmt.Errorf("TELEGRAM_CHAT_ID is required")
	}
	if cfg.PollInterval < 10*time.Second {
		return Config{}, fmt.Errorf("POLL_INTERVAL must be at least 10s")
	}
	return cfg, nil
}
