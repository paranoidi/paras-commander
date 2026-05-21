package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

func TestDrawUserMenuDialogSingleEntry(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	layout := Layout{Width: 80, Height: 22}
	state := UserMenuDialogState{
		Open:  true,
		Title: "User menu",
		Entries: []usermenu.MenuEntry{{
			Key:     "g",
			Title:   "lazygit",
			Command: "lazygit",
		}},
		Selected: 0,
		Focus:    0,
	}

	DrawUserMenuDialog(screen, layout, state, theme.Default())

	var hasEntry, hasCancel bool
	for y := 0; y < 24; y++ {
		var line strings.Builder
		for x := 0; x < 80; x++ {
			str, _, _ := screen.Get(x, y)
			line.WriteString(str)
		}
		s := line.String()
		if strings.Contains(s, "lazygit") {
			hasEntry = true
		}
		if strings.Contains(s, "Cancel") {
			hasCancel = true
		}
		if strings.Contains(s, " OK ") || strings.HasSuffix(strings.TrimSpace(s), " OK") {
			t.Fatalf("user menu should not show OK button: %q", s)
		}
	}
	if !hasEntry || !hasCancel {
		t.Fatalf("user menu dialog not drawn for a single entry (entry=%v cancel=%v)", hasEntry, hasCancel)
	}
}
