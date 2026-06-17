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
