package usermenu

import (
	"fmt"
	"strings"
)

const shortLoadErrorMaxRunes = 72

// ShortLoadError returns a compact user-facing explanation for menu load/decode failures.
func ShortLoadError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "menu.toml: entry ") {
		return truncateRunes(strings.TrimPrefix(msg, "menu.toml: "), shortLoadErrorMaxRunes)
	}
	msg = strings.TrimPrefix(msg, "menu.toml: ")
	if i := strings.Index(msg, "toml: "); i >= 0 {
		msg = strings.TrimSpace(msg[i+len("toml: "):])
	}
	if idx := strings.Index(msg, "(last key "); idx >= 0 {
		rest := msg[idx:]
		if end := strings.Index(rest, "): "); end >= 0 {
			key := extractQuotedKey(rest[len("(last key "):])
			detail := strings.TrimSpace(rest[end+3:])
			if key != "" {
				if short := shortTomlTypeMismatch(detail); short != "" {
					return truncateRunes(fmt.Sprintf("line %s: %s: %s", lineNumberPrefix(msg), key, short), shortLoadErrorMaxRunes)
				}
				return truncateRunes(fmt.Sprintf("line %s: %s: %s", lineNumberPrefix(msg), key, shortTomlDetail(detail)), shortLoadErrorMaxRunes)
			}
		}
	}
	if line := lineNumberPrefix(msg); line != "" {
		if rest := strings.TrimPrefix(msg, "line "+line); rest != msg {
			rest = strings.TrimSpace(strings.TrimPrefix(rest, ":"))
			if rest != "" {
				return truncateRunes("line "+line+": "+shortTomlDetail(rest), shortLoadErrorMaxRunes)
			}
		}
	}
	return truncateRunes(shortTomlDetail(msg), shortLoadErrorMaxRunes)
}

func extractQuotedKey(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' {
		if end := strings.Index(s[1:], "\""); end >= 0 {
			return s[1 : end+1]
		}
	}
	return ""
}

func lineNumberPrefix(msg string) string {
	msg = strings.TrimSpace(msg)
	if !strings.HasPrefix(msg, "line ") {
		return ""
	}
	rest := msg[5:]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return ""
	}
	return rest[:end]
}

func shortTomlTypeMismatch(detail string) string {
	d := strings.ToLower(detail)
	if strings.Contains(d, "incompatible types") {
		return "invalid value type"
	}
	return ""
}

func shortTomlDetail(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return "invalid menu.toml"
	}
	if short := shortTomlTypeMismatch(detail); short != "" {
		return short
	}
	if i := strings.Index(detail, "TOML value has type "); i >= 0 {
		return "invalid value type"
	}
	return detail
}

func truncateRunes(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return strings.TrimSpace(string(r[:maxRunes])) + "…"
}
