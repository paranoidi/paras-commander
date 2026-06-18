package previewpanel

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
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
	endRunes := utf8.RuneCountInString(endLabel)
	endStartCol := innerRight - endRunes + 1 - titleX
	gapStart := endStartCol - gapBeforePanelTitleEnd

	paintQuickViewTitleRow(screen, titleX, innerRight, contentCols, y,
		"/home/user/projects", "/home/user", chrome.Title, endLabel, chrome.Title, chrome.Frame)

	gap := tcelltest.TextAt(screen, titleX+gapStart, y, gapBeforePanelTitleEnd)
	if gap != strings.Repeat("─", gapBeforePanelTitleEnd) {
		t.Fatalf("gap = %q, want %q", gap, strings.Repeat("─", gapBeforePanelTitleEnd))
	}
}
