package dedup

import (
	"slices"
	"testing"

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
