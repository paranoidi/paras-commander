package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/filterpattern"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func newFilterPatternTestHandler(t *testing.T) (*Handler, *fakeMassRenamePatternHost) {
	t.Helper()
	h, fh := newMassRenamePatternTestHandler(t)
	keys, err := keymap.Build(keymap.DefaultFilterDialogOverlayKeys())
	if err != nil {
		t.Fatalf("keymap.Build: %v", err)
	}
	h.keysFilterDialog = keys
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
	got, err := filterpattern.Load(h.filterPatternsPath())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestFilterSaveAndLoadPattern(t *testing.T) {
	h, _ := newFilterPatternTestHandler(t)
	fd := &h.model.FilterDialog

	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	if fd.Phase != uidialog.FilterPhaseSavePrompt {
		t.Fatalf("F5: phase = %v, want save prompt", fd.Phase)
	}
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	if fd.Phase != uidialog.FilterPhaseSavePrompt || len(loadFilters(t, h)) != 0 {
		t.Fatal("an empty name must not save")
	}
	typeRunes(h, "lantern")
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	if fd.Phase != uidialog.FilterPhaseMain || fd.Focus != uidialog.PatternCase {
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
	if fd.Phase != uidialog.FilterPhaseLoadPicker || len(fd.Picker.Ranked) != 1 {
		t.Fatalf("F2: phase %v ranked %v", fd.Phase, fd.Picker.Ranked)
	}
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	if fd.Phase != uidialog.FilterPhaseMain || fd.Text != "*.pebble" || fd.PrefillPending ||
		fd.PatternMode != panel.GroupPatternShell || !fd.Invert || !fd.Types[uidialog.FilterTypeVideos] ||
		fd.Focus != uidialog.PatternInput || fd.TextCursor != 8 {
		t.Fatalf("after load: %+v", fd)
	}
}

func TestFilterOverwriteKeepsDescription(t *testing.T) {
	h, _ := newFilterPatternTestHandler(t)
	if err := filterpattern.Save(h.filterPatternsPath(), []filterpattern.Pattern{
		{Name: "harbor", Description: "willow clips", Pattern: "old"},
		{Name: "meadow", Pattern: "other"},
	}); err != nil {
		t.Fatal(err)
	}
	fd := &h.model.FilterDialog
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	if fd.Phase != uidialog.FilterPhaseOverwritePicker {
		t.Fatalf("F5 F5: phase = %v, want overwrite picker", fd.Phase)
	}
	h.FilterDialogCancel()
	if fd.Phase != uidialog.FilterPhaseSavePrompt {
		t.Fatalf("cancel from overwrite: phase = %v, want save prompt", fd.Phase)
	}
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	got := loadFilters(t, h)
	if fd.Phase != uidialog.FilterPhaseMain || len(got) != 2 || got[0].Pattern != "*.pebble" || got[0].Description != "willow clips" || got[1].Pattern != "other" {
		t.Fatalf("after overwrite: phase %v saved %+v", fd.Phase, got)
	}
	if h.filterPatternName != "harbor" {
		t.Fatalf("filterPatternName = %q", h.filterPatternName)
	}
}

func TestFilterLoadPickerDeleteAndPreselect(t *testing.T) {
	h, _ := newFilterPatternTestHandler(t)
	if err := filterpattern.Save(h.filterPatternsPath(), []filterpattern.Pattern{{Name: "cedar"}, {Name: "pebble"}, {Name: "maple"}}); err != nil {
		t.Fatal(err)
	}
	h.filterPatternName = "maple"
	fd := &h.model.FilterDialog
	h.HandleFilterDialogKey(fkey(tcell.KeyF2))
	if fd.Picker.Selected != 2 {
		t.Fatalf("Selected = %d, want the last used pattern (2)", fd.Picker.Selected)
	}
	if !h.FilterDeletePatternEligible() {
		t.Fatal("delete should be eligible with a selected entry")
	}
	h.HandleFilterDialogKey(fkey(tcell.KeyF8))
	got := loadFilters(t, h)
	if len(got) != 2 || got[1].Name != "pebble" || len(fd.Picker.Items) != 2 {
		t.Fatalf("after F8: saved %+v items %+v", got, fd.Picker.Items)
	}
	h.FilterDialogCancel()
	if fd.Phase != uidialog.FilterPhaseMain || !fd.Open {
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
	for i := range maxFilterHistory + 5 {
		apply("*.x" + string(rune('a'+i)))
	}
	if len(h.filterHistory) != maxFilterHistory {
		t.Fatalf("history len = %d, want %d", len(h.filterHistory), maxFilterHistory)
	}

	h.filterHistory = h.filterHistory[:2]
	fd := &h.model.FilterDialog
	*fd = uidialog.FilterDialogState{Open: true}
	h.HandleFilterDialogKey(fkey(tcell.KeyF3))
	if fd.Phase != uidialog.FilterPhaseHistoryPicker || len(fd.Picker.Items) != 2 {
		t.Fatalf("F3: phase %v items %+v", fd.Phase, fd.Picker.Items)
	}
	h.HandleFilterDialogKey(fkey(tcell.KeyF8))
	if len(h.filterHistory) != 1 || len(fd.Picker.Items) != 1 {
		t.Fatalf("after F8: history %+v items %+v", h.filterHistory, fd.Picker.Items)
	}
	want := h.filterHistory[0].Pattern
	h.HandleFilterDialogKey(fkey(tcell.KeyEnter))
	if fd.Phase != uidialog.FilterPhaseMain || fd.Text != want {
		t.Fatalf("history load: phase %v text %q, want %q", fd.Phase, fd.Text, want)
	}
}

func TestFilterOverwriteWithoutSavedPatterns(t *testing.T) {
	h, fh := newFilterPatternTestHandler(t)
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	h.HandleFilterDialogKey(fkey(tcell.KeyF5))
	if h.model.FilterDialog.Phase != uidialog.FilterPhaseSavePrompt {
		t.Fatalf("phase = %v, want the save prompt to stay", h.model.FilterDialog.Phase)
	}
	if len(fh.messages) == 0 || fh.messages[len(fh.messages)-1] != "No saved filters" {
		t.Fatalf("messages = %v", fh.messages)
	}
}
