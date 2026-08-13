package previewpanel

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func quickViewTitleRowGeom(panelWidth int) (titleX, innerRight, contentCols, y int) {
	const border = 1
	titleX = border + 1
	innerRight = panelWidth - border - 1
	contentCols = innerRight - titleX + 1
	y = 0
	return titleX, innerRight, contentCols, y
}

func TestDrawPreviewChromaFrameFillsEmptyContentRows(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	githubBG := tcell.NewRGBColor(0xf7, 0xf7, 0xf7)
	chromaFrame := styles.PanelInactiveFrame.Background(githubBG)
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	Draw(screen, rect, State{
		Open:             true,
		TitleBase:        "sample.go",
		Source:           SourceInternalHighlighted,
		HighlightedCells: []AnsiCell{{R: 'x', St: body}},
	}, DrawParams{
		Theme:      styles,
		BodyStyle:  body,
		FrameStyle: chromaFrame,
	})

	contentTop := rect.Y + 1
	contentH := panelHeight - 1
	emptyRowY := contentTop + 1
	if emptyRowY >= contentTop+contentH {
		t.Fatal("panel too short for empty-row check")
	}
	_, style, _ := screen.Get(rect.X+2, emptyRowY)
	_, bg, _ := style.Decompose()
	r, g, b := rgb(bg)
	if r != 0xf7 || g != 0xf7 || b != 0xf7 {
		t.Fatalf("empty content row bg = #%02x%02x%02x, want #f7f7f7", r, g, b)
	}
}

func TestDrawPreviewChromaFrameUsesBackground(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	githubBG := tcell.NewRGBColor(0xf7, 0xf7, 0xf7)
	chromaFrame := styles.PanelInactiveFrame.Background(githubBG)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	Draw(screen, rect, State{Open: true, TitleBase: "sample.go"}, DrawParams{
		Theme:      styles,
		BodyStyle:  BodyStyle(styles, false),
		FrameStyle: chromaFrame,
	})

	_, style, _ := screen.Get(rect.X, rect.Y)
	_, bg, _ := style.Decompose()
	r, g, b := rgb(bg)
	if r != 0xf7 || g != 0xf7 || b != 0xf7 {
		t.Fatalf("corner border bg = #%02x%02x%02x, want #f7f7f7", r, g, b)
	}
	_, marginStyle, _ := screen.Get(rect.X+1, rect.Y+1)
	_, marginBG, _ := marginStyle.Decompose()
	mr, mg, mb := rgb(marginBG)
	if mr != 0xf7 || mg != 0xf7 || mb != 0xf7 {
		t.Fatalf("left margin bg = #%02x%02x%02x, want #f7f7f7", mr, mg, mb)
	}
	_, rightMarginStyle, _ := screen.Get(rect.X+panelWidth-2, rect.Y+1)
	_, rightMarginBG, _ := rightMarginStyle.Decompose()
	rr, rg, rb := rgb(rightMarginBG)
	if rr != 0xf7 || rg != 0xf7 || rb != 0xf7 {
		t.Fatalf("right margin bg = #%02x%02x%02x, want #f7f7f7", rr, rg, rb)
	}
}

func rgb(c tcell.Color) (r, g, b int) {
	cr, cg, cb := c.RGB()
	return int(cr), int(cg), int(cb)
}

func TestPaintQuickViewTitleRowClearsStaleFilenameOnShorterRedraw(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth = 80
	screen.SetSize(panelWidth, 4)

	styles := theme.Default()
	chrome := styles.PanelChrome(false, false)
	titleX, innerRight, contentCols, y := quickViewTitleRowGeom(panelWidth)
	panelPath := "/home/user/projects"
	home := "/home/user"

	longEnd := " verylongfilename.txt "
	shortEnd := " README.md "

	paintQuickViewTitleRow(screen, titleX, innerRight, contentCols, y,
		panelPath, home, chrome.Title, longEnd, chrome.Title, chrome.Frame)
	paintQuickViewTitleRow(screen, titleX, innerRight, contentCols, y,
		panelPath, home, chrome.Title, shortEnd, chrome.Title, chrome.Frame)

	got := tcelltest.TextAt(screen, titleX, y, contentCols)
	if strings.Contains(got, "verylong") {
		t.Fatalf("title row = %q, want no stale long filename", got)
	}
	if !strings.Contains(got, "README.md") {
		t.Fatalf("title row = %q, want short filename", got)
	}

	shortIdx := strings.Index(got, "README.md")
	if shortIdx < 0 {
		t.Fatal("README.md not found in title row")
	}
	afterName := got[shortIdx+len("README.md"):]
	if len(afterName) > 0 && afterName[0] == ' ' {
		afterName = afterName[1:]
	}
	if strings.Trim(afterName, "─") != "" {
		t.Fatalf("after filename = %q, want border dashes only", afterName)
	}
}

func TestPaintQuickViewTitleRowClearsVolumeLabelPrefill(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth = 80
	screen.SetSize(panelWidth, 4)

	styles := theme.Default()
	chrome := styles.PanelChrome(false, false)
	titleX, innerRight, contentCols, y := quickViewTitleRowGeom(panelWidth)
	panelPath := "/home/user/projects"
	home := "/home/user"

	volumeLabel := "──── 28G / 1817G (1%) ─"
	primitive.TextOverlay(screen, titleX, y, contentCols, volumeLabel, chrome.DiskUsageOverview)

	endLabel := " README.md "
	paintQuickViewTitleRow(screen, titleX, innerRight, contentCols, y,
		panelPath, home, chrome.Title, endLabel, chrome.Title, chrome.Frame)

	got := tcelltest.TextAt(screen, titleX, y, contentCols)
	if strings.Contains(got, "1817G") || strings.Contains(got, "%") {
		t.Fatalf("title row = %q, want no stale volume stats", got)
	}
	if !strings.Contains(got, "README.md") {
		t.Fatalf("title row = %q, want filename", got)
	}
}

func TestDrawEmbeddedPreviewChromaFramePaintsMarginColumns(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	githubBG := tcell.NewRGBColor(0xf7, 0xf7, 0xf7)
	chromaFrame := styles.PanelActiveFrame.Background(githubBG)
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	Draw(screen, rect, State{
		Open:             true,
		TitleBase:        "sample.go",
		Source:           SourceInternalHighlighted,
		HighlightedCells: []AnsiCell{{R: 'x', St: body}},
	}, DrawParams{
		Theme:      styles,
		Embedded:   true,
		BodyStyle:  body,
		FrameStyle: chromaFrame,
	})

	for _, col := range []int{rect.X, rect.X + panelWidth - 1} {
		_, marginStyle, _ := screen.Get(col, rect.Y+1)
		_, marginBG, _ := marginStyle.Decompose()
		r, g, b := rgb(marginBG)
		if r != 0xf7 || g != 0xf7 || b != 0xf7 {
			t.Fatalf("embedded margin col %d bg = #%02x%02x%02x, want #f7f7f7", col, r, g, b)
		}
	}
}

func TestDrawBorderlessMarkdownPreviewPaintsOneSpaceMargin(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	Draw(screen, rect, State{
		Open:             true,
		TitleBase:        "README.md",
		Source:           SourceInternalHighlighted,
		IsMarkdown:       true,
		HighlightedCells: []AnsiCell{{R: 'x', St: body}},
	}, DrawParams{
		Theme:      styles,
		Borderless: true,
		BodyStyle:  body,
	})

	contentY := rect.Y + 1
	leftCh, _, _ := screen.Get(rect.X, contentY)
	if leftCh != " " {
		t.Fatalf("left margin col = %q, want blank", leftCh)
	}
	rightCh, _, _ := screen.Get(rect.X+panelWidth-1, contentY)
	if rightCh != " " {
		t.Fatalf("right margin col = %q, want blank", rightCh)
	}
	textCh, _, _ := screen.Get(rect.X+1, contentY)
	if textCh != "x" {
		t.Fatalf("first text col = %q, want 'x' one column in from the margin", textCh)
	}
}

func TestDrawEmbeddedPreviewTitleRowEdgesUseTitleBackground(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	githubBG := tcell.NewRGBColor(0xf7, 0xf7, 0xf7)
	chromaFrame := styles.PanelActiveFrame.Background(githubBG)
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	Draw(screen, rect, State{
		Open:             true,
		TitleBase:        "sample.go",
		Source:           SourceInternalHighlighted,
		HighlightedCells: []AnsiCell{{R: 'x', St: body}},
	}, DrawParams{
		Theme:      styles,
		Embedded:   true,
		BodyStyle:  body,
		FrameStyle: chromaFrame,
	})

	// The title row (rect.Y) edge columns must keep the title background, not the
	// chroma frame color used for content-row margins.
	_, wantBG, _ := styles.PanelChrome(true, false).HeaderCarousel.Decompose()
	wr, wg, wb := rgb(wantBG)
	for _, col := range []int{rect.X, rect.X + panelWidth - 1} {
		_, style, _ := screen.Get(col, rect.Y)
		_, bg, _ := style.Decompose()
		r, g, b := rgb(bg)
		if r != wr || g != wg || b != wb {
			t.Fatalf("title row edge col %d bg = #%02x%02x%02x, want title bg #%02x%02x%02x", col, r, g, b, wr, wg, wb)
		}
	}
}

func TestDrawEmbeddedPreviewMessageUsesThemeMarginsNotChroma(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	githubBG := tcell.NewRGBColor(0xf7, 0xf7, 0xf7)
	chromaFrame := styles.PanelActiveFrame.Background(githubBG)
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	Draw(screen, rect, State{
		Open:      true,
		TitleBase: "binary.bin",
		ErrorMsg:  "Not a text file",
	}, DrawParams{
		Theme:      styles,
		Embedded:   true,
		BodyStyle:  body,
		FrameStyle: chromaFrame,
	})

	// A message state has no chroma body, so the content margins must use the panel
	// surface, never the chroma frame color.
	_, wantBG, _ := body.Decompose()
	wr, wg, wb := rgb(wantBG)
	contentRow := rect.Y + 1
	for _, col := range []int{rect.X, rect.X + panelWidth - 1} {
		_, style, _ := screen.Get(col, contentRow)
		_, bg, _ := style.Decompose()
		r, g, b := rgb(bg)
		if r == 0xf7 && g == 0xf7 && b == 0xf7 {
			t.Fatalf("message margin col %d uses chroma frame bg #f7f7f7, want theme surface", col)
		}
		if r != wr || g != wg || b != wb {
			t.Fatalf("message margin col %d bg = #%02x%02x%02x, want surface #%02x%02x%02x", col, r, g, b, wr, wg, wb)
		}
	}
}

func TestDrawEmbeddedPreviewTitleShowsFilenameOnly(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	Draw(screen, rect, State{
		Open:      true,
		TitleBase: "go.mod",
	}, DrawParams{
		Theme:     styles,
		Embedded:  true,
		BodyStyle: BodyStyle(styles, false),
	})

	titleRow := tcelltest.TextAt(screen, rect.X+1, rect.Y, rect.Width-2)
	if strings.Contains(titleRow, "─") {
		t.Fatalf("title row = %q, want no border dashes in embedded preview title", titleRow)
	}
	if !strings.Contains(titleRow, "go.mod") {
		t.Fatalf("title row = %q, want filename only", titleRow)
	}
}

func TestPaintQuickViewTitleRowGapIsBorderDash(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth = 80
	screen.SetSize(panelWidth, 4)

	styles := theme.Default()
	chrome := styles.PanelChrome(false, false)
	titleX, innerRight, contentCols, y := quickViewTitleRowGeom(panelWidth)
	endLabel := " README.md "
	const endRightMargin = 1
	endRunes := utf8.RuneCountInString(endLabel)
	endStartCol := innerRight - endRunes + 1 - endRightMargin - titleX
	gapStart := endStartCol - gapBeforePanelTitleEnd

	paintQuickViewTitleRow(screen, titleX, innerRight, contentCols, y,
		"/home/user/projects", "/home/user", chrome.Title, endLabel, chrome.Title, chrome.Frame)

	gap := tcelltest.TextAt(screen, titleX+gapStart, y, gapBeforePanelTitleEnd)
	if gap != strings.Repeat("─", gapBeforePanelTitleEnd) {
		t.Fatalf("gap = %q, want %q", gap, strings.Repeat("─", gapBeforePanelTitleEnd))
	}
	// Trailing frame dash between end label and right border corner.
	trail := tcelltest.TextAt(screen, innerRight, y, 1)
	if trail != "─" {
		t.Fatalf("trailing cell = %q, want ─", trail)
	}
}

// overflowingLines returns HighlightedCells for n one-character lines, more than enough
// to overflow any contentH used by these tests.
func overflowingLines(n int, body tcell.Style) []AnsiCell {
	cells := make([]AnsiCell, 0, n*2)
	for range n {
		cells = append(cells, AnsiCell{R: 'x', St: body}, AnsiCell{R: '\n', St: body})
	}
	return cells
}

func TestDrawBoxedPreviewScrollbarVisibleRegardlessOfFocus(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	gutterX := rect.X + panelWidth - 1

	draw := func(focused bool) {
		Draw(screen, rect, State{
			Open:             true,
			TitleBase:        "sample.go",
			Source:           SourceInternalHighlighted,
			HighlightedCells: overflowingLines(20, body),
		}, DrawParams{
			Theme:          styles,
			PreviewFocused: focused,
			BodyStyle:      body,
			FrameStyle:     styles.PanelActiveFrame,
			ScrollbarStyle: uiscrollbar.StyleThumb,
		})
	}

	// Quick view's boxed preview usually renders unfocused (the user is browsing the
	// *other* panel), so the thumb must still show — hiding it here was the reported bug.
	contentH := panelHeight - 2
	hasThumbGlyph := func() bool {
		for row := 0; row < contentH; row++ {
			c, _, _ := screen.Get(gutterX, rect.Y+1+row)
			if c != "│" {
				return true
			}
		}
		return false
	}

	draw(true)
	if !hasThumbGlyph() {
		t.Fatal("no scrollbar thumb glyph found while focused, want one distinct from the plain border")
	}

	draw(false)
	if !hasThumbGlyph() {
		t.Fatal("no scrollbar thumb glyph found while unfocused (quick view), want it to still be visible")
	}
}

func TestDrawBoxedPreviewScrollbarHiddenWhenContentFits(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}

	Draw(screen, rect, State{
		Open:             true,
		TitleBase:        "sample.go",
		Source:           SourceInternalHighlighted,
		HighlightedCells: []AnsiCell{{R: 'x', St: body}},
	}, DrawParams{
		Theme:          styles,
		PreviewFocused: true,
		BodyStyle:      body,
		FrameStyle:     styles.PanelActiveFrame,
		ScrollbarStyle: uiscrollbar.StyleThumb,
	})

	ch, _, _ := screen.Get(rect.X+panelWidth-1, rect.Y+1)
	if ch != "│" {
		t.Fatalf("gutter col with content that fits = %q, want plain border (nothing to scroll)", ch)
	}
}

func TestDrawPlainBorderlessPreviewReservesScrollbarGutter(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	gutterX := rect.X + panelWidth - 1

	Draw(screen, rect, State{
		Open:             true,
		TitleBase:        "sample.go",
		Source:           SourceInternalHighlighted,
		HighlightedCells: overflowingLines(20, body),
	}, DrawParams{
		Theme:          styles,
		Borderless:     true,
		PreviewFocused: true,
		BodyStyle:      body,
		FrameStyle:     styles.PanelActiveFrame,
		ScrollbarStyle: uiscrollbar.StyleThumb,
	})

	lastTextCh, _, _ := screen.Get(gutterX, rect.Y+1)
	if lastTextCh == "x" {
		t.Fatalf("last column = %q, want scrollbar gutter reserved (not text)", lastTextCh)
	}
}

func TestDrawEmbeddedPreviewScrollGutterXOverridesToPanelBorder(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	body := BodyStyle(styles, false)
	// The carousel child column's own rect (panelWidth) stops one column short of the
	// enclosing panel's real border; ScrollGutterX simulates that border column, one past
	// this rect's own right edge.
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	panelBorderX := rect.X + panelWidth

	Draw(screen, rect, State{
		Open:             true,
		TitleBase:        "sample.go",
		Source:           SourceInternalHighlighted,
		HighlightedCells: overflowingLines(20, body),
	}, DrawParams{
		Theme:            styles,
		Embedded:         true,
		BodyStyle:        body,
		FrameStyle:       styles.PanelActiveFrame,
		ScrollbarStyle:   uiscrollbar.StyleThumb,
		HasScrollGutterX: true,
		ScrollGutterX:    panelBorderX,
	})

	foundThumb := false
	for row := 0; row < panelHeight-1; row++ {
		if c, _, _ := screen.Get(panelBorderX, rect.Y+1+row); c != "│" {
			foundThumb = true
			break
		}
	}
	if !foundThumb {
		t.Fatal("no scrollbar glyph found at overridden ScrollGutterX column")
	}
	// The rect's own margin column (one left of the override) must stay a plain blank
	// margin — the scrollbar must not also paint there.
	rightMarginX := rect.X + panelWidth - 1
	for row := 0; row < panelHeight-1; row++ {
		if c, _, _ := screen.Get(rightMarginX, rect.Y+1+row); c != " " {
			t.Fatalf("embedded margin col at row %d = %q, want blank (scrollbar should be at the override column only)", row, c)
		}
	}
}

func TestDrawFullscreenPreviewScrollbarRailStyleOverride(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 10
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	gutterX := rect.X + panelWidth - 1
	railStyle := tcell.StyleDefault.Foreground(tcell.NewRGBColor(0x75, 0x71, 0x5e))

	// Overflowing content so the scrollbar actually paints; the non-thumb rows use the
	// plain rail glyph ('│'), which should carry the override style.
	Draw(screen, rect, State{
		Open:             true,
		TitleBase:        "sample.go",
		Source:           SourceInternalHighlighted,
		HighlightedCells: overflowingLines(20, body),
	}, DrawParams{
		Theme:              styles,
		Borderless:         true,
		PreviewFocused:     true,
		BodyStyle:          body,
		FrameStyle:         styles.PanelActiveFrame,
		ScrollbarStyle:     uiscrollbar.StyleThumb,
		ScrollbarRailStyle: railStyle,
	})

	found := false
	for row := 0; row < panelHeight-1; row++ {
		c, st, _ := screen.Get(gutterX, rect.Y+1+row)
		if c == "│" {
			found = true
			if st != railStyle {
				t.Fatalf("gutter rail style at row %d = %v, want override %v", row, st, railStyle)
			}
		}
	}
	if !found {
		t.Fatal("no plain rail glyph found to check style against (content should overflow but not fill every row)")
	}
}

func TestDrawImageRecordsPlacementAndBlanksBody(t *testing.T) {
	_ = TakeFrameImage() // clear any prior frame
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 12
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	payload := "\x1bP0;0;8q#0;2;0;0;0#0~~~\x1b\\"
	Draw(screen, rect, State{
		Open:          true,
		Path:          "/tmp/garden.png",
		TitleBase:     "garden.png",
		Phase:         PhaseDone,
		ImagePayload:  payload,
		ImagePxW:      30,
		ImagePxH:      20,
		ImageProtocol: ImageProtocolSixel,
	}, DrawParams{
		Theme:     styles,
		BodyStyle: body,
	})

	plan := TakeFrameImage()
	if plan == nil {
		t.Fatal("TakeFrameImage() = nil, want placement")
	}
	if plan.Payload != payload {
		t.Fatalf("Payload mismatch")
	}
	if plan.Path != "/tmp/garden.png" {
		t.Fatalf("Path = %q", plan.Path)
	}
	if plan.X != rect.X+2 || plan.Y != rect.Y+1 {
		t.Fatalf("origin = (%d,%d), want (%d,%d)", plan.X, plan.Y, rect.X+2, rect.Y+1)
	}
	wantRows := geom.JobsPanelContentRows(geom.Rect(rect))
	if plan.MaxCols != rect.Width-4 || plan.MaxRows != wantRows {
		t.Fatalf("span budget = %d×%d, want %d×%d", plan.MaxCols, plan.MaxRows, rect.Width-4, wantRows)
	}
	if plan.PxW != 30 || plan.PxH != 20 {
		t.Fatalf("px = %d×%d", plan.PxW, plan.PxH)
	}

	main, _, _ := screen.Get(plan.X, plan.Y)
	if main != " " {
		t.Fatalf("body cell = %q, want space", main)
	}
	if TakeFrameImage() != nil {
		t.Fatal("second TakeFrameImage() should be nil")
	}
}

func rowText(screen tcell.SimulationScreen, y, x0, x1 int) string {
	var b strings.Builder
	for x := x0; x <= x1; x++ {
		main, _, _ := screen.Get(x, y)
		b.WriteString(main)
	}
	return b.String()
}

// TestDrawImageProtocolIndicatorSixelPlain covers the plain "Sixel" bottom-right label
// (Sixel protocol, not under tmux).
func TestDrawImageProtocolIndicatorSixelPlain(t *testing.T) {
	_ = TakeFrameImage()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 12
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	Draw(screen, rect, State{
		Open:          true,
		Path:          "/tmp/garden.png",
		Phase:         PhaseDone,
		ImagePayload:  "\x1bP0;0;8q#0;2;0;0;0#0~~~\x1b\\",
		ImagePxW:      30,
		ImagePxH:      20,
		ImageProtocol: ImageProtocolSixel,
		ImageInTmux:   false,
	}, DrawParams{Theme: styles, BodyStyle: BodyStyle(styles, false)})

	bottomRow := rowText(screen, rect.Y+rect.Height-1, rect.X, rect.X+rect.Width-1)
	if !strings.Contains(bottomRow, "Sixel") {
		t.Fatalf("bottom border row = %q, want it to contain %q", bottomRow, "Sixel")
	}
	if strings.Contains(bottomRow, "Tmux") {
		t.Fatalf("bottom border row = %q, want no Tmux label outside tmux", bottomRow)
	}
}

// TestDrawImageProtocolIndicatorSixelTmux covers the red "Sixel+Tmux" label for the
// known-flaky combination (see llm-docs/graphics-implementation-lessons.md lesson 11/12).
func TestDrawImageProtocolIndicatorSixelTmux(t *testing.T) {
	_ = TakeFrameImage()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 12
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	Draw(screen, rect, State{
		Open:          true,
		Path:          "/tmp/garden.png",
		Phase:         PhaseDone,
		ImagePayload:  "\x1bP0;0;8q#0;2;0;0;0#0~~~\x1b\\",
		ImagePxW:      30,
		ImagePxH:      20,
		ImageProtocol: ImageProtocolSixel,
		ImageInTmux:   true,
	}, DrawParams{Theme: styles, BodyStyle: BodyStyle(styles, false)})

	bottomRow := rowText(screen, rect.Y+rect.Height-1, rect.X, rect.X+rect.Width-1)
	if !strings.Contains(bottomRow, "Sixel+Tmux") {
		t.Fatalf("bottom border row = %q, want it to contain %q", bottomRow, "Sixel+Tmux")
	}

	labelX := rect.X + rect.Width - 2 - 1 - len([]rune(" Sixel+Tmux "))
	_, st, _ := screen.Get(labelX+1, rect.Y+rect.Height-1)
	fg, _, _ := st.Decompose()
	wantFg, _, _ := styles.PanelStatusImageSixelTmux.Decompose()
	if fg != wantFg {
		t.Fatalf("label style fg = %v, want %v (PanelStatusImageSixelTmux)", fg, wantFg)
	}
}

// TestDrawImageProtocolIndicatorKittyOmitted guards against showing the Sixel indicator for
// Kitty images — only Sixel has the known display gap this label calls out.
func TestDrawImageProtocolIndicatorKittyOmitted(t *testing.T) {
	_ = TakeFrameImage()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const panelWidth, panelHeight = 40, 12
	screen.SetSize(panelWidth, panelHeight)

	styles := theme.Default()
	rect := Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}
	Draw(screen, rect, State{
		Open:          true,
		Path:          "/tmp/garden.png",
		Phase:         PhaseDone,
		ImagePayload:  "\x1b_Ga=T,f=100;AAAA\x1b\\",
		ImagePxW:      30,
		ImagePxH:      20,
		ImageProtocol: ImageProtocolKitty,
		ImageInTmux:   true,
	}, DrawParams{Theme: styles, BodyStyle: BodyStyle(styles, false)})

	bottomRow := rowText(screen, rect.Y+rect.Height-1, rect.X, rect.X+rect.Width-1)
	if strings.Contains(bottomRow, "Sixel") {
		t.Fatalf("bottom border row = %q, want no Sixel label for Kitty", bottomRow)
	}
}

// TestDrawImagePlacementModeFollowsStateFlagNotEnvironment guards against Draw re-deriving
// placeholder mode from $TMUX: the decision is made upstream (capability-gated on the actual
// outer terminal) and carried in State.ImageUnicodePlaceholder. A Kitty image with the flag
// off must record a cursor-relative placement even with $TMUX set (tmux + WezTerm, which
// renders placeholder cells as garbage); with the flag on it must paint placeholder cells.
func TestDrawImagePlacementModeFollowsStateFlagNotEnvironment(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	styles := theme.Default()
	body := BodyStyle(styles, false)
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 10}
	baseState := State{
		Open:          true,
		TitleBase:     "photo.png",
		Source:        SourceExternalANSI,
		ImagePayload:  "\x1b_Ga=T,f=100,i=1;AAAA\x1b\\",
		ImagePxW:      100,
		ImagePxH:      100,
		ImageProtocol: ImageProtocolKitty,
	}

	for _, tc := range []struct {
		name            string
		placeholder     bool
		wantPlaceholder bool
	}{
		{"flag off records cursor-relative placement", false, false},
		{"flag on paints placeholder cells", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			defer screen.Fini()
			screen.SetSize(rect.Width, rect.Height)

			st := baseState
			st.ImageUnicodePlaceholder = tc.placeholder
			Draw(screen, rect, st, DrawParams{Theme: styles, BodyStyle: body})

			plan := TakeFrameImage()
			if plan == nil {
				t.Fatal("Draw recorded no ImagePlacement")
			}
			if plan.UnicodePlaceholder != tc.wantPlaceholder {
				t.Fatalf("plan.UnicodePlaceholder = %v, want %v", plan.UnicodePlaceholder, tc.wantPlaceholder)
			}
			cell, _, _ := screen.Get(rect.X+2, rect.Y+1)
			if gotCell := strings.HasPrefix(cell, string(unicodePlaceholderChar)); gotCell != tc.wantPlaceholder {
				t.Fatalf("first content cell placeholder rune = %v, want %v (cell %q)", gotCell, tc.wantPlaceholder, cell)
			}
		})
	}
}
