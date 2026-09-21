package dialog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

// identityTestHost is a Host stub for snapshot tests of delete/rename/chown dialogs.
type identityTestHost struct {
	model    *ui.Model
	cfg      config.Config
	errors   []string
	messages []string
}

func (f *identityTestHost) LayoutForTerminalSize(w, h int) ui.Layout {
	return ui.Layout{Width: w, Height: h}
}
func (f *identityTestHost) LayoutForTerminalSizePreview(w, h int, _ bool) ui.Layout {
	return ui.Layout{Width: w, Height: h}
}
func (f *identityTestHost) SetTransientMessage(text string, _ ui.MessageUrgency) {
	f.messages = append(f.messages, text)
}
func (f *identityTestHost) SetErrorMessage(title string, err error) {
	if err != nil {
		f.errors = append(f.errors, title+": "+err.Error())
	} else {
		f.errors = append(f.errors, title)
	}
}
func (f *identityTestHost) NavigatePanelToPath(int, string, string) error { return nil }
func (f *identityTestHost) ActivePanel() *panel.State                     { return &f.model.Primary }
func (f *identityTestHost) InactivePanel() *panel.State                   { return &f.model.Secondary }
func (f *identityTestHost) InactivePanelID() int                          { return ui.SecondaryPanel }
func (f *identityTestHost) PanelByID(panelID int) *panel.State {
	if panelID == ui.SecondaryPanel {
		return &f.model.Secondary
	}
	return &f.model.Primary
}
func (f *identityTestHost) ActiveViewportRows() int                     { return 20 }
func (f *identityTestHost) PanelViewportRows(int) int                   { return 20 }
func (f *identityTestHost) SelectionsStripViewportRows(int) int         { return 0 }
func (f *identityTestHost) PathVolumeContendsWithActiveJob(string) bool { return false }
func (f *identityTestHost) FilterJobContendedPaths(p []string) []string { return p }
func (f *identityTestHost) ClearTransientMessage()                      {}
func (f *identityTestHost) Config() config.Config                       { return f.cfg }
func (f *identityTestHost) Styles() theme.Theme                         { return theme.Default() }
func (f *identityTestHost) OpenMessageDialog(string, string)            {}
func (f *identityTestHost) InQuickFilterUI() bool                       { return false }
func (f *identityTestHost) OpenFileInExternalEditor(string) error       { return nil }
func (f *identityTestHost) ExecuteSFTPPassword()                        {}
func (f *identityTestHost) CancelSFTPPassword()                         {}
func (f *identityTestHost) HandlePathPickerScrollingQueryKey(*tcell.EventKey) bool {
	return false
}
func (f *identityTestHost) SyncFilteredListRanks(lines []string, _ string, _ int, _ bool) ([]int, [][]search.Range) {
	ranked := make([]int, len(lines))
	for i := range lines {
		ranked[i] = i
	}
	return ranked, make([][]search.Range, len(lines))
}
func (f *identityTestHost) ClampFilteredListSelection(selected *int, rankedLen int) {
	if rankedLen == 0 {
		*selected = 0
		return
	}
	if *selected < 0 {
		*selected = 0
	}
	if *selected >= rankedLen {
		*selected = rankedLen - 1
	}
}
func (f *identityTestHost) HandleFilteredListSelectionKey(*tcell.EventKey, int, *int, int, func() int, func()) bool {
	return false
}
func (f *identityTestHost) ActivePanelSources() []string { return nil }
func (f *identityTestHost) PrimaryPanel() *panel.State   { return &f.model.Primary }
func (f *identityTestHost) SecondaryPanel() *panel.State { return &f.model.Secondary }
func (f *identityTestHost) HandleQuit() bool             { return false }
func (f *identityTestHost) HandleQuitImmediate() bool    { return false }
func (f *identityTestHost) OpenMenu()                    {}
func (f *identityTestHost) OpenMenuByShortcut(rune) bool { return false }
func (f *identityTestHost) Dispatch(string)              {}
func (f *identityTestHost) TryDispatchAuxiliaryScreens(string) bool {
	return false
}
func (f *identityTestHost) ActionFromKeyEvent(*tcell.EventKey) string   { return "" }
func (f *identityTestHost) ActionForPreviewMenuKey(rune) (string, bool) { return "", false }
func (f *identityTestHost) ToggleLeaderMenu()                           {}
func (f *identityTestHost) DispatchLeaderLetter(*tcell.EventKey) bool   { return false }
func (f *identityTestHost) SetUnsupportedMessage(string)                {}
func (f *identityTestHost) RefreshBothPanels()                          {}
func (f *identityTestHost) RequestBothPanelsVolumeSpaceRefreshAsync()   {}
func (f *identityTestHost) OpenTransferDialogSelfCopyRename(uidialog.TransferKind, string, string) {
}
func (f *identityTestHost) SetJobFailedTransientMessage(error, string) {}
func (f *identityTestHost) DevMode() bool                              { return false }
func (f *identityTestHost) PromptDanglingDirDelete([]string)           {}
func (f *identityTestHost) LaunchedAsFileViewer() bool                 { return false }
func (f *identityTestHost) SwitchPanel()                               {}
func (f *identityTestHost) SyncFollowTargetPath(*panel.State) (string, bool) {
	return "", false
}
func (f *identityTestHost) PanelSyncFollowHeldListNav(string, *tcell.EventKey) bool {
	return false
}
func (f *identityTestHost) ArmPanelSyncFollowNavCoalesceAfterListNav() {}
func (f *identityTestHost) ClearPanelSyncFollowNavCoalesce()           {}
func (f *identityTestHost) ArmCursorNameHintNavCoalesceAfterListNav()  {}
func (f *identityTestHost) GitStatusScheduler(int) panel.GitStatusScheduler {
	return nil
}
func (f *identityTestHost) AsyncLoadScheduler(int) panel.AsyncLoadScheduler {
	return nil
}
func (f *identityTestHost) ScheduleCarouselParentSnapshot(int, int) {}
func (f *identityTestHost) ScheduleCarouselChildSnapshot(int, int)  {}
func (f *identityTestHost) PeekGitStatus(string, string, []gitstatus.ListingPaths) (map[string]gitstatus.Cell, bool) {
	return nil, false
}
func (f *identityTestHost) EffectivePaneSplitOrientation() ui.SplitOrientation {
	return ui.SplitHorizontal
}
func (f *identityTestHost) PanelPaneSplit(int, bool) ui.PanelPaneSplit {
	return ui.PanelPaneSplit{ActivePercent: 50, InactivePercent: 50}
}
func (f *identityTestHost) TerminalLayoutRows() int                   { return 24 }
func (f *identityTestHost) BrowserMenuDefinitions() []menu.Definition { return nil }
func (f *identityTestHost) CarouselAutohideInactivePanel() bool       { return false }
func (f *identityTestHost) EditActiveFile()                           {}
func (f *identityTestHost) EditFullscreenPreviewFile()                {}
func (f *identityTestHost) OpenDeleteDialogForPreviewedFile()         {}
func (f *identityTestHost) OpenPreviewLeaderMenu()                    {}
func (f *identityTestHost) OpenPreviewCopyMenu()                      {}
func (f *identityTestHost) FilePreviewFullscreenClosed()              {}
func (f *identityTestHost) HandleFileDialogFieldKey(*tcell.EventKey, *uidialog.FileDialogField, func()) bool {
	return false
}
func (f *identityTestHost) PersistPartial(map[string]interface{}) error { return nil }
func (f *identityTestHost) SetPreviewStyle(string)                      {}
func (f *identityTestHost) ApplyPreviewStyle(string) bool               { return true }

type snapshotHarness struct {
	h        *Handler
	host     *identityTestHost
	jobState *jobs.State
	dir      string
	target   string
	neighbor string
}

func newSnapshotHarness(t *testing.T, confirmDelete bool) *snapshotHarness {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "cedar.txt")
	neighbor := filepath.Join(dir, "meadow.txt")
	if err := os.WriteFile(target, []byte("cedar"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(neighbor, []byte("meadow"), 0o644); err != nil {
		t.Fatal(err)
	}

	model := &ui.Model{}
	if err := model.Primary.Load(dir); err != nil {
		t.Fatalf("Load primary: %v", err)
	}
	if err := model.Secondary.Load(dir); err != nil {
		t.Fatalf("Load secondary: %v", err)
	}
	if !model.Primary.SelectVisibleEntry("cedar.txt") {
		t.Fatal("cedar.txt not visible")
	}

	cfg := config.Default()
	cfg.Operations.ConfirmDelete = confirmDelete
	cfg.Preview.Prefetch = false
	fh := &identityTestHost{model: model, cfg: cfg}
	screen := uitest.Screen(t, 80, 24)
	jobState := jobs.NewState()
	jobsH := jobsctrl.New(jobsctrl.Deps{
		Host:   fh,
		Screen: screen,
		Model:  model,
		State:  jobState,
		Config: cfg,
	})
	previewH := previewctrl.New(previewctrl.Deps{
		Host:   fh,
		Screen: screen,
		Model:  model,
		Ctx:    context.Background(),
	})
	h := New(Deps{
		Host:    fh,
		Screen:  screen,
		Model:   model,
		Jobs:    jobsH,
		Preview: previewH,
	})
	return &snapshotHarness{
		h: h, host: fh, jobState: jobState,
		dir: dir, target: target, neighbor: neighbor,
	}
}

func (s *snapshotHarness) applyListingWithoutTarget(t *testing.T) {
	t.Helper()
	p := &s.h.model.Primary
	backend := panel.BackendEntriesFromPanel(p.Entries)
	filtered := backend[:0]
	for _, e := range backend {
		if e.Loc.String() == s.target {
			continue
		}
		filtered = append(filtered, e)
	}
	if err := p.ApplyListing(p.Path, filtered, "meadow.txt", 20, 0, false); err != nil {
		t.Fatalf("ApplyListing: %v", err)
	}
	cur, ok := p.CurrentEntry()
	if !ok || cur.Path == s.target {
		t.Fatalf("after listing change cursor = %+v, want neighbor", cur)
	}
}

func TestDeleteUsesOpenSnapshotForBothConfirmDeleteValues(t *testing.T) {
	t.Parallel()
	for _, confirm := range []bool{true, false} {
		t.Run(confirmDeleteName(confirm), func(t *testing.T) {
			t.Parallel()
			s := newSnapshotHarness(t, confirm)
			s.h.OpenDeleteDialog(&s.h.model.Primary)
			if confirm {
				if !s.h.model.FileDialog.Open {
					t.Fatal("confirm_delete=true should open the dialog")
				}
				s.applyListingWithoutTarget(t)
				s.h.ExecuteDelete()
			} else if s.h.model.FileDialog.Open {
				t.Fatal("confirm_delete=false should skip the dialog")
			}
			assertQueuedDeletePath(t, s.jobState, s.target, s.neighbor)
			if _, err := os.Stat(s.neighbor); err != nil {
				t.Fatalf("neighbor was touched: %v", err)
			}
		})
	}
}

func TestRenameUsesOpenSnapshotAfterListingDropsCursor(t *testing.T) {
	t.Parallel()
	s := newSnapshotHarness(t, true)
	s.h.OpenRenameDialog(&s.h.model.Primary)
	if !s.h.model.FileDialog.Open || s.h.model.FileDialog.DialogType != uidialog.FileDialogRename {
		t.Fatalf("rename dialog = %+v", s.h.model.FileDialog)
	}
	s.h.model.FileDialog.Fields[0].Value = "pine.txt"
	s.h.model.FileDialog.Fields[0].PrefillPending = false
	s.applyListingWithoutTarget(t)
	s.h.executeRename()
	if _, err := os.Stat(s.target); !os.IsNotExist(err) {
		t.Fatalf("original path still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "pine.txt")); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if _, err := os.Stat(s.neighbor); err != nil {
		t.Fatalf("neighbor was touched: %v", err)
	}
}

func confirmDeleteName(confirm bool) string {
	if confirm {
		return "confirm_delete=true"
	}
	return "confirm_delete=false"
}

func assertQueuedDeletePath(t *testing.T, state *jobs.State, want, notWant string) {
	t.Helper()
	all := state.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs = %d, want 1", len(all))
	}
	if all[0].Type != jobs.TypeDelete {
		t.Fatalf("job type = %s, want delete", all[0].Type)
	}
	got := make([]string, len(all[0].Sources))
	for i, src := range all[0].Sources {
		got[i] = src.String()
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("delete sources = %v, want [%q]", got, want)
	}
	for _, p := range got {
		if p == notWant {
			t.Fatalf("delete sources included neighbor %q", notWant)
		}
	}
}
