package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawCompareMergeDialogLeftRightAndShortPaths(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 30)

	home := "/home/alice"
	state := CompareMergeDialogState{
		Open:          true,
		Direction:     comparepkg.MergeTowardSecondary,
		CopyMissing:   true,
		CopyModified:  true,
		PreviewText:   "Preview: 0 to copy (0 B)",
		PrimaryPath:   "/home/alice/projects/paras-commander/test-cases/diff-b",
		SecondaryPath: "/home/alice/projects/paras-commander/test-cases/diff-a",
	}
	DrawCompareMergeDialog(screen, Layout{Width: 80, Height: 30}, state, theme.Default(), home)

	var body strings.Builder
	for y := 0; y < 30; y++ {
		body.WriteString(cellTextAt(screen, 0, y, 80))
		body.WriteByte('\n')
	}
	painted := body.String()
	for _, want := range []string{"Left side", "Right side", "Shared:", "test-cases", "diff-b", "diff-a"} {
		if !strings.Contains(painted, want) {
			t.Fatalf("painted dialog missing %q", want)
		}
	}
	if strings.Contains(painted, "/home/alice/projects/paras-commander/test-cases/diff-b") {
		t.Fatal("dialog should not paint full duplicated absolute primary path")
	}
}

func TestCompareMergeDialogHeightSharedPrefix(t *testing.T) {
	base := compareMergeDialogHeight(false)
	withShared := compareMergeDialogHeight(true)
	if withShared != base+1 {
		t.Fatalf("height with shared = %d, base = %d, want base+1", withShared, base)
	}
}
