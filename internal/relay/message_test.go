package relay

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFormatNotificationEscapesAndTruncates(t *testing.T) {
	message := FormatNotification(Notification{
		Kind:      `<提及>`,
		Actor:     `A & B`,
		Content:   strings.Repeat(`<script>&`, 1000),
		TargetURL: `https://sb.sb/t/1/?a=1&b=2`,
	})
	if strings.Contains(message, "<script>") || !strings.Contains(message, "&lt;script&gt;") {
		t.Fatalf("content was not escaped: %s", message[:200])
	}
	if !strings.Contains(message, `href="https://sb.sb/t/1/?a=1&amp;b=2"`) {
		t.Fatalf("target URL was not escaped: %s", message[len(message)-150:])
	}
	if utf8.RuneCountInString(message) > maxTelegramMessageRunes {
		t.Fatalf("message too long: %d", utf8.RuneCountInString(message))
	}
	if !strings.Contains(message, "…") {
		t.Fatal("expected truncation marker")
	}
}

func TestFormatNotificationWithoutTargetOmitsLink(t *testing.T) {
	message := FormatNotification(Notification{Kind: "邀请", Actor: "用户", Content: "无目标链接的通知"})
	if strings.Contains(message, "查看原帖") || strings.Contains(message, `href=""`) {
		t.Fatalf("message contains an empty target link: %q", message)
	}
}
