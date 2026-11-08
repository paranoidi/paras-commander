package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// The listing fact must survive every localfs <-> fsbackend conversion on its way to the panel.
func TestListCarriesBrokenLinkToPanelEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("absent", filepath.Join(dir, "harbor")); err != nil {
		t.Fatal(err)
	}
	loc, err := pathloc.Parse(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := New().List(context.Background(), loc)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	panelEntry := fsbackend.ToPanelEntry(entries[0])
	if !panelEntry.BrokenLink || panelEntry.AccessDenied {
		t.Fatalf("panel entry BrokenLink = %v, AccessDenied = %v, want true, false", panelEntry.BrokenLink, panelEntry.AccessDenied)
	}
	if !fsbackend.FromPanelEntry(panelEntry).BrokenLink {
		t.Fatal("FromPanelEntry dropped BrokenLink")
	}
}
