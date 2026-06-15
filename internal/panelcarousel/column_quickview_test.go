package panelcarousel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestShowChildPreviewColumnOffWhenQuickView(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "birch")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	state, err := panel.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !state.CarouselCenterHasSubdirectories() {
		t.Fatal("fixture should have subdirectories")
	}
	if !ShowChildPreviewColumn(state, false, false) {
		t.Fatal("want child column when quick view off and subdirs present")
	}
	if ShowChildPreviewColumn(state, true, false) {
		t.Fatal("want no child column when quick view on")
	}
	_, _, col, _ := BuildColumns(state, 10, true, false)
	if col.Populated {
		t.Fatal("BuildColumns should not populate child when quick view on")
	}
}
