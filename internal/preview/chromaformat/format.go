package chromaformat

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

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
	Language    string
	StyleName   string
	BaseStyle   tcell.Style
	LineNumbers bool
	TabWidth    int
	// Partial, when set, is called once on the caller's goroutine as soon as PartialAfterCells
	// cells are highlighted, with the whole body: highlighted so far, the rest unstyled. It holds
	// the same runes as the final Result, so it can be shown while the rest is tokenised. A body
	// that finishes first gets no call.
	Partial           func(Result)
	PartialAfterCells int
}

// Result is highlighted terminal cells plus optional line-number gutter width.
type Result struct {
	Cells       []previewpanel.AnsiCell
	GutterWidth int
}

// cellWriter appends source text as cells, expanding tabs and emitting the line-number gutter.
type cellWriter struct {
	out         []previewpanel.AnsiCell
	tabWidth    int
	gutterFmt   string // empty without line numbers
	gutterStyle tcell.Style
	gutterWidth int
	lineNum     int
	col         int
	atLineStart bool
}

func (w *cellWriter) emitGutter() {
	if w.gutterFmt == "" {
		return
	}
	for _, r := range fmt.Sprintf(w.gutterFmt+" ", w.lineNum) {
		w.out = append(w.out, previewpanel.AnsiCell{R: r, St: w.gutterStyle})
	}
	w.col = w.gutterWidth
	w.atLineStart = false
}

func (w *cellWriter) write(text string, st tcell.Style) {
	for _, r := range text {
		if w.atLineStart {
			w.emitGutter()
		}
		switch r {
		case '\t':
			for range w.tabWidth - (w.col % w.tabWidth) {
				w.out = append(w.out, previewpanel.AnsiCell{R: ' ', St: st})
				w.col++
			}
		case '\n':
			w.out = append(w.out, previewpanel.AnsiCell{R: '\n', St: st})
			w.lineNum++
			w.col = 0
			w.atLineStart = true
		default:
			w.out = append(w.out, previewpanel.AnsiCell{R: r, St: st})
			w.col += runeWidth(r)
			w.atLineStart = false
		}
	}
}

// finish emits the gutter of the empty row after a trailing newline.
func (w *cellWriter) finish(source string) Result {
	if w.atLineStart && strings.HasSuffix(source, "\n") {
		w.emitGutter()
	}
	return Result{Cells: w.out, GutterWidth: w.gutterWidth}
}

// withPlainTail returns the whole body as cells: what w wrote so far, then rest in style st.
// w itself is left to continue.
func (w cellWriter) withPlainTail(source, rest string, st tcell.Style) Result {
	w.out = append(make([]previewpanel.AnsiCell, 0, cap(w.out)), w.out...)
	w.write(rest, st)
	return w.finish(source)
}

// normalizeLF rewrites CRLF and CR to LF, as Chroma does, so that token byte counts index source.
func normalizeLF(source string) string {
	if !strings.Contains(source, "\r") {
		return source
	}
	return strings.ReplaceAll(strings.ReplaceAll(source, "\r\n", "\n"), "\r", "\n")
}

// tokenCtxCheckMask spaces out ctx polls: a token is far cheaper than a context lookup.
const tokenCtxCheckMask = 0xff

// Highlight tokenises source with Chroma and maps tokens to AnsiCells. Cells carry no width
// dependence: soft wrapping happens at draw time. Tokens are pulled on demand, so a canceled ctx
// stops the run within a few hundred tokens and returns whatever was reached: the caller discards
// it after checking ctx.
func Highlight(ctx context.Context, source string, opts Options) Result {
	source = normalizeLF(source)
	style := chromastyles.Get(opts.StyleName)
	lineCount := countLines(source)
	w := cellWriter{tabWidth: opts.TabWidth, lineNum: 1, atLineStart: true}
	if w.tabWidth < 1 {
		w.tabWidth = config.DefaultPreviewTabWidth
	}
	if opts.LineNumbers && lineCount > 0 {
		digits := len(fmt.Sprintf("%d", lineCount))
		w.gutterFmt = fmt.Sprintf("%%%dd", digits)
		w.gutterWidth = cellStringWidth(fmt.Sprintf(w.gutterFmt+" ", lineCount))
		w.gutterStyle = StyleEntryToTcell(opts.BaseStyle, style.Get(chroma.LineNumbers))
	}
	// Bytes bound runes, so only tab expansion can outgrow this.
	w.out = make([]previewpanel.AnsiCell, 0, len(source)+lineCount*(w.gutterWidth))

	lexer := chroma.Coalesce(lexerFor(opts, source))
	it, err := lexer.Tokenise(nil, source)
	if err != nil {
		w.write(source, opts.BaseStyle)
		return w.finish(source)
	}

	partialDue := opts.Partial != nil
	consumed := 0
	tokens := 0
	for token := it(); token != chroma.EOF; token = it() {
		if tokens++; tokens&tokenCtxCheckMask == 0 && ctx.Err() != nil {
			break
		}
		w.write(token.Value, StyleEntryToTcell(opts.BaseStyle, style.Get(token.Type)))
		consumed += len(token.Value)
		if partialDue && len(w.out) >= opts.PartialAfterCells {
			partialDue = false
			if consumed < len(source) {
				tail := StyleEntryToTcell(opts.BaseStyle, style.Get(chroma.Text))
				opts.Partial(w.withPlainTail(source, source[consumed:], tail))
			}
		}
	}
	return w.finish(source)
}

// lexerFor resolves a lexer by Language (fenced code block info string) first,
// falling back to matching Path, then content analysis (e.g. shebang lines
// for extensionless scripts), then the plain-text fallback lexer.
func lexerFor(opts Options, source string) chroma.Lexer {
	if opts.Language != "" {
		if l := lexers.Get(opts.Language); l != nil {
			return l
		}
	}
	if l := lexers.Match(filepath.Base(opts.Path)); l != nil {
		return l
	}
	if l := lexers.Analyse(source[:min(len(source), config.DefaultPreviewAnalyseBytes)]); l != nil {
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
