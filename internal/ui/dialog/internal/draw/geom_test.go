package draw

import "testing"

func TestDialogColumnOffsets(t *testing.T) {
	rect := Rect{X: 10, Y: 3, Width: 50, Height: 8}
	textX := DialogTextX(rect)
	optionX := DialogOptionX(rect)
	if textX != rect.X+2 {
		t.Fatalf("DialogTextX = %d, want %d", textX, rect.X+2)
	}
	if optionX != rect.X+1 {
		t.Fatalf("DialogOptionX = %d, want %d", optionX, rect.X+1)
	}
	if DialogContentWidth(rect) != rect.Width-4 {
		t.Fatalf("DialogContentWidth = %d, want %d", DialogContentWidth(rect), rect.Width-4)
	}
	// Radio/checkbox markers lead with a space; '(' / '[' must align with plain text column.
	if optionX+1 != textX {
		t.Fatalf("option marker column %d+1 != text column %d", optionX, textX)
	}
}
