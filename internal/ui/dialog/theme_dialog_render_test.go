package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestThemeDialogListHeight(t *testing.T) {
	t.Parallel()
	state := ThemeDialogState{Choices: make([]ThemeChoice, 100)}
	rows := func(layout Layout) int {
		list, _ := ThemeDialogView(state, layout, theme.Theme{}).Item(ThemeList)
		return list.Height
	}
	if got := rows(Layout{Width: 80, Height: 24}); got != 17 {
		t.Fatalf("tall dialog viewport = %d, want 17", got)
	}
	if got := rows(Layout{Width: 80, Height: 10}); got != 4 {
		t.Fatalf("clamped dialog viewport = %d, want 4", got)
	}
}
