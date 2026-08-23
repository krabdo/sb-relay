package relay

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

const alertInterval = 6 * time.Hour

type AppOptions struct {
	PollInterval time.Duration
	MaxPages     int
	MaxSeen      int
	Logger       *log.Logger
}

type App struct {
	forum        Forum
	sender       Sender
	store        StateStore
	pollInterval time.Duration
	maxPages     int
	maxSeen      int
	logger       *log.Logger
	seen         *seenSet
	initialized  bool
	lastAlert    time.Time
}

type fatalError struct{ err error }

func (e fatalError) Error() string { return e.err.Error() }
func (e fatalError) Unwrap() error { return e.err }

func NewApp(forum Forum, sender Sender, store StateStore, options AppOptions) *App {
	logger := options.Logger
	if logger == nil {
		logger = log.Default()
	}
	return &App{
		forum:        forum,
		sender:       sender,
		store:        store,
		pollInterval: options.PollInterval,
		maxPages:     options.MaxPages,
		maxSeen:      options.MaxSeen,
		logger:       logger,
	}
}

func (a *App) Run(ctx context.Context) error {
	if err := a.loadState(); err != nil {
		return err
	}
	a.logger.Printf("sb-relay started; poll_interval=%s", a.pollInterval)
	delay := time.Duration(0)
	for {
		if delay > 0 {
			if err := sleepContext(ctx, delay); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
		}
		err := a.Poll(ctx)
		if err == nil {
			delay = a.pollInterval
			continue
		}
		var fatal fatalError
		if errors.As(err, &fatal) {
			return fatal.err
		}
		if errors.Is(err, context.Canceled) {
			return nil
		}
		a.logger.Printf("poll failed: %v", err)
		if errors.Is(err, ErrAuthentication) || errors.Is(err, ErrPageStructure) {
			a.sendOperationalAlert(ctx, err)
		}
		if delay < a.pollInterval {
			delay = a.pollInterval
		} else {
			delay *= 2
			if delay > 15*time.Minute {
				delay = 15 * time.Minute
			}
		}
	}
}

func (a *App) loadState() error {
	state, exists, err := a.store.Load()
	if err != nil {
		return fatalError{fmt.Errorf("load persistent state: %w", err)}
	}
	a.seen = newSeenSet(state.Seen, a.maxSeen)
	a.initialized = exists
	return nil
}

func (a *App) Poll(ctx context.Context) error {
	if a.seen == nil {
		if err := a.loadState(); err != nil {
			return err
		}
	}
	if !a.initialized {
		return a.firstPoll(ctx)
	}
	return a.regularPoll(ctx)
}

func (a *App) firstPoll(ctx context.Context) error {
	page, err := a.forum.FetchPage(ctx, 1)
	if err != nil {
		return err
	}
	items := page.Notifications
	for i := len(items) - 1; i >= 1; i-- {
		a.seen.Add(items[i].ID)
	}
	if err := a.saveState(); err != nil {
		return err
	}
	a.initialized = true
	if len(items) == 0 {
		a.logger.Printf("initial baseline created; no current notifications")
		return nil
	}
	a.logger.Printf("initial baseline created; sending latest notification")
	return a.deliver(ctx, []Notification{items[0]})
}

func (a *App) regularPoll(ctx context.Context) error {
	var unseen []Notification
	collected := make(map[string]struct{})
	foundBoundary := false
	for pageNumber := 1; pageNumber <= a.maxPages; pageNumber++ {
		page, err := a.forum.FetchPage(ctx, pageNumber)
		if err != nil {
			return err
		}
		for _, item := range page.Notifications {
			if a.seen.Has(item.ID) {
				foundBoundary = true
				break
			}
			if _, duplicate := collected[item.ID]; !duplicate {
				unseen = append(unseen, item)
				collected[item.ID] = struct{}{}
			}
		}
		if foundBoundary || !page.HasNext {
			break
		}
	}
	if len(unseen) == 0 {
		return nil
	}
	if !foundBoundary {
		a.logger.Printf("warning: no known notification found within %d pages; forwarding collected notifications", a.maxPages)
	}
	for left, right := 0, len(unseen)-1; left < right; left, right = left+1, right-1 {
		unseen[left], unseen[right] = unseen[right], unseen[left]
	}
	return a.deliver(ctx, unseen)
}

func (a *App) deliver(ctx context.Context, items []Notification) error {
	for _, item := range items {
		if err := a.sender.Send(ctx, FormatNotification(item)); err != nil {
			return fmt.Errorf("send notification %s: %w", item.ID, err)
		}
		a.seen.Add(item.ID)
		if err := a.saveState(); err != nil {
			return err
		}
		a.logger.Printf("forwarded notification id=%s kind=%q", item.ID, item.Kind)
	}
	return nil
}

func (a *App) saveState() error {
	if err := a.store.Save(a.seen.State()); err != nil {
		return fatalError{fmt.Errorf("persist state: %w", err)}
	}
	return nil
}

func (a *App) sendOperationalAlert(ctx context.Context, cause error) {
	if !a.lastAlert.IsZero() && time.Since(a.lastAlert) < alertInterval {
		return
	}
	message := FormatOperationalAlert(errors.Is(cause, ErrAuthentication))
	if err := a.sender.Send(ctx, message); err != nil {
		a.logger.Printf("failed to send operational alert: %v", err)
		return
	}
	a.lastAlert = time.Now()
}
