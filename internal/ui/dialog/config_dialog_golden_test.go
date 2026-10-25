package dialog

import (
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Render goldens for the Configuration dialog and its two Yes/No sub-confirms: every focus
// position, option states, valid / invalid / placeholder / overflowing carousel split inputs,
// the odd-leftover width, narrow widths and the refused sizes. Focus is named; configGolden* are
// the only code that knows how the state encodes it.

func configGoldenField(value, prefill string) FileDialogField {
	f := FileDialogField{Value: value, Prefill: prefill, Cursor: utf8.RuneCountInString(value)}
	return f
}

func configGoldenState(focus string) ConfigDialogState {
	st := ConfigDialogState{
		Open: true, ScrollMode: panel.ScrollModeEdge, PanelScrollbar: uiscrollbar.StyleThumb, ListFormat: panel.ListFormatMtime,
		Split: [3]FileDialogField{configGoldenField("<<22%", "<<22%"), configGoldenField("<44%", "<44%"), configGoldenField("*", "*")},
	}
	configGoldenSetFocus(&st, focus)
	return st
}

func configGoldenSetFocus(st *ConfigDialogState, name string) {
	var n int
	switch name {
	case "nerdfont":
		st.Focus = ConfigNerdfont
	case "zoom":
		st.Focus = ConfigZoom
	case "hsplit":
		st.Focus = ConfigHorizontalSplit
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		if c, _ := fmt.Sscanf(name, "mode%d", &n); c == 1 {
			st.Focus = ConfigScrollModeID(n)
		} else if c, _ := fmt.Sscanf(name, "bar%d", &n); c == 1 {
			st.Focus = ConfigScrollbarID(n)
		} else if c, _ := fmt.Sscanf(name, "listing%d", &n); c == 1 {
			st.Focus = ConfigListingID(n)
		} else if c, _ := fmt.Sscanf(name, "split%d", &n); c == 1 {
			st.Focus = ConfigSplitID(n)
		} else {
			panic("unknown config focus " + name)
		}
	}
}

func TestConfigDialogGoldenRender(t *testing.T) {
	goldenBandRows["config_dialog"] = 28
	var cases []simpleGolden
	add := func(name string, w, h int, st ConfigDialogState) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawConfigDialog(s, l, st, asciiDialogStyles())
		}})
	}
	focuses := []string{"nerdfont", "zoom", "hsplit", "mode0", "bar0", "mode1", "bar1", "mode2", "bar2", "listing0", "listing1", "listing2", "split0", "split1", "split2", "ok", "cancel"}
	for _, f := range focuses {
		add("focus-"+f, 80, 30, configGoldenState(f))
	}

	allOn := configGoldenState("zoom")
	allOn.UseNerdfontIcons, allOn.ZoomActivePanel, allOn.PaneSplitStacked = true, true, true
	allOn.ScrollMode, allOn.PanelScrollbar, allOn.ListFormat = panel.ScrollModeCenter, uiscrollbar.StyleBar, panel.ListFormatBrief
	add("all_on", 80, 30, allOn)
	other := configGoldenState("listing1")
	other.ScrollMode, other.PanelScrollbar, other.ListFormat = panel.ScrollModeMinimal, uiscrollbar.StyleNone, panel.ListFormatPerm
	add("other_options", 80, 30, other)

	invalid := configGoldenState("split1")
	invalid.Split[0] = configGoldenField("bogus", "<<22%")
	invalid.Split[2] = configGoldenField("<5%", "*")
	add("split_invalid", 80, 30, invalid)
	placeholder := configGoldenState("split0")
	placeholder.Split[0].PrefillPending = true
	placeholder.Split[1].PrefillPending = true
	placeholder.Split[2] = configGoldenField("", "*")
	add("split_placeholder", 80, 30, placeholder)
	long := configGoldenState("split2")
	long.Split[0] = configGoldenField("<<22%abcdefgh", "<<22%")
	long.Split[1].Cursor, long.Split[1].Scroll = 2, 0
	long.Split[2] = configGoldenField("amberquartz", "*")
	add("split_overflow", 80, 30, long)

	// Widths: the preferred 54, the odd leftover (55 would be needed), narrower, the 38 minimum
	// and below it (nothing is drawn); heights: the 22 minimum and below it.
	add("w120", 120, 30, configGoldenState("ok"))
	add("w54", 54, 30, configGoldenState("zoom"))
	add("w53", 53, 30, configGoldenState("mode1"))
	add("w52", 52, 30, configGoldenState("bar1"))
	add("w45", 45, 30, configGoldenState("split1"))
	add("w44_odd", 44, 30, configGoldenState("listing0"))
	add("w40", 40, 30, configGoldenState("cancel"))
	add("w38", 38, 30, configGoldenState("ok"))
	add("w37_refused", 37, 30, configGoldenState("ok"))
	add("h22", 80, 22, configGoldenState("ok"))
	add("h21_refused", 80, 21, configGoldenState("ok"))
	add("h24", 80, 24, configGoldenState("hsplit"))

	// Yes/No sub-confirms.
	for i, f := range []dlg.ID{ConfigConfirmYes, ConfigConfirmNo} {
		st := ConfigDialogState{Open: true, EditStubConfirm: true, EditStubConfirmFocus: f}
		add(fmt.Sprintf("editstub_focus%d", i), 80, 24, st)
		rs := ConfigDialogState{Open: true, ResetDefaultsConfirm: true, ResetDefaultsConfirmFocus: f}
		add(fmt.Sprintf("reset_focus%d", i), 80, 24, rs)
	}
	st := ConfigDialogState{Open: true, EditStubConfirm: true}
	add("editstub_narrow60", 60, 24, st)
	add("editstub_narrow40", 40, 24, st)
	add("editstub_short6", 80, 6, st)
	rs := ConfigDialogState{Open: true, ResetDefaultsConfirm: true}
	add("reset_narrow60", 60, 24, rs)
	add("reset_narrow40", 40, 24, rs)
	add("reset_short6", 80, 6, rs)
	runSimpleGoldens(t, "config_dialog", cases)
}
