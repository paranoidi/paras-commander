package previewpanel

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// AnsiCell is one display rune with a resolved style (after SGR interpretation).
type AnsiCell struct {
	R  rune
	St tcell.Style
}

type sgrBuilder struct {
	fgSet, bgSet      bool
	fg, bg            tcell.Color
	bold, dim         bool
	italic, underline bool
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

// parseExtendedColor parses the mode-selector + operand params of an SGR extended-color
// sequence (the part after the 38/48 introducer: 5;N for indexed, 2;R;G;B for direct RGB),
// starting at nums[i+1]. advance is how many extra params (2 or 4) were consumed so the caller
// can move its loop index past them; it is 0 when the sequence is incomplete (nothing
// consumed). ok is false when advance > 0 but the operand was out of range (color left unset,
// but the params are still consumed) — this mirrors the identical fg/bg parsing that used to be
// duplicated for cases 38 and 48.
func parseExtendedColor(nums []int, i int) (color tcell.Color, advance int, ok bool) {
	if i+1 >= len(nums) {
		return 0, 0, false
	}
	switch nums[i+1] {
	case 5:
		if i+2 >= len(nums) {
			return 0, 0, false
		}
		c := nums[i+2]
		if c >= 0 && c <= 255 {
			return tcell.PaletteColor(c), 2, true
		}
		return 0, 2, false
	case 2:
		if i+4 >= len(nums) {
			return 0, 0, false
		}
		r := clamp8(nums[i+2])
		g := clamp8(nums[i+3])
		bb := clamp8(nums[i+4])
		return tcell.NewRGBColor(r, g, bb), 4, true
	}
	return 0, 0, false
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
			if color, advance, ok := parseExtendedColor(nums, i); advance > 0 {
				if ok {
					b.fg = color
					b.fgSet = true
				}
				i += advance
			}
		case n == 48:
			if color, advance, ok := parseExtendedColor(nums, i); advance > 0 {
				if ok {
					b.bg = color
					b.bgSet = true
				}
				i += advance
			}
		}
	}
}

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

// AnsiStyledCells expands s into styled cells using ECMA-48 SGR.
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
