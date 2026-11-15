package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/filterpattern"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func newFilterPatternTestHandler(t *testing.T) (*Handler, *fakeMassRenamePatternHost) {
	t.Helper()
	h, fh := newMassRenamePatternTestHandler(t)
	keys, err := keymap.Build(keymap.DefaultPatternDialogOverlayKeys())
	if err != nil {
		t.Fatalf("keymap.Build: %v", err)
	}
	h.keysPatternDialog = keys
	h.model.FilterDialog = uidialog.FilterDialogState{
		Open: true, Target: uidialog.FilterTargetPanel, Text: "*.pebble", TextCursor: 8,
		PatternMode: panel.GroupPatternShell, Invert: true, Focus: uidialog.PatternCase,
	}
	h.model.FilterDialog.Types[uidialog.FilterTypeVideos] = true
	return h, fh
}

func fkey(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }

func typeRunes(h *Handler, s string) {
	for _, r := range s {
		h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

func loadFilters(t *testing.T, h *Handler) []filterpattern.Pattern {
	t.Helper()
	got, err := filterpattern.Load(h.savedPatternsPath())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestFilterSaveAndLoadPattern(t *testing.T) {
	h, _ := newFilterPatternTestHandler(t)
	fd := &h.model.FilterDialog

	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	if fd.Phase != uidialog.PatternPhaseSavePrompt {
		t.Fatalf("F5: phase = %v, want save prompt", fd.Phase)
	}
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	if fd.Phase != uidialog.PatternPhaseSavePrompt || len(loadFilters(t, h)) != 0 {
		t.Fatal("an empty name must not save")
	}
	typeRunes(h, "lantern")
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	if fd.Phase != uidialog.PatternPhaseMain || fd.Focus != uidialog.PatternCase {
		t.Fatalf("after save: phase %v focus %q, want main with its focus kept", fd.Phase, fd.Focus)
	}
	saved := loadFilters(t, h)
	want := filterpattern.Pattern{Name: "lantern", Pattern: "*.pebble", Mode: filterpattern.ModeShell,
		Types: []string{"videos"}, Invert: true, IncludeMeta: true}
	if len(saved) != 1 || !saved[0].Equal(want) {
		t.Fatalf("saved = %+v, want %+v", saved, want)
	}

	*fd = uidialog.FilterDialogState{Open: true, Text: "*.ext", PrefillPending: true, PatternMode: panel.GroupPatternSimple}
	h.HandleFilterDialogKey(fkey(tcell.KeyF2))
	if fd.Phase != uidialog.PatternPhaseLoadPicker || len(fd.Picker.Ranked) != 1 {
		t.Fatalf("F2: phase %v ranked %v", fd.Phase, fd.Picker.Ranked)
	}
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	if fd.Phase != uidialog.PatternPhaseMain || fd.Text != "*.pebble" || fd.PrefillPending ||
		fd.PatternMode != panel.GroupPatternShell || !fd.Invert || !fd.Types[uidialog.FilterTypeVideos] ||
		fd.Focus != uidialog.PatternInput || fd.TextCursor != 8 {
		t.Fatalf("after load: %+v", fd)
	}
}

func TestFilterOverwriteKeepsDescription(t *testing.T) {
	h, _ := newFilterPatternTestHandler(t)
	if err := filterpattern.Save(h.savedPatternsPath(), []filterpattern.Pattern{
		{Name: "harbor", Description: "willow clips", Pattern: "old"},
		{Name: "meadow", Pattern: "other"},
	}); err != nil {
		t.Fatal(err)
	}
	fd := &h.model.FilterDialog
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	if fd.Phase != uidialog.PatternPhaseOverwritePicker {
		t.Fatalf("F5 F5: phase = %v, want overwrite picker", fd.Phase)
	}
	h.FilterDialogCancel()
	if fd.Phase != uidialog.PatternPhaseSavePrompt {
		t.Fatalf("cancel from overwrite: phase = %v, want save prompt", fd.Phase)
	}
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	got := loadFilters(t, h)
	if fd.Phase != uidialog.PatternPhaseMain || len(got) != 2 || got[0].Pattern != "*.pebble" || got[0].Description != "willow clips" || got[1].Pattern != "other" {
		t.Fatalf("after overwrite: phase %v saved %+v", fd.Phase, got)
	}
	if h.savedPatternName != "harbor" {
		t.Fatalf("savedPatternName = %q", h.savedPatternName)
	}
}

func TestFilterLoadPickerDeleteAndPreselect(t *testing.T) {
	h, _ := newFilterPatternTestHandler(t)
	if err := filterpattern.Save(h.savedPatternsPath(), []filterpattern.Pattern{{Name: "cedar"}, {Name: "pebble"}, {Name: "maple"}}); err != nil {
		t.Fatal(err)
	}
	h.savedPatternName = "maple"
	fd := &h.model.FilterDialog
	h.HandleFilterDialogKey(fkey(tcell.KeyF2))
	if fd.Picker.Selected != 2 {
		t.Fatalf("Selected = %d, want the last used pattern (2)", fd.Picker.Selected)
	}
	if !h.PatternDeletePatternEligible() {
		t.Fatal("delete should be eligible with a selected entry")
	}
	h.HandleFilterDialogKey(fkey(tcell.KeyF8))
	got := loadFilters(t, h)
	if len(got) != 2 || got[1].Name != "pebble" || len(fd.Picker.Items) != 2 {
		t.Fatalf("after F8: saved %+v items %+v", got, fd.Picker.Items)
	}
	h.FilterDialogCancel()
	if fd.Phase != uidialog.PatternPhaseMain || !fd.Open {
		t.Fatalf("cancel from load picker: phase %v open %v", fd.Phase, fd.Open)
	}
	h.FilterDialogCancel()
	if fd.Open {
		t.Fatal("cancel on the main screen should close the dialog")
	}
}

func TestFilterHistory(t *testing.T) {
	h, _ := newFilterPatternTestHandler(t)
	apply := func(text string) {
		fd := &h.model.FilterDialog
		*fd = uidialog.FilterDialogState{Open: true, Text: text, PatternMode: panel.GroupPatternShell}
		h.executeFilterDialog()
	}
	apply("*.otter")
	apply("*.walrus")
	apply("*.otter")
	apply("")          // clearing is not recorded
	apply("[unclosed") // an invalid pattern is rejected, not recorded
	if len(h.filterHistory) != 2 || h.filterHistory[0].Pattern != "*.otter" || h.filterHistory[1].Pattern != "*.walrus" {
		t.Fatalf("history = %+v", h.filterHistory)
	}
	for i := range maxPatternHistory + 5 {
		apply("*.x" + string(rune('a'+i)))
	}
	if len(h.filterHistory) != maxPatternHistory {
		t.Fatalf("history len = %d, want %d", len(h.filterHistory), maxPatternHistory)
	}

	h.filterHistory = h.filterHistory[:2]
	fd := &h.model.FilterDialog
	*fd = uidialog.FilterDialogState{Open: true}
	h.HandleFilterDialogKey(fkey(tcell.KeyF3))
	if fd.Phase != uidialog.PatternPhaseHistoryPicker || len(fd.Picker.Items) != 2 {
		t.Fatalf("F3: phase %v items %+v", fd.Phase, fd.Picker.Items)
	}
	h.HandleFilterDialogKey(fkey(tcell.KeyF8))
	if len(h.filterHistory) != 1 || len(fd.Picker.Items) != 1 {
		t.Fatalf("after F8: history %+v items %+v", h.filterHistory, fd.Picker.Items)
	}
	want := h.filterHistory[0].Pattern
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	if fd.Phase != uidialog.PatternPhaseMain || fd.Text != want {
		t.Fatalf("history load: phase %v text %q, want %q", fd.Phase, fd.Text, want)
	}
}

func TestFilterOverwriteWithoutSavedPatterns(t *testing.T) {
	h, fh := newFilterPatternTestHandler(t)
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	if h.model.FilterDialog.Phase != uidialog.PatternPhaseSavePrompt {
		t.Fatalf("phase = %v, want the save prompt to stay", h.model.FilterDialog.Phase)
	}
	if len(fh.messages) == 0 || fh.messages[len(fh.messages)-1] != "No saved patterns" {
		t.Fatalf("messages = %v", fh.messages)
	}
}

func TestGroupSelectSavedPatternsShareStore(t *testing.T) {
	h, _ := newFilterPatternTestHandler(t) // saves "lantern" from the Filter dialog below
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	typeRunes(h, "lantern")
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	h.model.FilterDialog = uidialog.FilterDialogState{}

	var mainKeys, oks, closes int
	hooks := GroupSelectHooks{
		Key:   func(*tcell.EventKey) { mainKeys++ },
		Event: func(dlg.Event) {},
		OK:    func() { oks++ },
		Close: func() { closes++ },
	}
	gs := &h.model.GroupSelect
	*gs = uidialog.GroupSelectState{Open: true, Mode: "select", Context: "panel", Focus: uidialog.PatternCase}
	h.HandleGroupSelectKey(hooks, fkey(tcell.KeyF2))
	if gs.Phase != uidialog.PatternPhaseLoadPicker || len(gs.Picker.Items) != 1 {
		t.Fatalf("F2: phase %v items %+v", gs.Phase, gs.Picker.Items)
	}
	h.HandleGroupSelectKey(hooks, fkey(tcell.KeyEnter))
	if gs.Phase != uidialog.PatternPhaseMain || gs.Text != "*.pebble" || gs.PatternMode != panel.GroupPatternShell ||
		!gs.Types[uidialog.FilterTypeVideos] || gs.Focus != uidialog.PatternInput {
		t.Fatalf("after load: %+v", gs)
	}
	h.HandleGroupSelectKey(hooks, tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	h.GroupSelectOK(hooks)
	h.GroupSelectCancel(hooks)
	if mainKeys != 1 || oks != 1 || closes != 1 {
		t.Fatalf("main hooks: keys %d ok %d close %d, want 1 each", mainKeys, oks, closes)
	}

	h.HandleGroupSelectKey(hooks, fkey(tcell.KeyF5))
	for _, r := range "harbor" {
		h.HandleGroupSelectKey(hooks, tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	h.HandleGroupSelectKey(hooks, fkey(tcell.KeyEnter))
	if got := loadFilters(t, h); len(got) != 2 || got[1].Name != "harbor" || got[1].Invert {
		t.Fatalf("saved = %+v, want harbor appended without Invert", got)
	}

	h.RecordGroupSelectHistory()
	if len(h.groupSelectHistory) != 1 || len(h.filterHistory) != 0 {
		t.Fatalf("histories: group select %d filter %d, want separate lists", len(h.groupSelectHistory), len(h.filterHistory))
	}
	h.HandleGroupSelectKey(hooks, fkey(tcell.KeyF3))
	if gs.Phase != uidialog.PatternPhaseHistoryPicker || len(gs.Picker.Items) != 1 {
		t.Fatalf("F3: phase %v items %+v", gs.Phase, gs.Picker.Items)
	}
}
