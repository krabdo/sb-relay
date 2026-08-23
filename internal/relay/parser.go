package relay

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

var (
	ErrAuthentication = errors.New("forum authentication failed")
	ErrPageStructure  = errors.New("forum notification page structure changed")
)

func ParseNotificationPage(r io.Reader, baseURL *url.URL, pageNumber int) (NotificationPage, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return NotificationPage{}, fmt.Errorf("parse forum HTML: %w", err)
	}
	if isLoginDocument(doc) {
		return NotificationPage{}, ErrAuthentication
	}
	if !hasNotificationPageMarker(doc) {
		return NotificationPage{}, ErrPageStructure
	}

	nodes := findAll(doc, func(n *html.Node) bool { return hasClass(n, "notification-item") })
	result := NotificationPage{Notifications: make([]Notification, 0, len(nodes))}
	for i, node := range nodes {
		n, err := parseNotification(node, baseURL)
		if err != nil {
			return NotificationPage{}, fmt.Errorf("%w: item %d: %v", ErrPageStructure, i+1, err)
		}
		result.Notifications = append(result.Notifications, n)
	}

	nextFragment := "/page/" + strconv.Itoa(pageNumber+1) + "/"
	for _, link := range findAll(doc, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "a" }) {
		href := attr(link, "href")
		if strings.Contains(href, nextFragment) && strings.Contains(href, "tab=notifications") {
			result.HasNext = true
			break
		}
	}
	return result, nil
}

func parseNotification(node *html.Node, baseURL *url.URL) (Notification, error) {
	kindNode := findFirst(node, func(n *html.Node) bool { return hasClass(n, "notification-kind") })
	actorNode := findFirst(node, func(n *html.Node) bool { return n.Data == "a" && hasClass(n, "post-title") })
	contentNode := findFirst(node, func(n *html.Node) bool { return hasClass(n, "notification-content") })
	timeNode := findFirst(node, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "time" })
	targetNode := findFirst(node, func(n *html.Node) bool {
		return n.Data == "a" && hasClass(n, "notification-reply-action")
	})
	if targetNode == nil {
		where := findFirst(node, func(n *html.Node) bool { return hasClass(n, "notification-where") })
		if where != nil {
			targetNode = findFirst(where, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "a" })
		}
	}
	if kindNode == nil || contentNode == nil || timeNode == nil || targetNode == nil {
		return Notification{}, errors.New("required kind, content, time, or target element is missing")
	}

	kind := normalizedText(kindNode)
	content := normalizedText(contentNode)
	actor := ""
	if actorNode != nil {
		actor = normalizedText(actorNode)
	}
	if kind == "" || content == "" {
		return Notification{}, errors.New("kind or content is empty")
	}
	createdAt, err := time.Parse(time.RFC3339, attr(timeNode, "datetime"))
	if err != nil {
		return Notification{}, fmt.Errorf("invalid notification time: %w", err)
	}
	target, err := resolveForumURL(baseURL, attr(targetNode, "href"))
	if err != nil {
		return Notification{}, fmt.Errorf("invalid notification target: %w", err)
	}

	n := Notification{Kind: kind, Actor: actor, Content: content, TargetURL: target, CreatedAt: createdAt}
	n.ID = notificationID(n)
	return n, nil
}

func notificationID(n Notification) string {
	if parsed, err := url.Parse(n.TargetURL); err == nil {
		if replyID := parsed.Query().Get("reply_id"); replyID != "" {
			return "reply:" + parsed.Path + ":" + replyID + ":" + n.Kind
		}
	}
	canonical := strings.Join([]string{n.Kind, n.Actor, n.CreatedAt.UTC().Format(time.RFC3339Nano), n.TargetURL, n.Content}, "\x00")
	sum := sha256.Sum256([]byte(canonical))
	return "hash:" + hex.EncodeToString(sum[:])
}

func resolveForumURL(base *url.URL, raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || raw == "" {
		return "", errors.New("empty or malformed URL")
	}
	u = base.ResolveReference(u)
	if u.Scheme != "https" || !strings.EqualFold(u.Host, base.Host) {
		return "", errors.New("target must remain on the configured HTTPS forum host")
	}
	return u.String(), nil
}

func isLoginDocument(root *html.Node) bool {
	return findFirst(root, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "form" && strings.HasPrefix(attr(n, "action"), "/login/")
	}) != nil
}

func hasNotificationPageMarker(root *html.Node) bool {
	return findFirst(root, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "tab") && strings.Contains(attr(n, "href"), "tab=notifications")
	}) != nil
}

func findFirst(root *html.Node, predicate func(*html.Node) bool) *html.Node {
	if root == nil {
		return nil
	}
	if predicate(root) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirst(child, predicate); found != nil {
			return found
		}
	}
	return nil
}

func findAll(root *html.Node, predicate func(*html.Node) bool) []*html.Node {
	var result []*html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if predicate(n) {
			result = append(result, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return result
}

func hasClass(n *html.Node, className string) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	for _, class := range strings.Fields(attr(n, "class")) {
		if class == className {
			return true
		}
	}
	return false
}

func attr(n *html.Node, name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func normalizedText(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(current *html.Node) {
		if current.Type == html.TextNode {
			b.WriteString(current.Data)
			b.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
