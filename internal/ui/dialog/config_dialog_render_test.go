package dialog

import (
	"testing"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func TestConfigDialogScrollbarColumnsCenteredLabel(t *testing.T) {
	t.Parallel()
	rect := draw.Rect{X: 10, Y: 5, Width: 54, Height: 21}
	labelCol, optionCol := configDialogScrollbarColumns(rect)
	labelW := utf8.RuneCountInString(configDialogScrollbarStyleLabel)
	contentWidth := draw.DialogContentWidth(rect)
	centerCol := draw.DialogTextX(rect) + contentWidth/2
	wantLabelCol := centerCol - labelW/2
	if labelCol != wantLabelCol {
		t.Fatalf("labelCol = %d, want %d", labelCol, wantLabelCol)
	}
	if optionCol != labelCol-1 {
		t.Fatalf("optionCol = %d, want %d", optionCol, labelCol-1)
	}
}

func TestConfigDialogSplitColumns(t *testing.T) {
	t.Parallel()
	rect := draw.Rect{X: 10, Y: 5, Width: 54, Height: 22}
	cols, odd := configDialogSplitColumns(rect)
	if odd {
		t.Fatal("width 54 should be even")
	}
	if cols[0] != draw.DialogTextX(rect) {
		t.Fatalf("first col = %d, want %d", cols[0], draw.DialogTextX(rect))
	}
	if end := cols[2] + configDialogSplitInputW; end != draw.DialogTextX(rect)+draw.DialogContentWidth(rect) {
		t.Fatalf("last input ends at %d, want right margin", end)
	}
	if cols[1]-cols[0] != cols[2]-cols[1] {
		t.Fatalf("uneven gaps: %v", cols)
	}
	rect.Width = 53
	if _, odd := configDialogSplitColumns(rect); !odd {
		t.Fatal("width 53 should report odd remainder")
	}
}
