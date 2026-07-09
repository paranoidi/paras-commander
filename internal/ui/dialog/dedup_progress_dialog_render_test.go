package dialog

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func TestDrawDedupProgressDialogHashBarAndLabel(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	styles := theme.Default()
	layout := Layout{Width: 80, Height: 24}
	snap := comparepkg.DedupSnapshot{
		Root:           pathloc.MustParse("/scan/root"),
		Phase:          comparepkg.DedupHashing,
		Hashed:         1,
		HashTotal:      4,
		HashBytesTotal: 400,
		HashedBytes:    100,
		Current:        "nested",
	}
	DrawDedupProgressDialog(screen, layout, DedupProgressDialogState{Open: true}, snap, styles, "")

	rect := draw.CenteredDialogRect(layout, PreferredFormDialogWidth, dedupProgressDialogHeight(snap.Phase))
	barY := dedupHashProgressBarRow(screen, rect, snap.Hashed, snap.HashTotal)
	if barY < 0 {
		t.Fatal("hash progress bar row not found")
	}

	textX := draw.DialogTextX(rect)
	textW := draw.DialogContentWidth(rect)
	line := strings.TrimSpace(cellTextAt(screen, textX, barY, textW))
	if !strings.Contains(line, "nested") {
		t.Fatalf("bar line = %q, want current directory label", line)
	}
	if strings.Contains(line, "1/4") {
		t.Fatalf("bar line = %q, count must not be overlaid on bar", line)
	}

	hasBlock := false
	for col := textX; col < textX+textW; col++ {
		ch, _, _ := screen.Get(col, barY)
		if ch == "█" || ch == "░" {
			hasBlock = true
			break
		}
	}
	if !hasBlock {
		t.Fatal("progress row should include █ or ░ bar glyphs")
	}
	lastCh, _, _ := screen.Get(textX+textW-1, barY)
	if lastCh != "█" && lastCh != "░" {
		t.Fatalf("last bar col = %q, want █ or ░", lastCh)
	}

	_, wantFillBG, _ := styles.DialogProgressFill.Decompose()
	_, gotBG, _ := cellStyleAt(screen, textX, barY).Decompose()
	if gotBG != wantFillBG {
		t.Fatalf("first col bg %v, want fill %v", gotBG, wantFillBG)
	}

	countY, ok := dialogRowContaining(screen, rect, "1/4")
	if !ok {
		t.Fatal("hash count row not found")
	}
	if countY != barY+2 {
		t.Fatalf("count row y=%d, want barY+2=%d", countY, barY+2)
	}
	// No CurrentFile: the per-file bar row between bar and count stays dialog surface.
	_, wantSurface, _ := styles.DialogSurface.Decompose()
	_, fileRowBG, _ := cellStyleAt(screen, textX+textW/2, barY+1).Decompose()
	if fileRowBG != wantSurface {
		t.Fatalf("per-file row bg %v, want dialog surface %v", fileRowBG, wantSurface)
	}
	countLine := strings.TrimSpace(cellTextAt(screen, textX, countY, textW))
	if countLine != "1/4" {
		t.Fatalf("count line = %q, want 1/4", countLine)
	}
	_, wantSurfaceBG, _ := styles.DialogSurface.Decompose()
	_, countBG, _ := cellStyleAt(screen, textX+textW/2, countY).Decompose()
	if countBG != wantSurfaceBG {
		t.Fatalf("count row bg %v, want dialog surface %v", countBG, wantSurfaceBG)
	}
}

func TestDrawDedupProgressDialogWalkingStatus(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	styles := theme.Default()
	layout := Layout{Width: 80, Height: 24}
	snap := comparepkg.DedupSnapshot{
		Root:   pathloc.MustParse("/scan/root"),
		Phase:  comparepkg.DedupWalking,
		Walked: 42,
	}
	DrawDedupProgressDialog(screen, layout, DedupProgressDialogState{Open: true}, snap, styles, "")

	rect := draw.CenteredDialogRect(layout, PreferredFormDialogWidth, dedupProgressDialogHeight(snap.Phase))
	statusY, ok := dialogRowContaining(screen, rect, "Walking directories")
	if !ok {
		t.Fatal("walking status row not found")
	}

	textX := draw.DialogTextX(rect)
	textW := draw.DialogContentWidth(rect)
	line := strings.TrimSpace(cellTextAt(screen, textX, statusY, textW))
	if !strings.Contains(line, "42 files") {
		t.Fatalf("status line = %q, want file count 42 files", line)
	}

	_, wantBG, _ := styles.DialogSurface.Decompose()
	_, gotBG, _ := cellStyleAt(screen, textX, statusY).Decompose()
	if gotBG != wantBG {
		t.Fatalf("status bg %v, want dialog surface %v", gotBG, wantBG)
	}
}

func dialogRowContaining(screen tcell.Screen, rect draw.Rect, want string) (int, bool) {
	textX := draw.DialogTextX(rect)
	textW := draw.DialogContentWidth(rect)
	for y := rect.Y + 1; y < rect.Y+rect.Height-2; y++ {
		line := cellTextAt(screen, textX, y, textW)
		if strings.Contains(line, want) {
			return y, true
		}
	}
	return 0, false
}

func cellStyleAt(screen tcell.Screen, x, y int) tcell.Style {
	_, style, _ := screen.Get(x, y)
	return style
}

func TestDrawDedupHashProgressBarFullWidth(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	_, wantFillBG, _ := styles.DialogProgressFill.Decompose()
	_, wantTrackBG, _ := styles.DialogProgressTrack.Decompose()

	measure := func(t *testing.T, current string) (screen tcell.Screen, rect draw.Rect, barY, fillCols, trackCols int, lastChar rune) {
		t.Helper()
		screen = tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatalf("Init() error = %v", err)
		}
		t.Cleanup(func() { screen.Fini() })
		screen.SetSize(80, 24)

		layout := Layout{Width: 80, Height: 24}
		snap := comparepkg.DedupSnapshot{
			Root:           pathloc.MustParse("/scan/root"),
			Phase:          comparepkg.DedupHashing,
			Hashed:         14,
			HashTotal:      56,
			HashBytesTotal: 56,
			HashedBytes:    14,
			Current:        current,
		}
		DrawDedupProgressDialog(screen, layout, DedupProgressDialogState{Open: true}, snap, styles, "")

		rect = draw.CenteredDialogRect(layout, PreferredFormDialogWidth, dedupProgressDialogHeight(snap.Phase))
		textX := draw.DialogTextX(rect)
		textW := draw.DialogContentWidth(rect)
		barY = dedupHashProgressBarRow(screen, rect, snap.Hashed, snap.HashTotal)
		if barY < 0 {
			t.Fatal("hash progress bar row not found")
		}

		for col := textX; col < textX+textW; col++ {
			ch, style, _ := screen.Get(col, barY)
			r, _ := utf8.DecodeRuneInString(ch)
			_, bg, _ := style.Decompose()
			switch bg {
			case wantFillBG:
				fillCols++
			case wantTrackBG:
				trackCols++
			}
			if col == textX+textW-1 {
				lastChar = r
			}
		}
		return screen, rect, barY, fillCols, trackCols, lastChar
	}

	_, _, _, shortFill, shortTrack, shortLast := measure(t, "a")
	longScreen, longRect, longBarY, longFill, longTrack, _ := measure(t, strings.Repeat("segment/", 12)+"verylongfilename.txt")

	wantFilled := dedupBarFilledCols(draw.DialogContentWidth(draw.CenteredDialogRect(Layout{Width: 80, Height: 24}, PreferredFormDialogWidth, dedupProgressDialogHeight(comparepkg.DedupHashing))), dedupFrac(14, 56))
	wantTrack := draw.DialogContentWidth(draw.CenteredDialogRect(Layout{Width: 80, Height: 24}, PreferredFormDialogWidth, dedupProgressDialogHeight(comparepkg.DedupHashing))) - wantFilled

	if shortFill != wantFilled || longFill != wantFilled {
		t.Fatalf("fill cols short=%d long=%d, want %d", shortFill, longFill, wantFilled)
	}
	if shortTrack != wantTrack || longTrack != wantTrack {
		t.Fatalf("track cols short=%d long=%d, want %d", shortTrack, longTrack, wantTrack)
	}
	if shortFill != longFill || shortTrack != longTrack {
		t.Fatalf("meter width must not depend on label length")
	}
	// Short label leaves bar glyphs on the track; long label overlays text across full width.
	if shortLast != '░' && shortLast != '█' {
		t.Fatalf("short label last char = %q, want bar glyph", shortLast)
	}

	textX := draw.DialogTextX(longRect)
	textW := draw.DialogContentWidth(longRect)
	trackTextCols := 0
	for col := textX + wantFilled; col < textX+textW; col++ {
		ch, _, _ := longScreen.Get(col, longBarY)
		r, _ := utf8.DecodeRuneInString(ch)
		if r != '█' && r != '░' && r != ' ' {
			trackTextCols++
		}
	}
	if trackTextCols == 0 {
		t.Fatal("long label should render text on track region, not only bar glyphs")
	}
}

func TestDedupProgressTrackLabelBackgroundMatch(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	layout := Layout{Width: 80, Height: 24}
	snap := comparepkg.DedupSnapshot{
		Root:           pathloc.MustParse("/scan/root"),
		Phase:          comparepkg.DedupHashing,
		Hashed:         1,
		HashTotal:      4,
		HashBytesTotal: 4,
		HashedBytes:    1,
		Current:        strings.Repeat("seg/", 8) + "file.txt",
	}
	DrawDedupProgressDialog(screen, layout, DedupProgressDialogState{Open: true}, snap, styles, "")

	rect := draw.CenteredDialogRect(layout, PreferredFormDialogWidth, dedupProgressDialogHeight(snap.Phase))
	textX := draw.DialogTextX(rect)
	textW := draw.DialogContentWidth(rect)
	barY := dedupHashProgressBarRow(screen, rect, snap.Hashed, snap.HashTotal)
	if barY < 0 {
		t.Fatal("hash progress bar row not found")
	}

	_, wantTrackBG, _ := styles.DialogProgressTrack.Decompose()
	filled := dedupBarFilledCols(textW, dedupFrac(snap.HashedBytes, snap.HashBytesTotal))
	var trackBarBG, trackTextBG tcell.Color
	for col := textX + filled; col < textX+textW; col++ {
		ch, st, _ := screen.Get(col, barY)
		r, _ := utf8.DecodeRuneInString(ch)
		_, bg, _ := st.Decompose()
		if r == '░' && trackBarBG == tcell.ColorDefault {
			trackBarBG = bg
		}
		if r != '░' && r != '█' && r != ' ' && trackTextBG == tcell.ColorDefault {
			trackTextBG = bg
		}
	}
	if trackBarBG == tcell.ColorDefault {
		t.Fatal("no track bar glyph found")
	}
	if trackTextBG == tcell.ColorDefault {
		t.Fatal("no track label text found")
	}
	if trackBarBG != wantTrackBG || trackTextBG != wantTrackBG || trackBarBG != trackTextBG {
		t.Fatalf("track bar bg=%v text bg=%v want track %v", trackBarBG, trackTextBG, wantTrackBG)
	}
}

func TestDedupBarFilledCols(t *testing.T) {
	t.Parallel()
	cases := []struct {
		width int
		frac  float64
		want  int
	}{
		{56, dedupFrac(14, 56), 14},
		{56, dedupFrac(1, 10000), 1}, // started: never fully empty
		{56, dedupFrac(0, 56), 0},    // untouched
		{56, dedupFrac(56, 56), 56},  // complete
		{56, dedupFrac(100, 56), 56}, // clamp overshoot
		{56, dedupFrac(28, 56), 28},  // exact half
		{10, dedupFrac(55, 100), 6},  // rounds, not truncates (5.5 -> 6)
		{56, dedupFrac(1, 0), 0},     // unknown total
	}
	for _, c := range cases {
		if got := dedupBarFilledCols(c.width, c.frac); got != c.want {
			t.Fatalf("dedupBarFilledCols(%d, %v) = %d, want %d", c.width, c.frac, got, c.want)
		}
	}
}

func TestDrawDedupProgressDialogPerFileBar(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	styles := theme.Default()
	layout := Layout{Width: 80, Height: 24}
	snap := comparepkg.DedupSnapshot{
		Root:            pathloc.MustParse("/scan/root"),
		Phase:           comparepkg.DedupHashing,
		Hashed:          1,
		HashTotal:       4,
		HashBytesTotal:  1 << 30,
		HashedBytes:     1 << 29,
		Current:         "media/archive",
		CurrentFile:     "voyage.iso",
		CurrentFileSize: 1 << 29,
		CurrentFileDone: 1 << 28,
	}
	DrawDedupProgressDialog(screen, layout, DedupProgressDialogState{Open: true}, snap, styles, "")

	rect := draw.CenteredDialogRect(layout, PreferredFormDialogWidth, dedupProgressDialogHeight(snap.Phase))
	barY := dedupHashProgressBarRow(screen, rect, snap.Hashed, snap.HashTotal)
	if barY < 0 {
		t.Fatal("hash progress bar row not found")
	}
	textX := draw.DialogTextX(rect)
	textW := draw.DialogContentWidth(rect)

	fileLine := strings.TrimSpace(cellTextAt(screen, textX, barY+1, textW))
	if !strings.Contains(fileLine, "voyage.iso") {
		t.Fatalf("per-file line = %q, want filename label", fileLine)
	}
	if strings.Contains(fileLine, "media/archive") {
		t.Fatalf("per-file line = %q, must not contain the directory path", fileLine)
	}
	_, wantFillBG, _ := styles.DialogProgressFill.Decompose()
	_, gotBG, _ := cellStyleAt(screen, textX, barY+1).Decompose()
	if gotBG != wantFillBG {
		t.Fatalf("per-file first col bg %v, want fill %v", gotBG, wantFillBG)
	}
	lastCh, _, _ := screen.Get(textX+textW-1, barY+1)
	if lastCh != "█" && lastCh != "░" {
		t.Fatalf("per-file last bar col = %q, want █ or ░", lastCh)
	}
}

func dedupHashProgressBarRow(screen tcell.Screen, rect draw.Rect, hashed, total int) int {
	countY, ok := dialogRowContaining(screen, rect, fmt.Sprintf("%d/%d", hashed, total))
	if !ok {
		return -1
	}
	return countY - 2 // per-file bar row sits between the main bar and the count
}

func cellTextAt(screen tcell.Screen, x, y, w int) string {
	var b strings.Builder
	for col := x; col < x+w; col++ {
		ch, _, _ := screen.Get(col, y)
		b.WriteString(ch)
	}
	return b.String()
}
