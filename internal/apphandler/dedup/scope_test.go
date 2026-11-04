package dedup

import (
	"context"
	"slices"
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestScanScope(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dirs     []string
		wantRoot string
		wantOnly []string
	}{
		{"single dir is the root", []string{"/scan/meadow"}, "/scan/meadow", nil},
		{"nested collapses to outer", []string{"/scan/meadow", "/scan/meadow/lantern"}, "/scan/meadow", nil},
		{"siblings share parent", []string{"/scan/meadow/lantern", "/scan/harbor"}, "/scan", []string{"harbor", "meadow/lantern"}},
		{"name prefix is not an ancestor", []string{"/scan/ab", "/scan/abc"}, "/scan", []string{"ab", "abc"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, only := scanScope(tc.dirs)
			if root.String() != tc.wantRoot || !slices.Equal(only, tc.wantOnly) {
				t.Fatalf("scanScope = %s %v, want %s %v", root, only, tc.wantRoot, tc.wantOnly)
			}
		})
	}
}

func TestOpenWithSelectedDirsAsksForScope(t *testing.T) {
	model := &ui.Model{ViewMode: ui.ViewBrowser}
	model.Primary.Path = pathloc.MustParse("/scan")
	model.Primary.SelectedDirPaths = map[string]bool{"/scan/meadow": true, "/scan/harbor": true}
	h := New(Deps{Host: &dedupHandlerHost{}, Model: model})
	h.Open()
	st := model.DedupReturnDialog
	if !st.Open || st.Dir != "/scan" || !slices.Equal(st.ScopeDirs, []string{"/scan/harbor", "/scan/meadow"}) {
		t.Fatalf("scope dialog = %+v, want open over /scan with both selected dirs", st)
	}
	if h.session != nil {
		t.Fatal("Open must not start a scan before the scope is chosen")
	}
}

// OpenOutside roots an indexed, partial scan at the index root; Refresh keeps that mode and
// any plain Open leaves it.
func TestOpenOutsideModeSurvivesRefreshUntilPlainOpen(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var got comparepkg.DedupOptions
	orig := startDedup
	startDedup = func(ctx context.Context, root pathloc.Path, opts comparepkg.DedupOptions) *comparepkg.DedupSession {
		got = opts
		if opts.Indexed != nil { // no index in this test: the scan finds nothing
			opts.Indexed = func(context.Context) ([]comparepkg.FileRecord, map[string][32]byte, error) { return nil, nil, nil }
		}
		return orig(ctx, root, opts)
	}
	t.Cleanup(func() { startDedup = orig })

	model := &ui.Model{ViewMode: ui.ViewBrowser}
	model.Primary.Path = pathloc.MustParse(t.TempDir())
	h := New(Deps{Host: &dedupHandlerHost{}, Model: model})
	t.Cleanup(h.Close)

	h.OpenOutside("/orchard/pear")
	if got.Indexed == nil || !got.IndexedPartial || model.DedupView.OutsideOf != "/orchard/pear" ||
		model.DedupSnapshot.Root.String() != "fileindex:///" {
		t.Fatalf("OpenOutside: indexed=%v partial=%v outsideOf=%q root=%s", got.Indexed != nil, got.IndexedPartial, model.DedupView.OutsideOf, model.DedupSnapshot.Root)
	}
	got = comparepkg.DedupOptions{}
	model.ViewMode = ui.ViewDedup
	h.Refresh()
	if got.Indexed == nil || !got.IndexedPartial || model.DedupView.OutsideOf != "/orchard/pear" {
		t.Fatalf("Refresh lost the mode: indexed=%v partial=%v outsideOf=%q", got.Indexed != nil, got.IndexedPartial, model.DedupView.OutsideOf)
	}
	got = comparepkg.DedupOptions{}
	h.Open()
	if got.Indexed != nil || got.IndexedPartial || model.DedupView.OutsideOf != "" {
		t.Fatalf("plain Open kept the mode: indexed=%v partial=%v outsideOf=%q", got.Indexed != nil, got.IndexedPartial, model.DedupView.OutsideOf)
	}
}
