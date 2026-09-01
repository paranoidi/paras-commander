package find

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fswalk"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/scan"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// fakeFindHost is a minimal Host stub — only the methods findDialogSelectParentDirs
// actually exercises (PanelByID, SetTransientMessage) do real work.
type fakeFindHost struct {
	p            *panel.State
	transientMsg string
	transientUrg ui.MessageUrgency
}

func (f *fakeFindHost) LayoutForTerminalSize(w, h int) ui.Layout { return ui.Layout{} }
func (f *fakeFindHost) SetTransientMessage(text string, urgency ui.MessageUrgency) {
	f.transientMsg = text
	f.transientUrg = urgency
}
func (f *fakeFindHost) SetErrorMessage(title string, err error) {}
func (f *fakeFindHost) PanelByID(panelID int) *panel.State      { return f.p }
func (f *fakeFindHost) ActivePanel() *panel.State               { return f.p }
func (f *fakeFindHost) ActiveViewportRows() int                 { return 0 }
func (f *fakeFindHost) InQuickFilterUI() bool                   { return false }
func (f *fakeFindHost) NavigatePanelToPath(int, string, string) error {
	return nil
}
func (f *fakeFindHost) HandleScrollingQueryKey(*tcell.EventKey, bool, ScrollingQueryEdit) bool {
	return false
}
func (f *fakeFindHost) FindDialogScrollingQuery(*dialog.FindDialogState, int, func()) ScrollingQueryEdit {
	return ScrollingQueryEdit{}
}
func (f *fakeFindHost) FindDialogQueryWidth() int                     { return 0 }
func (f *fakeFindHost) DiskUsageIgnore() diskusage.ShouldIgnoreFolder { return nil }
func (f *fakeFindHost) GitignoreCache() *gitignore.Cache              { return nil }
func (f *fakeFindHost) PanelViewportRows(int) int                     { return 0 }
func (f *fakeFindHost) OpenGroupSelectDialog(GroupSelectMode, bool)   {}
func (f *fakeFindHost) OpenFullscreenFilePreviewAt(string) error      { return nil }
func (f *fakeFindHost) PinTogglePath(string, string, bool)            {}

func newTestFindHandler(host *fakeFindHost, model *ui.Model) *Handler {
	return &Handler{
		host:  host,
		model: model,
		scan:  scan.NewCoordinator(fswalk.Params{}, func(scan.Event) {}),
	}
}

func TestFindDialogMarkedPaths(t *testing.T) {
	t.Parallel()
	st := &dialog.FindDialogState{
		MarkedPaths: map[string]bool{
			"/root/a/one.txt": true,
			"/root/b/two.txt": true,
			"/root/c/off.txt": false,
		},
	}
	got := findDialogMarkedPaths(st)
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 marked paths", got)
	}
}

func TestFindDialogCursorPath(t *testing.T) {
	t.Parallel()
	st := &dialog.FindDialogState{
		RootPath: "/root",
		Entries:  []dialog.FindEntry{{RelLine: "a/one.txt"}},
		Ranked:   []int{0},
		Selected: 0,
	}
	got, ok := findDialogCursorPath(st)
	if !ok || got != "/root/a/one.txt" {
		t.Fatalf("got (%q, %v), want (/root/a/one.txt, true)", got, ok)
	}

	st.Selected = -1
	if _, ok := findDialogCursorPath(st); ok {
		t.Fatal("out-of-range Selected should report false")
	}
}

func TestFindDialogSelectParentDirsMarked(t *testing.T) {
	t.Parallel()
	p := &panel.State{}
	host := &fakeFindHost{p: p}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			PanelID: ui.PrimaryPanel,
			MarkedPaths: map[string]bool{
				"/root/dirA/one.txt": true,
				"/root/dirA/two.txt": true,
				"/root/dirB/three":   true,
			},
		},
	}
	h := newTestFindHandler(host, model)
	h.findDialogSelectParentDirs()

	want := map[string]bool{"/root/dirA": true, "/root/dirB": true}
	if len(p.SelectedPaths) != len(want) {
		t.Fatalf("selected paths = %v, want %v", p.SelectedPaths, want)
	}
	for k := range want {
		if !p.SelectedPaths[k] {
			t.Fatalf("missing selected parent dir %q, got %v", k, p.SelectedPaths)
		}
	}
	if host.transientMsg == "" {
		t.Fatal("expected a transient message")
	}
}

func TestFindDialogSelectParentDirsCursorFallback(t *testing.T) {
	t.Parallel()
	p := &panel.State{}
	host := &fakeFindHost{p: p}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			PanelID:  ui.PrimaryPanel,
			RootPath: "/root",
			Entries:  []dialog.FindEntry{{RelLine: "dirA/one.txt"}},
			Ranked:   []int{0},
			Selected: 0,
		},
	}
	h := newTestFindHandler(host, model)
	h.findDialogSelectParentDirs()

	if !p.SelectedPaths["/root/dirA"] {
		t.Fatalf("expected cursor entry's parent dir selected, got %v", p.SelectedPaths)
	}
}

func TestFindDialogSelectParentDirsNoSelection(t *testing.T) {
	t.Parallel()
	p := &panel.State{}
	host := &fakeFindHost{p: p}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{PanelID: ui.PrimaryPanel, Selected: -1},
	}
	h := newTestFindHandler(host, model)
	h.findDialogSelectParentDirs()

	if len(p.SelectedPaths) != 0 {
		t.Fatalf("expected no selection change, got %v", p.SelectedPaths)
	}
	if host.transientMsg != "No selection" {
		t.Fatalf("got message %q, want %q", host.transientMsg, "No selection")
	}
}
