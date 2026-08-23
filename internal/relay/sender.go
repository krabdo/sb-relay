package relay

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/router"
	"github.com/containrrr/shoutrrr/pkg/types"
)

type Sender interface {
	Send(context.Context, OutboundMessage) error
}

type destinationRouter interface {
	Send(string, *types.Params) []error
}

type shoutrrrDestination struct {
	index  int
	scheme string
	router destinationRouter
}

type ShoutrrrSender struct {
	destinations []shoutrrrDestination
	logger       *log.Logger
}

func NewShoutrrrSender(rawURLs []string, timeout time.Duration, logger *log.Logger) (*ShoutrrrSender, error) {
	if logger == nil {
		logger = log.Default()
	}
	destinations := make([]shoutrrrDestination, 0, len(rawURLs))
	for i, rawURL := range rawURLs {
		scheme, normalized, err := normalizeDestinationURL(rawURL)
		if err != nil {
			return nil, fmt.Errorf("invalid Shoutrrr destination #%d: configuration rejected", i+1)
		}
		r, err := shoutrrr.NewSender(nil, normalized)
		if err != nil {
			return nil, fmt.Errorf("invalid Shoutrrr destination #%d (%s): configuration rejected", i+1, scheme)
		}
		r.Timeout = timeout
		destinations = append(destinations, shoutrrrDestination{index: i + 1, scheme: scheme, router: r})
	}
	if len(destinations) == 0 {
		return nil, errors.New("at least one Shoutrrr destination is required")
	}
	return &ShoutrrrSender{destinations: destinations, logger: logger}, nil
}

func normalizeDestinationURL(raw string) (string, string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return "", "", errors.New("missing or invalid scheme")
	}
	scheme := strings.ToLower(strings.Split(parsed.Scheme, "+")[0])
	if scheme == "telegram" {
		query := parsed.Query()
		for key := range query {
			if strings.EqualFold(key, "parsemode") || strings.EqualFold(key, "preview") {
				query.Del(key)
			}
		}
		query.Set("parsemode", "HTML")
		query.Set("preview", "No")
		parsed.RawQuery = query.Encode()
	}
	return scheme, parsed.String(), nil
}

func (s *ShoutrrrSender) Send(ctx context.Context, message OutboundMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	type result struct {
		destination shoutrrrDestination
		err         error
	}
	results := make(chan result, len(s.destinations))
	var wg sync.WaitGroup
	for _, destination := range s.destinations {
		wg.Add(1)
		go func(destination shoutrrrDestination) {
			defer wg.Done()
			results <- result{destination: destination, err: sendToDestination(ctx, destination, message)}
		}(destination)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	succeeded := 0
	failed := 0
	for result := range results {
		if result.err == nil {
			succeeded++
			continue
		}
		failed++
		s.logger.Printf("Shoutrrr destination #%d (%s) failed: %s", result.destination.index, result.destination.scheme, sanitizedDeliveryError(result.err))
	}
	if succeeded > 0 {
		if failed > 0 {
			s.logger.Printf("Shoutrrr delivery partially succeeded; succeeded=%d failed=%d", succeeded, failed)
		}
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("all %d Shoutrrr destinations failed", failed)
}

func sendToDestination(ctx context.Context, destination shoutrrrDestination, message OutboundMessage) error {
	body := message.PlainText
	params := types.Params{}
	if destination.scheme == "telegram" {
		body = message.TelegramHTML
	} else {
		params.SetTitle(message.Title)
	}
	done := make(chan []error, 1)
	go func() { done <- destination.router.Send(body, &params) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case errs := <-done:
		if len(errs) == 0 {
			return errors.New("destination returned no delivery result")
		}
		for _, err := range errs {
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func sanitizedDeliveryError(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "request canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "request timed out"
	case strings.Contains(strings.ToLower(err.Error()), "timed out"):
		return "request timed out"
	default:
		return "delivery rejected"
	}
}

var _ destinationRouter = (*router.ServiceRouter)(nil)

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
