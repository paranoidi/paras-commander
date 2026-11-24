package dialog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

type locationHost struct {
	Host
	panel     *panel.State
	dest      *panel.State
	cfg       config.Config
	navigated []string
}

func (f *locationHost) NavigatePanelToPath(_ int, path, _ string) error {
	f.navigated = append(f.navigated, path)
	return nil
}
func (f *locationHost) PanelByID(int) *panel.State                    { return f.panel }
func (f *locationHost) PanelViewportRows(int) int                     { return 10 }
func (f *locationHost) DestinationPanel() *panel.State                { return f.dest }
func (f *locationHost) Config() config.Config                         { return f.cfg }
func (f *locationHost) SetTransientMessage(string, ui.MessageUrgency) {}

// TestBookmarkOfIndexLocationOpensAsWritten: a bookmark of a fileindex path (plain, tag view,
// flat tag view) reaches the picker and navigation as written; path/filepath would have turned
// "fileindex:///a" into "fileindex:/a".
func TestBookmarkOfIndexLocationOpensAsWritten(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		"fileindex:///meadow/orchard",
		"fileindex(clover&willow):///meadow/orchard",
		"fileindex@(clover|willow):///meadow//orchard",
	} {
		dir := t.TempDir()
		marksPath := filepath.Join(dir, "marks")
		if err := os.WriteFile(marksPath, []byte("lantern : "+want+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := config.Default()
		cfg.Bookmarks.File = marksPath
		host := &locationHost{panel: &panel.State{}, cfg: cfg}
		h := &Handler{host: host, model: &ui.Model{UserHomeDir: dir}, getenv: func(string) string { return "" }}

		items, err := h.PathPickerItemsBookmarks()
		if err != nil || len(items) != 1 || items[0].Path != want {
			t.Fatalf("%s: items = %+v, err %v; want the path as written", want, items, err)
		}
		h.model.PathPicker = uidialog.PathPickerState{
			Open: true, Purpose: uidialog.PathPickerPurposeNavigate, Items: items, Ranked: []int{0},
		}
		if !h.openSelectedBookmarkInPanel(ui.PrimaryPanel) || len(host.navigated) != 1 || host.navigated[0] != want {
			t.Fatalf("%s: navigated to %v", want, host.navigated)
		}
	}
}

// TestLinkDefaultPathFollowsDestinationHostDirectory: the default path of a symlink or hardlink
// is in the destination panel's real directory; a destination with none falls back to the source's.
func TestLinkDefaultPathFollowsDestinationHostDirectory(t *testing.T) {
	t.Parallel()
	src := &panel.State{Path: pathloc.FileMust("/meadow")}
	for _, tc := range []struct{ dest, want string }{
		{"/orchard", "/orchard/lantern"},
		{"fileindex:///orchard/willow", "/orchard/willow/lantern"},
		{"fileindex(clover):///orchard", "/orchard/lantern"},
		{"fileindex@(clover):///", "/meadow/lantern"},
		{"sftp://harbor/orchard", "/meadow/lantern"},
	} {
		h := &Handler{host: &locationHost{dest: &panel.State{Path: pathloc.MustParse(tc.dest)}}}
		if got := h.linkDefaultPath(src, "lantern"); got != tc.want {
			t.Errorf("destination %s: default link path = %q, want %q", tc.dest, got, tc.want)
		}
	}
}
