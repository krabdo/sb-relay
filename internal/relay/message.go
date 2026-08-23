package relay

import (
	"html"
	"strings"
	"unicode/utf8"
)

const maxTelegramMessageRunes = 3900

func FormatNotification(n Notification) string {
	header := "<b>🔔 " + html.EscapeString(n.Kind) + "</b>"
	if n.Actor != "" {
		header += " · " + html.EscapeString(n.Actor)
	}
	footer := "\n\n<a href=\"" + html.EscapeString(n.TargetURL) + "\">查看原帖</a>"
	budget := maxTelegramMessageRunes - utf8.RuneCountInString(header) - utf8.RuneCountInString(footer) - 2
	content := escapeWithin(n.Content, budget)
	return header + "\n\n" + content + footer
}

func escapeWithin(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	truncated := false
	for _, r := range value {
		escaped := html.EscapeString(string(r))
		width := utf8.RuneCountInString(escaped)
		if used+width > maxRunes-1 {
			truncated = true
			break
		}
		b.WriteString(escaped)
		used += width
	}
	if truncated {
		b.WriteRune('…')
	}
	return b.String()
}
