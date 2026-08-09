package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ops"
)

func TestMassRenamePatternSearchLine(t *testing.T) {
	cases := []struct {
		name string
		p    ops.MassRenamePattern
		want string
	}{
		{
			name: "saved pattern uses name and description",
			p:    ops.MassRenamePattern{Name: "harbor", Description: "strip prefix"},
			want: "harbor strip prefix",
		},
		{
			name: "unnamed history entry falls back to find/replace summary",
			p:    ops.MassRenamePattern{Mode: "simple", Find: "IMG_", Replace: ""},
			want: "IMG_ →",
		},
		{
			name: "unnamed capitalize history entry has no find/replace to show",
			p:    ops.MassRenamePattern{Mode: "capitalize"},
			want: "Capitalize",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MassRenamePatternSearchLine(tc.p); got != tc.want {
				t.Fatalf("MassRenamePatternSearchLine() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFileDialogRectMassRenameSavePromptPhase(t *testing.T) {
	layout := testLayout(120, 40)
	state := FileDialogState{
		Open:            true,
		DialogType:      FileDialogMassRename,
		MassRenamePhase: MassRenamePhaseSavePrompt,
		Fields: []FileDialogField{
			{Label: "Name", Value: "harbor"},
			{Label: "Description", Value: "strip camera prefix"},
		},
	}
	rect, ok := FileDialogRect(layout, state, 0)
	if !ok {
		t.Fatal("expected drawable rect for save-pattern prompt")
	}
	if rect.Height != massRenameSavePromptDialogHeight() {
		t.Fatalf("height = %d, want %d", rect.Height, massRenameSavePromptDialogHeight())
	}
	if title := fileDialogOuterTitle(state); title != "Save pattern" {
		t.Fatalf("title = %q, want %q", title, "Save pattern")
	}
}

func TestFileDialogRectMassRenameLoadPickerPhase(t *testing.T) {
	layout := testLayout(120, 40)
	state := FileDialogState{
		Open:            true,
		DialogType:      FileDialogMassRename,
		MassRenamePhase: MassRenamePhaseLoadPicker,
		Fields: []FileDialogField{
			{Label: "Find", Value: "a"},
			{Label: "Replace", Value: "b"},
		},
	}
	rect, ok := FileDialogRect(layout, state, 0)
	if !ok {
		t.Fatal("expected drawable rect for load-pattern picker")
	}
	if rect.Height != massRenamePatternPickerDialogHeight(layout.Height) {
		t.Fatalf("height = %d, want %d", rect.Height, massRenamePatternPickerDialogHeight(layout.Height))
	}
	if title := fileDialogOuterTitle(state); title != "Load pattern" {
		t.Fatalf("title = %q, want %q", title, "Load pattern")
	}
}

func TestFileDialogRectMassRenameHistoryPickerPhase(t *testing.T) {
	layout := testLayout(120, 40)
	state := FileDialogState{
		Open:            true,
		DialogType:      FileDialogMassRename,
		MassRenamePhase: MassRenamePhaseHistoryPicker,
		Fields: []FileDialogField{
			{Label: "Find", Value: "a"},
			{Label: "Replace", Value: "b"},
		},
	}
	rect, ok := FileDialogRect(layout, state, 0)
	if !ok {
		t.Fatal("expected drawable rect for pattern-history picker")
	}
	if rect.Height != massRenamePatternPickerDialogHeight(layout.Height) {
		t.Fatalf("height = %d, want %d", rect.Height, massRenamePatternPickerDialogHeight(layout.Height))
	}
	if title := fileDialogOuterTitle(state); title != "Pattern history" {
		t.Fatalf("title = %q, want %q", title, "Pattern history")
	}
}

func TestFileDialogRectMassRenamePatternPickerHeightClampsOnShortTerminal(t *testing.T) {
	layout := testLayout(120, 15)
	got := massRenamePatternPickerDialogHeight(layout.Height)
	if got > layout.Height-2 {
		t.Fatalf("height = %d, must fit within layout height %d minus 2", got, layout.Height)
	}
}

func TestMassRenameOKCancelFocusIndicesForSubPhases(t *testing.T) {
	savePrompt := FileDialogState{
		DialogType:      FileDialogMassRename,
		MassRenamePhase: MassRenamePhaseSavePrompt,
		Fields:          []FileDialogField{{Label: "Name"}, {Label: "Description"}},
	}
	if ok := FileDialogOKFocusIndex(savePrompt); ok != 2 {
		t.Fatalf("save prompt OK focus = %d, want 2", ok)
	}
	if cancel := FileDialogCancelFocusIndex(savePrompt); cancel != 3 {
		t.Fatalf("save prompt Cancel focus = %d, want 3", cancel)
	}

	loadPicker := FileDialogState{
		DialogType:      FileDialogMassRename,
		MassRenamePhase: MassRenamePhaseLoadPicker,
	}
	if ok := FileDialogOKFocusIndex(loadPicker); ok != 1 {
		t.Fatalf("load picker OK focus = %d, want 1", ok)
	}
	if cancel := FileDialogCancelFocusIndex(loadPicker); cancel != 2 {
		t.Fatalf("load picker Cancel focus = %d, want 2", cancel)
	}

	historyPicker := FileDialogState{
		DialogType:      FileDialogMassRename,
		MassRenamePhase: MassRenamePhaseHistoryPicker,
	}
	if ok := FileDialogOKFocusIndex(historyPicker); ok != 1 {
		t.Fatalf("history picker OK focus = %d, want 1", ok)
	}
	if cancel := FileDialogCancelFocusIndex(historyPicker); cancel != 2 {
		t.Fatalf("history picker Cancel focus = %d, want 2", cancel)
	}
}
