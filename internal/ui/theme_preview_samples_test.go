package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func themeSampleScreenText(screen tcell.SimulationScreen, w, h int) []string {
	lines := make([]string, h)
	for y := range h {
		var b strings.Builder
		for x := range w {
			ch, _, _ := screen.Get(x, y)
			if ch == "" {
				ch = " "
			}
			b.WriteString(ch)
		}
		lines[y] = strings.TrimRight(b.String(), " ")
	}
	return lines
}

func TestThemeDialogAppPageRender(t *testing.T) {
	const w, h = 130, 60
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(w, h)

	styles := theme.Default()
	state := dialog.ThemeDialogState{
		Open: true, Page: 1, CurrentName: "default", Ranked: []int{0},
		Choices: []dialog.ThemeChoice{{Name: "default", Label: "Default"}},
	}
	dialog.DrawThemeDialog(screen, Layout{Width: w, Height: h}, state, styles, themeAppSamples(Model{UseNerdfontIcons: true}))
	lines := themeSampleScreenText(screen, w, h)
	text := strings.Join(lines, "\n")
	t.Log("\n" + text)

	if len(themeSampleJobStatuses) != dialog.ThemeAppJobRows {
		t.Fatalf("%d sample job statuses, the pane reserves %d rows", len(themeSampleJobStatuses), dialog.ThemeAppJobRows)
	}
	want := []string{"Example 1", "Example 2", "Entry 1", "Entry 2", "Active", "Inactive", "Completed", "100%", " 45%", "2m 10s", "48MB/s", "5m 40s"}
	for _, status := range themeSampleJobStatuses {
		want = append(want, string(status))
	}
	for _, s := range want {
		if !strings.Contains(text, s) {
			t.Errorf("theme dialog on its app page lacks %q", s)
		}
	}

	// The samples carry the previewed theme's own styles.
	_, selBG, _ := styles.MenuDropdownSelected.Decompose()
	found := false
	for y, line := range lines {
		if x := strings.Index(line, "Entry 1"); x >= 0 {
			_, st, _ := screen.Get(len([]rune(line[:x])), y)
			_, bg, _ := st.Decompose()
			found = bg == selBG
		}
	}
	if !found {
		t.Error("Entry 1 is not drawn in menu.dropdown.selected")
	}
	for _, status := range themeSampleJobStatuses {
		wantFG, _, _ := styles.StatusStyle(string(status)).Decompose()
		var gotFG tcell.Color
		for y, line := range lines {
			if x := strings.Index(line, string(status)); x >= 0 && gotFG == 0 && strings.Contains(line, "copy") {
				_, st, _ := screen.Get(len([]rune(line[:x])), y)
				gotFG, _, _ = st.Decompose()
			}
		}
		if gotFG != wantFG {
			t.Errorf("job status %q fg = %v, want %v", status, gotFG, wantFG)
		}
	}
}

func TestThemeDialogPanelPageRender(t *testing.T) {
	const w, h = 130, 60
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(w, h)

	state := dialog.ThemeDialogState{
		Open: true, Page: 0, CurrentName: "default", Ranked: []int{0},
		Choices: []dialog.ThemeChoice{{Name: "default", Label: "Default"}},
	}
	dialog.DrawThemeDialog(screen, Layout{Width: w, Height: h}, state, theme.Default(), themeAppSamples(Model{UseNerdfontIcons: true}))
	text := strings.Join(themeSampleScreenText(screen, w, h), "\n")
	t.Log("\n" + text)
	for _, name := range []string{"harbor", "lantern", "quarry", "current", "stale", "main.go", "notes.md", "photo.png", "archive.zip", "deploy.sh", "cache.log", "secret.key", "todo.txt", "readme.md"} {
		if !strings.Contains(text, name) {
			t.Errorf("theme dialog page 3 lacks %q", name)
		}
	}
	// Short screens clip the sample without panicking.
	for _, height := range []int{12, 16, 22, 30} {
		screen.SetSize(w, height)
		dialog.DrawThemeDialog(screen, Layout{Width: w, Height: height}, state, theme.Default(), themeAppSamples(Model{}))
	}
}
