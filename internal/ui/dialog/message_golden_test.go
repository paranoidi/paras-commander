package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Render goldens for the generic message dialog: one line, two lines, blank-line paragraphs,
// wrapped long lines, an unbreakable word, overflowing bodies with the truncation ellipsis, the
// default and a long title, and narrow / short screens.

func messageGoldenText(lines int) string {
	words := []string{"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor"}
	out := make([]string, lines)
	for i := range out {
		out[i] = fmt.Sprintf("line %d %s %s", i, words[i%len(words)], words[(i*3+1)%len(words)])
	}
	return strings.Join(out, "\n")
}

func TestMessageGoldenRender(t *testing.T) {
	goldenBandRows["message"] = 24
	var cases []simpleGolden
	add := func(name string, w, h int, title, msg string) {
		st := MessageDialogState{Open: true, Title: title, Message: msg}
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawMessageDialog(s, l, st, asciiDialogStyles())
		}})
	}
	long := strings.Repeat("harbor lantern willow meadow ", 12)
	add("one_line", 80, 24, "Extract", "lantern failed")
	add("one_line_no_title", 80, 24, "", "lantern failed")
	add("blank_message", 80, 24, "Extract", "")
	add("two_lines", 80, 24, "Extract", "first harbor line\nsecond willow line")
	add("three_lines", 80, 24, "Extract", "first harbor line\nsecond willow line\nthird amber line")
	add("paragraphs", 80, 24, "User menu", "first paragraph here\n\nsecond paragraph here\n\n\nthird")
	add("crlf", 80, 24, "User menu", "first harbor\r\nsecond willow\r\n")
	add("wrapped_long", 80, 24, "Theme failed", long)
	add("wrapped_long_100", 100, 30, "Theme failed", long)
	add("unbreakable_word", 80, 24, "Extract", strings.Repeat("abcdefghij", 12))
	add("long_title", 80, 24, strings.Repeat("Lantern ", 12), "short body")
	add("title_wider_than_body", 80, 24, "Configuration could not be written", "tiny")
	add("exactly_18_lines", 80, 24, "Extract", messageGoldenText(18))
	add("overflow_19_lines", 80, 40, "Extract", messageGoldenText(19))
	add("overflow_30_lines", 80, 40, "Extract", messageGoldenText(30))
	add("overflow_long_last", 80, 40, "Extract", messageGoldenText(20)+"\n"+long)
	add("overflow_short_screen", 80, 14, "Extract", messageGoldenText(30))
	add("tall_in_short_screen_10", 80, 10, "Extract", messageGoldenText(12))
	add("tall_in_short_screen_8", 80, 8, "Extract", messageGoldenText(12))
	add("narrow_60", 60, 24, "Theme failed", long)
	add("narrow_40", 40, 24, "Theme failed", long)
	add("narrow_32", 32, 24, "Theme failed", long)
	add("narrow_30", 30, 24, "Theme failed", "harbor lantern willow meadow amber")
	add("narrow_20", 20, 24, "Theme failed", "harbor lantern willow meadow amber")
	add("wide_unicode", 80, 24, "Extract", "harbor 日本語の長いメッセージ lantern")
	runSimpleGoldens(t, "message", cases)
}
