package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func TestDrawCompareMergeDialogShowsFullPaths(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 30)

	state := CompareMergeDialogState{
		Open:          true,
		Direction:     comparepkg.MergeTowardSecondary,
		CopyMissing:   true,
		CopyModified:  true,
		PreviewText:   "Preview: 0 to copy (0 B)",
		PrimaryPath:   "/home/alice/harbor/orchard/meadow",
		SecondaryPath: "/home/alice/harbor/orchard/willow",
	}
	DrawCompareMergeDialog(screen, Layout{Width: 80, Height: 30}, state, theme.Default(), "/home/alice")

	var body strings.Builder
	for y := 0; y < 30; y++ {
		body.WriteString(cellTextAt(screen, 0, y, 80))
		body.WriteByte('\n')
	}
	painted := body.String()
	for _, want := range []string{"Left side", "Right side", "~/harbor/orchard/meadow", "~/harbor/orchard/willow"} {
		if !strings.Contains(painted, want) {
			t.Fatalf("painted dialog missing %q", want)
		}
	}
	if strings.Contains(painted, "Shared:") {
		t.Fatal("dialog should not paint a shared path row")
	}
}

func TestCompareMergeDialogHeightIgnoresSharedParent(t *testing.T) {
	height := func(primary, secondary string) int {
		p, ok := dlg.Place(80, 40, CompareMergeView(CompareMergeDialogState{PrimaryPath: primary, SecondaryPath: secondary}, "/home/alice"))
		if !ok {
			t.Fatal("Place not ok")
		}
		return p.Rect.Height
	}
	unrelated := height("/srv/orchard", "/mnt/willow")
	sharedParent := height("/srv/harbor/orchard", "/srv/harbor/willow")
	if sharedParent != unrelated {
		t.Fatalf("height with shared parent = %d, unrelated = %d, want equal", sharedParent, unrelated)
	}
}
