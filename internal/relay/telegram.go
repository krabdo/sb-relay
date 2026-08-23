package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxTelegramBody = 1 << 20

type Sender interface {
	Send(context.Context, string) error
}

type TelegramClient struct {
	endpoint string
	chatID   string
	client   *http.Client
	sleep    func(context.Context, time.Duration) error
}

func NewTelegramClient(rawBaseURL, token, chatID string, timeout time.Duration) (*TelegramClient, error) {
	base, err := url.Parse(rawBaseURL)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return nil, errors.New("Telegram base URL must be a valid HTTPS URL")
	}
	if strings.ContainsAny(token, "/?#") {
		return nil, errors.New("Telegram bot token contains invalid URL characters")
	}
	return &TelegramClient{
		endpoint: strings.TrimRight(base.String(), "/") + "/bot" + token + "/sendMessage",
		chatID:   chatID,
		client:   &http.Client{Timeout: timeout},
		sleep:    sleepContext,
	}, nil
}

func (c *TelegramClient) Send(ctx context.Context, message string) error {
	payload := struct {
		ChatID             string `json:"chat_id"`
		Text               string `json:"text"`
		ParseMode          string `json:"parse_mode"`
		LinkPreviewOptions struct {
			IsDisabled bool `json:"is_disabled"`
		} `json:"link_preview_options"`
	}{ChatID: c.chatID, Text: message, ParseMode: "HTML"}
	payload.LinkPreviewOptions.IsDisabled = true
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Telegram message: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<(attempt-1)) * time.Second
			if err := c.sleep(ctx, delay); err != nil {
				return err
			}
		}
		retryAfter, retry, err := c.sendOnce(ctx, body)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retry {
			return err
		}
		if retryAfter > 0 {
			if err := c.sleep(ctx, retryAfter); err != nil {
				return err
			}
		}
	}
	return fmt.Errorf("Telegram send failed after retries: %w", lastErr)
}

func (c *TelegramClient) sendOnce(ctx context.Context, body []byte) (time.Duration, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, false, fmt.Errorf("create Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, true, fmt.Errorf("send Telegram request: %w", err)
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxTelegramBody))
	if readErr != nil {
		return 0, true, fmt.Errorf("read Telegram response: %w", readErr)
	}
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	_ = json.Unmarshal(data, &result)
	if resp.StatusCode == http.StatusOK && result.OK {
		return 0, false, nil
	}
	description := result.Description
	if description == "" {
		description = "HTTP " + strconv.Itoa(resp.StatusCode)
	}
	err = fmt.Errorf("Telegram API error: %s", description)
	if resp.StatusCode == http.StatusTooManyRequests {
		return time.Duration(result.Parameters.RetryAfter) * time.Second, true, err
	}
	return 0, resp.StatusCode >= 500, err
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
