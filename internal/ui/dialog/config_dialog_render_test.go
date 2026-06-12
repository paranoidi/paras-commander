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
