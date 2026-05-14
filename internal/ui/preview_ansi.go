package ui

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// AnsiCell is one display rune with a resolved style (after SGR interpretation).
type AnsiCell struct {
	R rune
	St tcell.Style
}

type sgrBuilder struct {
	fgSet, bgSet       bool
	fg, bg             tcell.Color
	bold, dim          bool
	italic, underline  bool
}

func clamp8(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return int32(n)
}

func ansi16(i int, bright bool) tcell.Color {
	if bright {
		return tcell.PaletteColor(8 + i)
	}
	return tcell.PaletteColor(i)
}

func parseSGRNums(param string) []int {
	if param == "" {
		return []int{0}
	}
	parts := strings.Split(param, ";")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			out = append(out, 0)
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			out = append(out, 0)
			continue
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return []int{0}
	}
	return out
}

func (b *sgrBuilder) style(base tcell.Style) tcell.Style {
	baseFG, baseBG, _ := base.Decompose()
	fg := baseFG
	if b.fgSet {
		fg = b.fg
	}
	bg := baseBG
	if b.bgSet {
		bg = b.bg
	}
	out := base.Foreground(fg).Background(bg)
	if b.bold {
		out = out.Bold(true)
	}
	if b.dim {
		out = out.Dim(true)
	}
	if b.italic {
		out = out.Italic(true)
	}
	if b.underline {
		out = out.Underline(true)
	}
	return out
}

func applySGRParams(nums []int, b *sgrBuilder) {
	for i := 0; i < len(nums); i++ {
		n := nums[i]
		switch {
		case n == 0:
			*b = sgrBuilder{}
		case n == 1:
			b.bold = true
		case n == 2:
			b.dim = true
		case n == 3:
			b.italic = true
		case n == 4:
			b.underline = true
		case n == 21 || n == 22:
			b.bold = false
			b.dim = false
		case n == 23:
			b.italic = false
		case n == 24:
			b.underline = false
		case n >= 30 && n <= 37:
			b.fg = ansi16(n-30, false)
			b.fgSet = true
		case n >= 90 && n <= 97:
			b.fg = ansi16(n-90, true)
			b.fgSet = true
		case n == 39:
			b.fgSet = false
		case n >= 40 && n <= 47:
			b.bg = ansi16(n-40, false)
			b.bgSet = true
		case n >= 100 && n <= 107:
			b.bg = ansi16(n-100, true)
			b.bgSet = true
		case n == 49:
			b.bgSet = false
		case n == 38:
			if i+1 < len(nums) {
				switch nums[i+1] {
				case 5:
					if i+2 < len(nums) {
						c := nums[i+2]
						if c >= 0 && c <= 255 {
							b.fg = tcell.PaletteColor(c)
							b.fgSet = true
						}
						i += 2
					}
				case 2:
					if i+4 < len(nums) {
						r := clamp8(nums[i+2])
						g := clamp8(nums[i+3])
						bb := clamp8(nums[i+4])
						b.fg = tcell.NewRGBColor(r, g, bb)
						b.fgSet = true
						i += 4
					}
				}
			}
		case n == 48:
			if i+1 < len(nums) {
				switch nums[i+1] {
				case 5:
					if i+2 < len(nums) {
						c := nums[i+2]
						if c >= 0 && c <= 255 {
							b.bg = tcell.PaletteColor(c)
							b.bgSet = true
						}
						i += 2
					}
				case 2:
					if i+4 < len(nums) {
						r := clamp8(nums[i+2])
						g := clamp8(nums[i+3])
						bb := clamp8(nums[i+4])
						b.bg = tcell.NewRGBColor(r, g, bb)
						b.bgSet = true
						i += 4
					}
				}
			}
		}
	}
}

// parseCSI consumes a CSI sequence starting immediately after ESC '['. Returns index past the sequence.
func parseCSI(s string, afterBracket int) (next int, final byte, param string) {
	j := afterBracket
	for j < len(s) {
		b := s[j]
		if b >= 0x40 && b <= 0x7e {
			return j + 1, b, s[afterBracket:j]
		}
		j++
	}
	return len(s), 0, s[afterBracket:]
}

// AnsiStyledCells expands s into styled cells using ECMA-48 SGR (CSI … m) and common escapes; non-UTF-8
// bytes become replacement characters. OSC sequences (ESC ]) are skipped.
func AnsiStyledCells(s string, base tcell.Style) []AnsiCell {
	var out []AnsiCell
	var st sgrBuilder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) {
			switch s[i+1] {
			case '[':
				next, final, param := parseCSI(s, i+2)
				if final == 'm' {
					applySGRParams(parseSGRNums(param), &st)
				}
				i = next
				continue
			case ']':
				// OSC: BEL or ST
				j := i + 2
				for j < len(s) {
					if s[j] == '\a' {
						j++
						break
					}
					if j+1 < len(s) && s[j] == '\x1b' && s[j+1] == '\\' {
						j += 2
						break
					}
					j++
				}
				i = j
				continue
			case '(', ')':
				if i+2 < len(s) {
					i += 3
					continue
				}
				i += 2
				continue
			default:
				if i+1 < len(s) {
					i += 2
					continue
				}
				i++
				continue
			}
		}
		r, w := utf8.DecodeRuneInString(s[i:])
		if w == 0 {
			break
		}
		i += w
		if r == '\r' {
			continue
		}
		out = append(out, AnsiCell{R: r, St: st.style(base)})
	}
	return out
}

// WrapAnsiCells breaks cells into visual lines of at most width terminal cells (width >= 1).
// A newline rune ends the current line (which may be empty) and starts the next.
func WrapAnsiCells(cells []AnsiCell, width int) [][]AnsiCell {
	if width < 1 {
		width = 1
	}
	if len(cells) == 0 {
		return [][]AnsiCell{{}}
	}
	var lines [][]AnsiCell
	var line []AnsiCell
	lineWidth := 0
	flushHard := func() {
		lines = append(lines, line)
		line = nil
		lineWidth = 0
	}
	for _, c := range cells {
		if c.R == '\n' {
			flushHard()
			continue
		}
		rw := runewidth.RuneWidth(c.R)
		if rw < 1 {
			rw = 1
		}
		if lineWidth+rw > width {
			if len(line) > 0 {
				lines = append(lines, line)
				line = nil
				lineWidth = 0
			}
		}
		line = append(line, c)
		lineWidth += rw
	}
	lines = append(lines, line)
	return lines
}
