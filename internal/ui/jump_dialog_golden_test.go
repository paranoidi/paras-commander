package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Render goldens for Jump to directory painted through PaintJumpDialog (the path the app and its
// partial repaint use), so the coloured row devicons and the row marks are covered.

const jumpGoldenDir = "testdata/jump_dialog"

func TestJumpDialogGoldenRender(t *testing.T) {
	root := findGoldenRoot
	results := []dialog.JumpResult{}
	for _, rel := range []string{"amber", "birch", "cinder", "amber/delta", "cinder/ember"} {
		results = append(results, dialog.JumpResult{Path: filepath.Join(root, rel), Rel: rel})
	}
	cases := []struct {
		name            string
		icons, nerdfont bool
	}{
		{"80x24_icons", true, false},
		{"80x24_nerdfont", true, true},
		{"80x24_noicons", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(screen.Fini)
			screen.SetSize(80, 24)
			styles := theme.Default()
			styles.UseNerdfontIcons = c.nerdfont
			model := &Model{
				UseNerdfontIcons: c.icons,
				PanelScrollbar:   uiscrollbar.StyleThumb,
				PinnedItems:      []PinnedItem{{Path: filepath.Join(root, "birch")}},
				JumpDialog: dialog.JumpDialogState{
					Open: true, Root: root, Results: results, Selected: 1, StayOnVolume: true, Scanned: len(results),
				},
			}
			PaintJumpDialog(screen, Layout{Width: 80, Height: 24}, model, styles)
			screen.Show()
			got := tcelltest.DumpRect(screen, 0, 0, 80, 24)
			path := filepath.Join(jumpGoldenDir, c.name+".golden")
			if *updateFindGolden {
				if err := os.MkdirAll(jumpGoldenDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if string(want) != got {
				t.Errorf("%s differs from golden %s\n--- got ---\n%s\n--- want ---\n%s", c.name, path, got, want)
			}
		})
	}
}
