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
	UserID       string
	Cookie       string
	ShoutrrrURLs []string
	PollInterval time.Duration
	HTTPTimeout  time.Duration
	StateFile    string
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		UserID:       strings.TrimSpace(os.Getenv("SB_USER_ID")),
		Cookie:       strings.TrimSpace(os.Getenv("SB_COOKIE")),
		PollInterval: defaultPollInterval,
		HTTPTimeout:  defaultHTTPTimeout,
		StateFile:    strings.TrimSpace(os.Getenv("STATE_FILE")),
	}
	urls, err := loadShoutrrrURLs(
		strings.TrimSpace(os.Getenv("SHOUTRRR_URLS")),
		strings.TrimSpace(os.Getenv("SHOUTRRR_URLS_FILE")),
	)
	if err != nil {
		return Config{}, err
	}
	cfg.ShoutrrrURLs = urls
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
	if cfg.PollInterval < 10*time.Second {
		return Config{}, fmt.Errorf("POLL_INTERVAL must be at least 10s")
	}
	return cfg, nil
}

func loadShoutrrrURLs(inline, path string) ([]string, error) {
	if inline != "" && path != "" {
		return nil, fmt.Errorf("SHOUTRRR_URLS and SHOUTRRR_URLS_FILE are mutually exclusive")
	}
	var values []string
	switch {
	case inline != "":
		values = strings.Fields(inline)
	case path != "":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read SHOUTRRR_URLS_FILE: %w", err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if value := strings.TrimSpace(line); value != "" {
				values = append(values, value)
			}
		}
	default:
		return nil, fmt.Errorf("exactly one of SHOUTRRR_URLS or SHOUTRRR_URLS_FILE is required")
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("Shoutrrr destination list must not be empty")
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return nil, fmt.Errorf("duplicate Shoutrrr destination URL")
		}
		seen[value] = struct{}{}
	}
	return values, nil
}
