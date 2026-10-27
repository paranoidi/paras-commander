package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func statusCommandsGoldenState(focus dlg.ID) StatusCommandsDialogState {
	return StatusCommandsDialogState{
		Open: true,
		Rows: []StatusCommandsDialogRow{
			{Name: "loadavg", Label: "Load average", Shown: true},
			{Name: "memory", Label: "Memory used"},
			{Name: "harbor", Label: "Harbor lantern", Shown: true},
		},
		Focus: focus,
	}
}

func TestStatusCommandsGoldenRender(t *testing.T) {
	var cases []simpleGolden
	for _, c := range []struct {
		name  string
		focus dlg.ID
	}{
		{"80x24_focus-first", ""},
		{"80x24_focus-memory", dlg.ID(statusCommandPrefix + "memory")},
		{"80x24_focus-ok", dlg.OK},
	} {
		st := statusCommandsGoldenState(c.focus)
		cases = append(cases, simpleGolden{c.name, 80, 24, func(s tcell.Screen, l Layout) {
			DrawStatusCommandsDialog(s, l, st, asciiDialogStyles())
		}})
	}
	runSimpleGoldens(t, "status_commands", cases)
}

func TestStatusCommandsDialogApplyTogglesRow(t *testing.T) {
	st := statusCommandsGoldenState("")
	st.Apply(dlg.Event{ID: dlg.ID(statusCommandPrefix + "memory")})
	st.Apply(dlg.Event{ID: dlg.ID(statusCommandPrefix + "loadavg")})
	if st.Rows[0].Shown || !st.Rows[1].Shown || !st.Rows[2].Shown {
		t.Fatalf("rows after toggles = %+v", st.Rows)
	}
}
