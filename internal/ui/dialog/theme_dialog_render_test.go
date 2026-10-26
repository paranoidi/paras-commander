package dialog

import "testing"

func TestThemeDialogListHeight(t *testing.T) {
	t.Parallel()
	state := ThemeDialogState{Choices: make([]ThemeChoice, 100)}
	rows := func(layout Layout) int {
		list, _ := ThemeDialogView(state, layout).Item(ThemeList)
		return list.Height
	}
	if got := rows(Layout{Width: 80, Height: 24}); got != 18 {
		t.Fatalf("tall dialog viewport = %d, want 18", got)
	}
	if got := rows(Layout{Width: 80, Height: 10}); got != 4 {
		t.Fatalf("clamped dialog viewport = %d, want 4", got)
	}
}
