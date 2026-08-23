package relay

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFormatNotificationTelegramEscapesAndLinks(t *testing.T) {
	message := FormatNotification(Notification{
		Kind:      "<提及>",
		Actor:     "A & B",
		Content:   strings.Repeat("<script>&", 1000),
		TargetURL: "https://sb.sb/t/1/?a=1&b=2",
	})
	if strings.Contains(message.TelegramHTML, "<script>") || !strings.Contains(message.TelegramHTML, "&lt;script&gt;") {
		t.Fatalf("content was not escaped: %s", message.TelegramHTML[:200])
	}
	if !strings.Contains(message.TelegramHTML, "href=\"https://sb.sb/t/1/?a=1&amp;b=2\"") {
		t.Fatalf("target URL was not escaped: %s", message.TelegramHTML[len(message.TelegramHTML)-150:])
	}
	if len(message.TelegramHTML) > maxMessageBytes || !utf8.ValidString(message.TelegramHTML) {
		t.Fatalf("invalid Telegram message: bytes=%d valid=%v", len(message.TelegramHTML), utf8.ValidString(message.TelegramHTML))
	}
	if !strings.Contains(message.TelegramHTML, "…") {
		t.Fatal("expected truncation marker")
	}
}

func TestFormatNotificationPlainTextHasNoHTML(t *testing.T) {
	message := FormatNotification(Notification{
		Kind:      "回复",
		Actor:     "Alice",
		Content:   "<b>原样文本</b>",
		TargetURL: "https://sb.sb/t/1/",
	})
	if strings.Contains(message.PlainText, "<a href=") || !strings.Contains(message.PlainText, "<b>原样文本</b>") {
		t.Fatalf("unexpected plain message: %q", message.PlainText)
	}
	if !strings.Contains(message.PlainText, "https://sb.sb/t/1/") {
		t.Fatalf("plain message lacks full URL: %q", message.PlainText)
	}
}

func TestFormatNotificationTruncatesChineseByBytes(t *testing.T) {
	message := FormatNotification(Notification{
		Kind:      "提及",
		Actor:     "用户",
		Content:   strings.Repeat("中文🙂", 2000),
		TargetURL: "https://sb.sb/t/1/",
	})
	for name, value := range map[string]string{"plain": message.PlainText, "telegram": message.TelegramHTML} {
		if len(value) > maxMessageBytes || !utf8.ValidString(value) {
			t.Fatalf("%s message invalid: bytes=%d valid=%v", name, len(value), utf8.ValidString(value))
		}
	}
}

func TestFormatOperationalAlertVariants(t *testing.T) {
	auth := FormatOperationalAlert(true)
	structure := FormatOperationalAlert(false)
	if !strings.Contains(auth.PlainText, "Cookie") || !strings.Contains(auth.TelegramHTML, "<b>") {
		t.Fatalf("unexpected auth alert: %+v", auth)
	}
	if !strings.Contains(structure.PlainText, "结构") || strings.Contains(structure.PlainText, "<b>") {
		t.Fatalf("unexpected structure alert: %+v", structure)
	}
}
