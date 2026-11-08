package meta

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/columnprofile"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func columnsHandler(t *testing.T) (*Handler, *fakeHost) {
	t.Helper()
	fh := &fakeHost{panels: [2]*panel.State{testPanel(t, t.TempDir(), nil)}}
	h := &Handler{host: fh, model: &ui.Model{}, config: config.Default()}
	return h, fh
}

func TestCycleProfile_walksBuiltinsThenSavedAndWraps(t *testing.T) {
	h, fh := columnsHandler(t)
	h.profiles = []columnprofile.Profile{{Name: "orchard", Columns: []string{"owner"}}}
	p := fh.panels[0]
	p.ColumnProfile = config.ColumnProfileModified
	for _, want := range []string{config.ColumnProfilePermissions, config.ColumnProfileBrief, "orchard", config.ColumnProfileModified} {
		h.CycleProfile(0)
		if p.ColumnProfile != want {
			t.Fatalf("profile = %q, want %q", p.ColumnProfile, want)
		}
	}
}

func TestCycleProfile_customColumnsGoToFirst(t *testing.T) {
	h, fh := columnsHandler(t)
	fh.panels[0].ColumnProfile = ""
	h.CycleProfile(0)
	if fh.panels[0].ColumnProfile != config.ColumnProfileModified {
		t.Fatalf("profile = %q", fh.panels[0].ColumnProfile)
	}
}

func TestApplyProfile_switchesMetaOffAndOn(t *testing.T) {
	h, fh := columnsHandler(t)
	h.ApplyProfile(0, columnprofile.Profile{Name: "orchard", Columns: []string{"size", "owner"}, Meta: []string{"meadow"}})
	p := fh.panels[0]
	if p.Columns != panel.ColumnsBrief.With(panel.ColumnOwner) || !slices.Equal(h.activeEntries[0], []string{"meadow"}) {
		t.Fatalf("columns = %v, meta = %v", p.Columns.Names(), h.activeEntries[0])
	}
	h.ApplyProfile(0, columnprofile.Profile{Name: config.ColumnProfileBrief, Columns: []string{"size"}})
	if len(h.activeEntries[0]) != 0 || h.model.MetaResults[0] != nil {
		t.Fatalf("meta = %v after a profile without meta", h.activeEntries[0])
	}
}

func TestApplyProfile_sortOnRemovedColumnFallsBackToName(t *testing.T) {
	h, fh := columnsHandler(t)
	h.ApplyProfile(0, columnprofile.Profile{Name: "orchard", Meta: []string{"meadow", "lantern"}})
	p := fh.panels[0]
	p.Sort = panel.SortState{Mode: panel.SortMeta, MetaColumn: "meadow"}
	h.ApplyProfile(0, columnprofile.Profile{Name: "harbor", Meta: []string{"lantern"}})
	if p.Sort.Mode != panel.SortName || p.Sort.MetaColumn != "" {
		t.Fatalf("Sort = %+v, want the Name fallback", p.Sort)
	}
}

func TestApplyProfile_createdColumnReloadsOnlyWhenTurnedOn(t *testing.T) {
	h, fh := columnsHandler(t)
	created := columnprofile.Profile{Name: "orchard", Columns: []string{"created"}}
	h.ApplyProfile(0, created)
	h.ApplyProfile(0, created)
	if !slices.Equal(fh.reloads, []int{0}) {
		t.Fatalf("reloads = %v, want one", fh.reloads)
	}
}

func TestIndexProfile_restoresColumnsOnLeaving(t *testing.T) {
	h, fh := columnsHandler(t)
	p := fh.panels[0]
	p.Columns, p.ColumnProfile = panel.ColumnsModified, config.ColumnProfileModified
	p.WithIndex = true
	h.HandlePanelDirChanged(0)
	if p.ColumnProfile != config.ColumnProfileIndex || p.Columns != panel.ColumnsBrief || !slices.Equal(h.activeEntries[0], []string{config.FileIndexTagsColumn}) {
		t.Fatalf("in index: profile %q columns %v meta %v", p.ColumnProfile, p.Columns.Names(), h.activeEntries[0])
	}
	p.WithIndex = false
	h.HandlePanelDirChanged(0)
	if p.ColumnProfile != config.ColumnProfileModified || p.Columns != panel.ColumnsModified || len(h.activeEntries[0]) != 0 {
		t.Fatalf("after: profile %q columns %v meta %v", p.ColumnProfile, p.Columns.Names(), h.activeEntries[0])
	}
}

func TestIndexProfile_userChangeInsideSticks(t *testing.T) {
	h, fh := columnsHandler(t)
	p := fh.panels[0]
	p.Columns = panel.ColumnsModified
	p.WithIndex = true
	h.HandlePanelDirChanged(0)
	h.ApplyProfile(0, columnprofile.Profile{Name: "orchard", Columns: []string{"group"}, Meta: []string{"meadow"}})
	p.WithIndex = false
	h.HandlePanelDirChanged(0)
	if p.ColumnProfile != "orchard" || !slices.Equal(h.activeEntries[0], []string{"meadow"}) {
		t.Fatalf("profile %q meta %v, want the user's columns kept", p.ColumnProfile, h.activeEntries[0])
	}
}

func TestIndexProfile_emptyNameDisables(t *testing.T) {
	h, fh := columnsHandler(t)
	h.config.FileIndex.DefaultProfile = ""
	p := fh.panels[0]
	p.Columns = panel.ColumnsModified
	p.WithIndex = true
	h.HandlePanelDirChanged(0)
	if p.Columns != panel.ColumnsModified || len(h.activeEntries[0]) != 0 {
		t.Fatalf("columns %v meta %v, want untouched", p.Columns.Names(), h.activeEntries[0])
	}
}

// A profile column from a local meta.toml must come back after a directory without it.
func TestHandleLoad_missingNamesStayActive(t *testing.T) {
	h, fh := columnsHandler(t)
	h.activate(0, []string{"meadow"})
	path := fh.panels[0].PathString()
	h.HandleLoad(LoadPayload{PanelID: 0, LoadGen: h.loadGen[0], Path: path, ActiveNames: h.activeEntries[0], MF: &metacmds.MetaFile{}})
	if h.model.MetaResults[0] != nil {
		t.Fatalf("results = %v, want no empty headers", h.model.MetaResults[0])
	}
	if !slices.Equal(h.activeEntries[0], []string{"meadow"}) {
		t.Fatalf("activeEntries = %v, want the name kept", h.activeEntries[0])
	}

	fh.panels[0] = testPanel(t, t.TempDir(), nil)
	gen := h.loadGen[0]
	h.HandlePanelDirChanged(0)
	if h.loadGen[0] == gen {
		t.Fatal("entering another directory must load the columns again")
	}
}

func TestActivateSelection_keepsHiddenNamesAndMatchesProfile(t *testing.T) {
	h, fh := columnsHandler(t)
	h.profiles = []columnprofile.Profile{{Name: "orchard", Columns: []string{"size"}, Meta: []string{"lantern", "meadow"}}}
	h.model.ColumnsDialog = dialog.ColumnsDialogState{
		Open: true, FileColumns: panel.ColumnsBrief,
		Entries: []dialog.MetaEntry{{Name: "meadow"}}, Checked: []bool{true},
		HiddenMeta: []string{"lantern"},
	}
	h.ActivateSelection()
	if !slices.Equal(h.activeEntries[0], []string{"meadow", "lantern"}) {
		t.Fatalf("activeEntries = %v", h.activeEntries[0])
	}
	if fh.panels[0].ColumnProfile != "orchard" {
		t.Fatalf("profile = %q, want the matching profile named", fh.panels[0].ColumnProfile)
	}
}

// profileDialogHandler is a handler with the Columns dialog open and the real overlay keys.
func profileDialogHandler(t *testing.T) (*Handler, *fakeHost) {
	t.Helper()
	h, fh := columnsHandler(t)
	keys, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 40)
	h.screen = screen
	h.keysColumnsDialog = keys.ColumnsDialog
	h.keysDialogInput = keys.DialogInput
	h.configDir = t.TempDir()
	h.OpenDialog(0)
	return h, fh
}

func press(h *Handler, key tcell.Key) { h.HandleDialogKey(tcell.NewEventKey(key, 0, tcell.ModNone)) }

func typeText(h *Handler, text string) {
	for _, r := range text {
		h.HandleDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

func TestColumnsDialog_saveLoadDeleteRoundTrip(t *testing.T) {
	h, _ := profileDialogHandler(t)
	st := &h.model.ColumnsDialog
	st.FileColumns = panel.ColumnsBrief.With(panel.ColumnOwner)

	press(h, tcell.KeyF5)
	if st.Phase != dialog.ColumnsPhaseSavePrompt {
		t.Fatalf("phase = %d, want the save prompt", st.Phase)
	}
	typeText(h, "orchard")
	press(h, tcell.KeyEnter)
	if st.Phase != dialog.ColumnsPhaseMain {
		t.Fatalf("phase = %d, want main after saving", st.Phase)
	}
	saved, err := columnprofile.Load(filepath.Join(h.configDir, config.DefaultColumnProfilesFileName))
	if err != nil || len(saved) != 1 || saved[0].Name != "orchard" || !slices.Equal(saved[0].Columns, []string{"size", "owner"}) {
		t.Fatalf("saved = %+v, err = %v", saved, err)
	}

	st.FileColumns = 0
	press(h, tcell.KeyF2)
	if st.Phase != dialog.ColumnsPhaseLoadPicker {
		t.Fatalf("phase = %d, want the load picker", st.Phase)
	}
	typeText(h, "orchard")
	press(h, tcell.KeyEnter)
	if st.Phase != dialog.ColumnsPhaseMain || st.FileColumns != panel.ColumnsBrief.With(panel.ColumnOwner) {
		t.Fatalf("phase = %d, columns = %v", st.Phase, st.FileColumns.Names())
	}

	press(h, tcell.KeyF2)
	typeText(h, "orchard")
	press(h, tcell.KeyF8)
	if len(h.profiles) != 0 {
		t.Fatalf("profiles = %+v after delete", h.profiles)
	}
}

func TestColumnsDialog_savePromptRefusesBuiltinAndEmptyNames(t *testing.T) {
	h, _ := profileDialogHandler(t)
	st := &h.model.ColumnsDialog
	press(h, tcell.KeyF5)
	press(h, tcell.KeyEnter)
	if st.Phase != dialog.ColumnsPhaseSavePrompt {
		t.Fatal("an empty name must not save")
	}
	typeText(h, config.ColumnProfileBrief)
	press(h, tcell.KeyEnter)
	if st.Phase != dialog.ColumnsPhaseSavePrompt || len(h.profiles) != 0 {
		t.Fatalf("phase = %d profiles = %+v, a built-in name must not save", st.Phase, h.profiles)
	}
}

func TestColumnsDialog_deleteBuiltinWarns(t *testing.T) {
	h, fh := profileDialogHandler(t)
	press(h, tcell.KeyF2)
	fh.messages = nil
	press(h, tcell.KeyF8) // the first row is a built-in
	if len(fh.messages) != 1 || fh.urgencies[len(fh.urgencies)-1] != ui.MessageUrgencyWarn {
		t.Fatalf("messages = %v", fh.messages)
	}
	if len(h.model.ColumnsDialog.Picker.Items) != len(columnprofile.Builtins()) {
		t.Fatal("a built-in must stay listed")
	}
}

func TestColumnsDialog_setDefaultPersists(t *testing.T) {
	h, fh := profileDialogHandler(t)
	press(h, tcell.KeyF2)
	press(h, tcell.KeyF6) // the first row
	want := columnprofile.Builtins()[0].Name
	if !slices.Equal(fh.saveCalls, []string{want}) || h.config.Panels.DefaultColumnProfile != want {
		t.Fatalf("saveCalls = %v, config = %q", fh.saveCalls, h.config.Panels.DefaultColumnProfile)
	}
	if h.model.ColumnsDialog.DefaultProfile != want {
		t.Fatalf("dialog default = %q", h.model.ColumnsDialog.DefaultProfile)
	}
}

func TestColumnsDialog_cancelReturnsThroughPhases(t *testing.T) {
	h, _ := profileDialogHandler(t)
	st := &h.model.ColumnsDialog
	press(h, tcell.KeyF2)
	h.ApplyDialogEvent(dlg.Event{ID: dlg.Cancel})
	if !st.Open || st.Phase != dialog.ColumnsPhaseMain {
		t.Fatalf("open = %v phase = %d, want back on main", st.Open, st.Phase)
	}
	h.DialogCancel()
	if h.model.ColumnsDialog.Open {
		t.Fatal("Cancel on main must close the dialog")
	}
}
