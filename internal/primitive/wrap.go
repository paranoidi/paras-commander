package primitive

import "strings"

// WrapText splits text into lines that fit maxCols runes, breaking at spaces when practical.
// Newlines end a line and blank lines stay as empty lines.
func WrapText(text string, maxCols int) []string {
	if maxCols < 1 {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimRight(para, "\r")
		if strings.TrimSpace(para) == "" {
			lines = append(lines, "")
			continue
		}
		runes := []rune(strings.TrimSpace(para))
		for len(runes) > 0 {
			if len(runes) <= maxCols {
				lines = append(lines, strings.TrimSpace(string(runes)))
				break
			}
			breakAt := maxCols
			minBreak := max(maxCols/3, 1)
			for i := maxCols - 1; i >= minBreak; i-- {
				if runes[i] == ' ' {
					breakAt = i
					break
				}
			}
			line := strings.TrimSpace(string(runes[:breakAt]))
			if line != "" {
				lines = append(lines, line)
			}
			runes = runes[breakAt:]
			for len(runes) > 0 && runes[0] == ' ' {
				runes = runes[1:]
			}
		}
	}
	return lines
}
