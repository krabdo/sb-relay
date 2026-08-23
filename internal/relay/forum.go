package relay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxForumBody = 4 << 20

type Forum interface {
	FetchPage(context.Context, int) (NotificationPage, error)
}

type ForumClient struct {
	baseURL *url.URL
	userID  string
	cookie  string
	version string
	client  *http.Client
}

func NewForumClient(rawBaseURL, userID, cookie string, timeout time.Duration, version string) (*ForumClient, error) {
	baseURL, err := url.Parse(rawBaseURL)
	if err != nil || baseURL.Scheme != "https" || baseURL.Host == "" {
		return nil, errors.New("forum base URL must be a valid HTTPS URL")
	}
	return &ForumClient{
		baseURL: baseURL,
		userID:  userID,
		cookie:  cookie,
		version: version,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *ForumClient) FetchPage(ctx context.Context, page int) (NotificationPage, error) {
	if page < 1 {
		return NotificationPage{}, errors.New("page number must be positive")
	}
	path := "/u/" + url.PathEscape(c.userID) + "/"
	if page > 1 {
		path += "page/" + strconv.Itoa(page) + "/"
	}
	u := c.baseURL.ResolveReference(&url.URL{Path: path, RawQuery: "tab=notifications"})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return NotificationPage{}, fmt.Errorf("create forum request: %w", err)
	}
	req.Header.Set("Cookie", c.cookie)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.7")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; sb-relay/"+c.version+"; +https://github.com/krabdo/sb-relay)")

	resp, err := c.client.Do(req)
	if err != nil {
		return NotificationPage{}, fmt.Errorf("fetch forum page %d: %w", page, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || strings.HasPrefix(resp.Request.URL.Path, "/login/") {
		return NotificationPage{}, ErrAuthentication
	}
	if resp.StatusCode != http.StatusOK {
		return NotificationPage{}, fmt.Errorf("forum returned HTTP %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxForumBody+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return NotificationPage{}, fmt.Errorf("read forum response: %w", err)
	}
	if len(body) > maxForumBody {
		return NotificationPage{}, fmt.Errorf("forum response exceeds %d bytes", maxForumBody)
	}
	return ParseNotificationPage(strings.NewReader(string(body)), c.baseURL, page)
}
