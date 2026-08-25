package relay

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

const fixturePage = `<!doctype html><html><body><main>
<div class="tab-bar"><a class="tab active" href="/u/42/?tab=notifications">通知</a></div>
<ul class="post-list">
<li class="post-item notification-item">
  <div class="post-title-row"><a class="post-title" href="/u/7/">Alice</a><span class="notification-kind">回复</span></div>
  <div class="post-meta"><time datetime="2026-08-23T07:32:33Z">刚刚</time><span class="notification-where"><a href="/t/8/?reply_id=99">示例主题 #2</a></span></div>
  <div class="notification-content"><p>回复了你的主题：hello &amp; welcome</p></div>
  <a class="notification-reply-action" href="/t/8/?reply_id=99">查看</a>
</li>
<li class="post-item notification-item">
  <div class="post-title-row"><a class="post-title" href="/u/8/">Bob</a><span class="notification-kind">系统事件</span></div>
  <time datetime="2026-08-23T07:30:00Z">2分钟前</time>
  <div class="notification-content"><p>这是一种未来新增的通知</p></div>
  <a class="notification-reply-action" href="/events/abc">查看</a>
</li>
</ul>
<nav><a href="/u/42/page/2/?tab=notifications">下一页</a></nav>
</main></body></html>`

func TestParseNotificationPage(t *testing.T) {
	base, _ := url.Parse("https://sb.sb")
	page, err := ParseNotificationPage(strings.NewReader(fixturePage), base, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasNext || len(page.Notifications) != 2 {
		t.Fatalf("unexpected page: %+v", page)
	}
	first := page.Notifications[0]
	if first.ID != "reply:/t/8/:99:回复" {
		t.Fatalf("unexpected ID: %q", first.ID)
	}
	if first.Content != "回复了你的主题：hello & welcome" || first.TargetURL != "https://sb.sb/t/8/?reply_id=99" {
		t.Fatalf("unexpected notification: %+v", first)
	}
	second := page.Notifications[1]
	if second.Kind != "系统事件" || !strings.HasPrefix(second.ID, "hash:") {
		t.Fatalf("unknown notification type was not preserved: %+v", second)
	}
}

func TestParseNotificationPageAllowsMissingTopic(t *testing.T) {
	base, _ := url.Parse("https://sb.sb")
	html := strings.Replace(fixturePage, `<span class="notification-where"><a href="/t/8/?reply_id=99">示例主题 #2</a></span>`, "", 1)
	page, err := ParseNotificationPage(strings.NewReader(html), base, 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Notifications[0].TargetURL != "https://sb.sb/t/8/?reply_id=99" {
		t.Fatalf("reply action fallback failed: %+v", page.Notifications[0])
	}
}

func TestParseNotificationPageAllowsMissingTarget(t *testing.T) {
	base, _ := url.Parse("https://sb.sb")
	html := strings.Replace(fixturePage, `  <a class="notification-reply-action" href="/events/abc">查看</a>`, "", 1)
	page, err := ParseNotificationPage(strings.NewReader(html), base, 1)
	if err != nil {
		t.Fatal(err)
	}
	n := page.Notifications[1]
	if n.TargetURL != "" || !strings.HasPrefix(n.ID, "hash:") {
		t.Fatalf("unexpected linkless notification: %+v", n)
	}
}

func TestParseNotificationPageRejectsLoginAndShapeChanges(t *testing.T) {
	base, _ := url.Parse("https://sb.sb")
	_, err := ParseNotificationPage(strings.NewReader(`<html><body><form action="/login/"></form></body></html>`), base, 1)
	if !errors.Is(err, ErrAuthentication) {
		t.Fatalf("expected authentication error, got %v", err)
	}
	_, err = ParseNotificationPage(strings.NewReader(`<html><body>unexpected</body></html>`), base, 1)
	if !errors.Is(err, ErrPageStructure) {
		t.Fatalf("expected structure error, got %v", err)
	}
}
