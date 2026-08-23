package relay

import (
	"html"
	"strings"
)

const maxMessageBytes = 3900

type OutboundMessage struct {
	Title        string
	PlainText    string
	TelegramHTML string
}

func FormatNotification(n Notification) OutboundMessage {
	title := "sb-relay 通知"
	if n.Kind != "" {
		title += " · " + n.Kind
	}

	header := "<b>🔔 " + escapeWithinBytes(n.Kind, 256) + "</b>"
	if n.Actor != "" {
		header += " · " + escapeWithinBytes(n.Actor, 256)
	}
	footer := "\n\n<a href=\"" + escapeWithinBytes(n.TargetURL, 1200) + "\">查看原帖</a>"
	htmlBudget := maxMessageBytes - len(header) - len(footer) - 2
	telegramHTML := header + "\n\n" + escapeWithinBytes(n.Content, htmlBudget) + footer

	plainHeader := "🔔 " + truncateUTF8Bytes(n.Kind, 256)
	if n.Actor != "" {
		plainHeader += " · " + truncateUTF8Bytes(n.Actor, 256)
	}
	plainFooter := "\n\n查看原帖：" + truncateUTF8Bytes(n.TargetURL, 1200)
	plainBudget := maxMessageBytes - len(plainHeader) - len(plainFooter) - 2
	plain := plainHeader + "\n\n" + truncateUTF8Bytes(n.Content, plainBudget) + plainFooter
	return OutboundMessage{Title: truncateUTF8Bytes(title, 256), PlainText: plain, TelegramHTML: telegramHTML}
}

func FormatOperationalAlert(authentication bool) OutboundMessage {
	title := "sb-relay 抓取失败"
	body := "烧饼论坛通知页结构可能已经变化，请检查容器日志并升级 sb-relay。"
	if authentication {
		body = "烧饼论坛登录 Cookie 已失效或无权访问，请更新 SB_COOKIE 后重启容器。"
	}
	return OutboundMessage{
		Title:        title,
		PlainText:    "⚠️ " + title + "\n\n" + body,
		TelegramHTML: "<b>⚠️ " + html.EscapeString(title) + "</b>\n\n" + html.EscapeString(body),
	}
}

func escapeWithinBytes(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	truncated := false
	for _, r := range value {
		escaped := html.EscapeString(string(r))
		if used+len(escaped) > maxBytes-len("…") {
			truncated = true
			break
		}
		b.WriteString(escaped)
		used += len(escaped)
	}
	if truncated && used+len("…") <= maxBytes {
		b.WriteRune('…')
	}
	return b.String()
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	var b strings.Builder
	for _, r := range value {
		width := len(string(r))
		if b.Len()+width > maxBytes-len("…") {
			break
		}
		b.WriteRune(r)
	}
	if b.Len()+len("…") <= maxBytes {
		b.WriteRune('…')
	}
	return b.String()
}
