package chromaformat

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/preview/chromastyles"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// Options configures Chroma highlighting for a single file body.
type Options struct {
	Path string
	// Language, when set, selects the lexer by name/alias (e.g. a fenced code block's
	// info string) instead of matching Path. Path is still used as a fallback.
	Language     string
	StyleName    string
	BaseStyle    tcell.Style
	LineNumbers  bool
	TabWidth     int
	ContentWidth int // wrap budget for code tokens (terminal width minus gutter)
}

// Result is highlighted terminal cells plus optional line-number gutter width.
type Result struct {
	Cells        []previewpanel.AnsiCell
	GutterWidth  int
	ContentWidth int
}

// Highlight tokenises source with Chroma and maps tokens to AnsiCells.
func Highlight(source string, opts Options) Result {
	tabWidth := opts.TabWidth
	if tabWidth < 1 {
		tabWidth = config.DefaultPreviewTabWidth
	}
	style := chromastyles.Get(opts.StyleName)
	lineCount := countLines(source)
	gutterWidth := 0
	var gutterFmt string
	var gutterStyle tcell.Style
	if opts.LineNumbers && lineCount > 0 {
		digits := len(fmt.Sprintf("%d", lineCount))
		gutterFmt = fmt.Sprintf("%%%dd", digits)
		gutterWidth = cellStringWidth(fmt.Sprintf(gutterFmt+" ", lineCount))
		gutterEntry := style.Get(chroma.LineNumbers)
		gutterStyle = StyleEntryToTcell(opts.BaseStyle, gutterEntry)
	}
	contentWidth := opts.ContentWidth
	if contentWidth < 1 {
		contentWidth = 1
	}

	lexer := lexerFor(opts)
	lexer = chroma.Coalesce(lexer)
	it, err := lexer.Tokenise(nil, source)
	if err != nil {
		return fallbackPlain(source, opts.BaseStyle, tabWidth, gutterFmt, gutterStyle, gutterWidth, contentWidth, opts.LineNumbers)
	}

	var out []previewpanel.AnsiCell
	lineNum := 1
	colOnLine := 0
	atLineStart := true
	emitGutter := func() {
		if !opts.LineNumbers || gutterFmt == "" {
			return
		}
		gutter := fmt.Sprintf(gutterFmt+" ", lineNum)
		for _, r := range gutter {
			out = append(out, previewpanel.AnsiCell{R: r, St: gutterStyle})
		}
		colOnLine = gutterWidth
		atLineStart = false
	}
	for token := it(); token != chroma.EOF; token = it() {
		entry := style.Get(token.Type)
		tokenStyle := StyleEntryToTcell(opts.BaseStyle, entry)
		for _, r := range token.Value {
			if atLineStart {
				emitGutter()
			}
			if r == '\t' {
				spaces := tabWidth - (colOnLine % tabWidth)
				if spaces < 1 {
					spaces = tabWidth
				}
				for range spaces {
					out = append(out, previewpanel.AnsiCell{R: ' ', St: tokenStyle})
					colOnLine++
				}
				continue
			}
			if r == '\n' {
				out = append(out, previewpanel.AnsiCell{R: '\n', St: tokenStyle})
				lineNum++
				colOnLine = 0
				atLineStart = true
				continue
			}
			out = append(out, previewpanel.AnsiCell{R: r, St: tokenStyle})
			colOnLine += runeWidth(r)
			atLineStart = false
		}
	}
	if atLineStart && opts.LineNumbers && strings.HasSuffix(source, "\n") {
		emitGutter()
	}
	return Result{Cells: out, GutterWidth: gutterWidth, ContentWidth: contentWidth}
}

// lexerFor resolves a lexer by Language (fenced code block info string) first,
// falling back to matching Path, then the plain-text fallback lexer.
func lexerFor(opts Options) chroma.Lexer {
	if opts.Language != "" {
		if l := lexers.Get(opts.Language); l != nil {
			return l
		}
	}
	if l := lexers.Match(filepath.Base(opts.Path)); l != nil {
		return l
	}
	return lexers.Fallback
}

func countLines(s string) int {
	if s == "" {
		return 1
	}
	return strings.Count(s, "\n") + 1
}

func fallbackPlain(source string, base tcell.Style, tabWidth int, gutterFmt string, gutterStyle tcell.Style, gutterWidth int, contentWidth int, lineNumbers bool) Result {
	var out []previewpanel.AnsiCell
	lineNum := 1
	colOnLine := 0
	atLineStart := true
	emitGutter := func() {
		if !lineNumbers || gutterFmt == "" {
			return
		}
		gutter := fmt.Sprintf(gutterFmt+" ", lineNum)
		for _, r := range gutter {
			out = append(out, previewpanel.AnsiCell{R: r, St: gutterStyle})
		}
		colOnLine = gutterWidth
		atLineStart = false
	}
	for _, r := range source {
		if atLineStart {
			emitGutter()
		}
		if r == '\t' {
			spaces := tabWidth - (colOnLine % tabWidth)
			if spaces < 1 {
				spaces = tabWidth
			}
			for range spaces {
				out = append(out, previewpanel.AnsiCell{R: ' ', St: base})
				colOnLine++
			}
			continue
		}
		if r == '\n' {
			out = append(out, previewpanel.AnsiCell{R: '\n', St: base})
			lineNum++
			colOnLine = 0
			atLineStart = true
			continue
		}
		if r == utf8.RuneError {
			r = '\ufffd'
		}
		out = append(out, previewpanel.AnsiCell{R: r, St: base})
		colOnLine += runeWidth(r)
		atLineStart = false
	}
	return Result{Cells: out, GutterWidth: gutterWidth, ContentWidth: contentWidth}
}

// StyleEntryToTcell converts a Chroma style entry to a tcell.Style, filling in
// unset foreground/background from base. Shared with mdformat so both packages
// derive terminal styles from Chroma style entries the same way.
func StyleEntryToTcell(base tcell.Style, entry chroma.StyleEntry) tcell.Style {
	baseFG, baseBG, _ := base.Decompose()
	fg := baseFG
	if entry.Colour.IsSet() {
		fg = tcell.NewRGBColor(int32(entry.Colour.Red()), int32(entry.Colour.Green()), int32(entry.Colour.Blue()))
	}
	bg := baseBG
	if entry.Background.IsSet() {
		bg = tcell.NewRGBColor(int32(entry.Background.Red()), int32(entry.Background.Green()), int32(entry.Background.Blue()))
	}
	out := base.Foreground(fg).Background(bg)
	if entry.Bold == chroma.Yes {
		out = out.Bold(true)
	}
	if entry.Italic == chroma.Yes {
		out = out.Italic(true)
	}
	if entry.Underline == chroma.Yes {
		out = out.Underline(true)
	}
	return out
}

func cellStringWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func runeWidth(r rune) int {
	if r < 0x20 || r == 0x7f {
		return 1
	}
	// runewidth is imported in previewpanel; keep local simple width for ASCII gutter.
	if r < 0x80 {
		return 1
	}
	return 1
}
