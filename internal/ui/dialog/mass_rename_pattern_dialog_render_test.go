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

// Each mass rename screen is a drawable dialog titled after the phase.
func TestFileDialogRectMassRenameSubPhases(t *testing.T) {
	layout := testLayout(120, 40)
	for _, tc := range []struct {
		phase MassRenamePhase
		title string
	}{
		{MassRenamePhaseSavePrompt, "Save pattern"},
		{MassRenamePhaseLoadPicker, "Load pattern"},
		{MassRenamePhaseHistoryPicker, "Pattern history"},
		{MassRenamePhaseOverwritePicker, "Overwrite pattern"},
	} {
		state := FileDialogState{
			Open:            true,
			DialogType:      FileDialogMassRename,
			MassRenamePhase: tc.phase,
			Fields:          []FileDialogField{{Label: "Name", Value: "harbor"}, {Label: "Description", Value: "strip camera prefix"}},
		}
		if _, ok := FileDialogRect(layout, state, 0); !ok {
			t.Fatalf("phase %d: expected a drawable rect", tc.phase)
		}
		if title := fileDialogOuterTitle(state); title != tc.title {
			t.Fatalf("phase %d: title = %q, want %q", tc.phase, title, tc.title)
		}
	}
}
