package relay

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestForumClientDetectsForbiddenAndLoginRedirect(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
	}{
		{
			name: "forbidden",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			}),
		},
		{
			name: "login document",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`<html><body><form action="/login/"></form></body></html>`))
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(tt.handler)
			defer server.Close()
			base, _ := url.Parse(server.URL)
			client := &ForumClient{baseURL: base, userID: "42", cookie: "session=test", version: "test", client: server.Client()}
			_, err := client.FetchPage(context.Background(), 1)
			if !errors.Is(err, ErrAuthentication) {
				t.Fatalf("expected authentication error, got %v", err)
			}
		})
	}
}

func TestForumClientSendsCookieAndParsesPage(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "session=dummy" {
			t.Errorf("unexpected Cookie header: %q", got)
		}
		if r.URL.Path != "/u/42/" || r.URL.Query().Get("tab") != "notifications" {
			t.Errorf("unexpected request URL: %s", r.URL.String())
		}
		_, _ = w.Write([]byte(fixturePage))
	}))
	defer server.Close()
	base, _ := url.Parse(server.URL)
	client := &ForumClient{baseURL: base, userID: "42", cookie: "session=dummy", version: "test", client: server.Client()}
	page, err := client.FetchPage(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Notifications) != 2 {
		t.Fatalf("unexpected notification count: %d", len(page.Notifications))
	}
}
