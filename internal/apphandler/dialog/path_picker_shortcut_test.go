package dialog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

type pathPickerItemsHost struct {
	Host
	active   *panel.State
	inactive *panel.State
	cfg      config.Config
}

func (f pathPickerItemsHost) ActivePanel() *panel.State   { return f.active }
func (f pathPickerItemsHost) InactivePanel() *panel.State { return f.inactive }
func (f pathPickerItemsHost) Config() config.Config       { return f.cfg }

func TestPathPickerBuildersLeaveShortcutIDsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	harbor := filepath.Join(dir, "harbor")
	if err := os.Mkdir(harbor, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(dir, "marks")
	if err := os.WriteFile(marksPath, []byte("anchor : "+harbor+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	active := &panel.State{Path: pathloc.FileMust(dir), History: []string{dir}}
	inactive := &panel.State{Path: pathloc.FileMust(dir)}
	h := &Handler{
		host: pathPickerItemsHost{active: active, inactive: inactive, cfg: cfg},
		model: &ui.Model{
			UserHomeDir: dir,
			PinnedItems: []ui.PinnedItem{{Path: harbor, IsDir: true}},
		},
	}

	history, err := h.PathPickerItemsHistory()
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	bookmarks, err := h.PathPickerItemsBookmarks()
	if err != nil {
		t.Fatalf("bookmarks: %v", err)
	}
	pinned, err := h.PathPickerItemsPinned()
	if err != nil {
		t.Fatalf("pinned: %v", err)
	}
	items := append(append(history, bookmarks...), pinned...)
	if len(items) == 0 {
		t.Fatal("want at least one path-picker item from history/bookmarks/pinned")
	}
	for _, item := range items {
		if item.ID != "" {
			t.Fatalf("production path-picker item has ID %q; shortcut IDs are not assigned", item.ID)
		}
		if got, want := item.SearchLine(), searchLineWithoutID(item); got != want {
			t.Fatalf("SearchLine() = %q, want %q (Name/Path only)", got, want)
		}
	}
}

func searchLineWithoutID(item uidialog.PathPickerItem) string {
	parts := make([]string, 0, 2)
	if item.Name != "" {
		parts = append(parts, item.Name)
	}
	if item.Path != "" {
		parts = append(parts, item.Path)
	}
	return strings.Join(parts, " ")
}
