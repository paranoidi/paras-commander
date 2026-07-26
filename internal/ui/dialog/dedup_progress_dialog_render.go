package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func dedupProgressDialogHeight(phase comparepkg.DedupPhase) int {
	// Directory label + blank + path + separator + status row (+ per-file bar row
	// + hash count row when hashing) + blank + buttons + borders.
	if phase == comparepkg.DedupHashing {
		return 12
	}
	return 11
}

// DrawDedupProgressDialog paints the find-duplicates scan progress modal.
func DrawDedupProgressDialog(
	screen tcell.Screen,
	layout Layout,
	state DedupProgressDialogState,
	snap comparepkg.DedupSnapshot,
	styles theme.Theme,
	userHomeDir string,
) {
	if !state.Open {
		return
	}
	width := PreferredFormDialogWidth
	if width < 52 {
		width = 52
	}
	height := dedupProgressDialogHeight(snap.Phase)
	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, "Find Duplicates", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	textX := draw.DialogTextX(rect)
	textW := draw.DialogContentWidth(rect)

	y := rect.Y + 1
	primitive.Text(screen, textX, y, textW, "Directory:", textStyle)
	y += 2
	rootPath := primitive.FitPathForWidth(primitive.PathWithHomeTilde(snap.Root.String(), userHomeDir), textW)
	primitive.Text(screen, textX, y, textW, rootPath, textStyle)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	switch snap.Phase {
	case comparepkg.DedupWalking:
		status := "Walking directories" + string(primitive.Ellipsis) + dedupWalkFilesSuffix(snap.Walked)
		primitive.Text(screen, textX, y, textW, primitive.FitPathForWidth(status, textW), textStyle)
	case comparepkg.DedupAwaitConfirm:
		primitive.Text(screen, textX, y, textW, formatDedupByteSize(snap.HashBytesTotal)+" to hash. Continue?", textStyle)
	case comparepkg.DedupHashing:
		label := snap.Current
		if label == "" {
			label = "Hashing files" + string(primitive.Ellipsis)
		}
		drawDedupBar(screen, textX, y, textW, dedupFrac(snap.HashedBytes, snap.HashBytesTotal), label, styles)
		y++
		// Per-file bar for large files; the row stays blank dialog surface otherwise
		// so the dialog never resizes mid-scan.
		if snap.CurrentFile != "" {
			drawDedupBar(screen, textX, y, textW, dedupFrac(snap.CurrentFileDone, snap.CurrentFileSize), snap.CurrentFile, styles)
		}
		y++
		drawDedupHashCountRow(screen, rect, y, snap.Hashed, snap.HashTotal, textStyle)
	default:
		primitive.Text(screen, textX, y, textW, "Scanning"+string(primitive.Ellipsis), textStyle)
	}

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-1, borderStyle)
	if snap.Phase == comparepkg.DedupAwaitConfirm {
		draw.DrawOKCancelButtonRow(screen, rect, buttonY, state.ButtonFocus == 0, state.ButtonFocus == 1, styles)
		return
	}
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "Cancel", Shortcut: 'C', Focused: state.ButtonFocus == 0},
	}, styles)
}

func dedupWalkFilesSuffix(walked int) string {
	noun := "files"
	if walked == 1 {
		noun = "file"
	}
	return fmt.Sprintf(" %d %s", walked, noun)
}

// dedupFrac returns done/total clamped to [0,1]; zero when total is unknown.
func dedupFrac(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	if done >= total {
		return 1
	}
	if done < 0 {
		return 0
	}
	return float64(done) / float64(total)
}

// drawDedupBar paints a full-width meter whose fill/track are carried by the
// cell background (no block glyphs — a glyph shows its foreground and made the
// bar look patchy), with the label overlaid on top. The label is fitted as a
// path (middle-ellipsized segments) to the bar width.
func drawDedupBar(screen tcell.Screen, x, y, width int, frac float64, label string, styles theme.Theme) {
	if width <= 0 {
		return
	}
	filled := dedupBarFilledCols(width, frac)
	labelRunes := []rune(primitive.FitPathForWidth(label, width))

	for col := 0; col < width; col++ {
		onFill := col < filled
		st := styles.DialogProgressTrack
		if onFill {
			st = styles.DialogProgressFill
		}
		ch := ' '
		if col < len(labelRunes) {
			ch = labelRunes[col]
			if ch != ' ' {
				st = styles.DialogProgressLabelOnBar(onFill)
			}
		}
		screen.SetContent(x+col, y, ch, nil, st)
	}
}

func dedupBarFilledCols(width int, frac float64) int {
	if width <= 0 || frac <= 0 {
		return 0
	}
	filled := int(frac*float64(width) + 0.5)
	if filled == 0 {
		filled = 1 // started: never show a completely empty bar
	}
	if filled > width {
		return width
	}
	return filled
}

func drawDedupHashCountRow(screen tcell.Screen, rect draw.Rect, y, hashed, total int, textStyle tcell.Style) {
	label := fmt.Sprintf("%d/%d", hashed, total)
	innerW := draw.DialogContentWidth(rect)
	x := draw.DialogTextX(rect)
	n := utf8.RuneCountInString(label)
	if n > innerW {
		primitive.Text(screen, x, y, innerW, label, textStyle)
		return
	}
	pad := (innerW - n) / 2
	primitive.Text(screen, x+pad, y, innerW-pad, label, textStyle)
}

func formatDedupByteSize(n int64) string {
	if n < 0 {
		n = 0
	}
	const (
		KiB = int64(1024)
		MiB = KiB * 1024
		GiB = MiB * 1024
		TiB = GiB * 1024
	)
	switch {
	case n < KiB:
		return fmt.Sprintf("%d B", n)
	case n < MiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(KiB))
	case n < GiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(MiB))
	case n < TiB:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(GiB))
	default:
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(TiB))
	}
}
