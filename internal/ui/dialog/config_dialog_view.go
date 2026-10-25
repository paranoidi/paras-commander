package dialog

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Widget IDs of the Configuration dialog. The radios and inputs are named by their row; the OK
// and Cancel buttons are dlg.OK and dlg.Cancel. The Yes / No sub-confirms use ConfigConfirmYes
// and ConfigConfirmNo.
const (
	ConfigNerdfont        = dlg.ID("nerdfont")
	ConfigZoom            = dlg.ID("zoom")
	ConfigHorizontalSplit = dlg.ID("hsplit")

	ConfigConfirmYes = dlg.ID("yes")
	ConfigConfirmNo  = dlg.ID("no")
)

const (
	configScrollModePrefix = "scroll-mode-"
	configScrollbarPrefix  = "scrollbar-"
	configListingPrefix    = "listing-"
	configSplitPrefix      = "split-"
)

// ConfigScrollModeID names the scroll-mode radio of row i of panel.ScrollModeDialogRadios.
func ConfigScrollModeID(i int) dlg.ID { return dlg.ID(fmt.Sprintf("%s%d", configScrollModePrefix, i)) }

// ConfigScrollbarID names the scrollbar-style radio of row i of uiscrollbar.DialogRadios.
func ConfigScrollbarID(i int) dlg.ID { return dlg.ID(fmt.Sprintf("%s%d", configScrollbarPrefix, i)) }

// ConfigListingID names the listing-format radio of row i of panel.ListFormatDialogRadios.
func ConfigListingID(i int) dlg.ID { return dlg.ID(fmt.Sprintf("%s%d", configListingPrefix, i)) }

// ConfigSplitID names carousel split input i (0 parent, 1 center, 2 child).
func ConfigSplitID(i int) dlg.ID { return dlg.ID(fmt.Sprintf("%s%d", configSplitPrefix, i)) }

func configIDIndex(id dlg.ID, prefix string) (int, bool) {
	var i int
	n, _ := fmt.Sscanf(string(id), prefix+"%d", &i)
	return i, n == 1 && id == dlg.ID(fmt.Sprintf("%s%d", prefix, i))
}

// ConfigSplitIndex is the inverse of ConfigSplitID: the split input a focus ID names.
func ConfigSplitIndex(id dlg.ID) (int, bool) {
	i, ok := configIDIndex(id, configSplitPrefix)
	return i, ok && i >= 0 && i < len(ConfigDialogState{}.Split)
}

const (
	configDialogScrollbarStyleLabel = "Scrollbar style:"
	// configDialogScrollbarBox is the width of the scrollbar-style column of the scroll block: a
	// marker cell, the label and a trailing cell, so the block sits in the middle of the dialog.
	configDialogScrollbarBox = 17
	configDialogSplitLabel   = "Carousel split (parent / center / child):"
	configDialogSplitInputW  = 6

	configDialogHorizontalSplitLabel = "Start in horizontal split mode"
)

// ConfigDialogView is the Configuration modal's widget spec: view checkboxes, the scroll-mode /
// scrollbar-style radio block, the default listing format, the three carousel split inputs and
// OK / Cancel. The scrollbar column and the gaps between the split inputs share the spare width
// (Grow cells), so they stay centered at any dialog width.
func ConfigDialogView(st ConfigDialogState) dlg.Spec {
	check := func(id dlg.ID, label string, m rune, on bool) dlg.Item {
		return dlg.Item{ID: id, Kind: dlg.KindCheckbox, Label: label, Mnemonic: m, On: on}
	}
	text := func(label string) dlg.Item { return dlg.Item{Kind: dlg.KindLabel, Label: label} }
	sep := dlg.Item{Kind: dlg.KindSeparator}
	spacer := dlg.Item{Kind: dlg.KindSpacer, Grow: true}

	view := []dlg.Item{
		text("View options:"),
		check(ConfigNerdfont, "Use nerdfont icons", 'd', st.UseNerdfontIcons),
		check(ConfigZoom, "Zoom active panel", 'z', st.ZoomActivePanel),
		check(ConfigHorizontalSplit, configDialogHorizontalSplitLabel, 'h', st.PaneSplitStacked),
		sep,
	}

	scrollRadios := panel.ScrollModeDialogRadios()
	sbRadios := uiscrollbar.DialogRadios()
	scrollMode := panel.EffectiveScrollMode(st.ScrollMode)
	sb := uiscrollbar.EffectiveStyle(st.PanelScrollbar)
	scroll := []dlg.Item{{Kind: dlg.KindRow, Cells: []dlg.Item{
		{Kind: dlg.KindLabel, Label: "Scroll mode:", Grow: true},
		{Kind: dlg.KindLabel, Label: configDialogScrollbarStyleLabel, Width: configDialogScrollbarBox},
		spacer,
	}}}
	for i := 0; i < max(len(scrollRadios), len(sbRadios)); i++ {
		cells := []dlg.Item{{Kind: dlg.KindSpacer, Grow: true}, {Kind: dlg.KindSpacer, Width: configDialogScrollbarBox}, spacer}
		if i < len(scrollRadios) {
			r := scrollRadios[i]
			cells[0] = dlg.Item{ID: ConfigScrollModeID(i), Kind: dlg.KindRadio, Label: r.Label, Mnemonic: r.Shortcut, On: scrollMode == r.Mode, Grow: true}
		}
		if i < len(sbRadios) {
			r := sbRadios[i]
			cells[1] = dlg.Item{ID: ConfigScrollbarID(i), Kind: dlg.KindRadio, Label: r.Label, Mnemonic: r.Shortcut, On: sb == r.Style, Width: configDialogScrollbarBox}
		}
		scroll = append(scroll, dlg.Item{Kind: dlg.KindRow, Cells: cells})
	}
	scroll = append(scroll, sep)

	lf := panel.EffectiveListFormat(st.ListFormat)
	listing := []dlg.Item{text("Default listing format:")}
	for i, r := range panel.ListFormatDialogRadios() {
		listing = append(listing, dlg.Item{ID: ConfigListingID(i), Kind: dlg.KindRadio, Label: r.Label, Mnemonic: r.Shortcut, On: lf == r.Format})
	}
	listing = append(listing, sep)

	var inputs []dlg.Item
	for i := range st.Split {
		in := fileFieldInput(ConfigSplitID(i), st.Split[i], !panelcarousel.ValidSplitToken(st.Split[i].Value, i))
		in.Width = configDialogSplitInputW + 1 // the box includes the cell before the text
		if i > 0 {
			inputs = append(inputs, spacer)
		}
		inputs = append(inputs, in)
	}
	split := []dlg.Item{
		text(configDialogSplitLabel),
		{Kind: dlg.KindRow, Cells: inputs},
		text(""),
	}

	return dlg.Spec{
		Title: "Configuration", Width: 54, MinWidth: 38, MinHeight: 22, PlainButtonKeys: true,
		Groups: []dlg.Group{
			{Columns: [][]dlg.Item{view}},
			{Columns: [][]dlg.Item{scroll}},
			{Columns: [][]dlg.Item{listing}},
			{Columns: [][]dlg.Item{split}},
		},
		Buttons: []dlg.Button{{ID: dlg.OK, Label: "OK", Mnemonic: 'O'}, {ID: dlg.Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

// Apply records an activation from the engine in the state: the checkboxes toggle and the radios
// are picked. The inputs and the buttons are the caller's.
func (st *ConfigDialogState) Apply(ev dlg.Event) {
	switch ev.ID {
	case ConfigNerdfont:
		st.UseNerdfontIcons = !st.UseNerdfontIcons
	case ConfigZoom:
		st.ZoomActivePanel = !st.ZoomActivePanel
	case ConfigHorizontalSplit:
		st.PaneSplitStacked = !st.PaneSplitStacked
	default:
		if i, ok := configIDIndex(ev.ID, configScrollModePrefix); ok && i < len(panel.ScrollModeDialogRadios()) {
			st.ScrollMode = panel.ScrollModeDialogRadios()[i].Mode
		} else if i, ok := configIDIndex(ev.ID, configScrollbarPrefix); ok && i < len(uiscrollbar.DialogRadios()) {
			st.PanelScrollbar = uiscrollbar.DialogRadios()[i].Style
		} else if i, ok := configIDIndex(ev.ID, configListingPrefix); ok && i < len(panel.ListFormatDialogRadios()) {
			st.ListFormat = panel.ListFormatDialogRadios()[i].Format
		}
	}
}

// ConfigEditStubView is the "config.toml does not exist, generate default and open it?"
// confirmation shown when F9 is pressed with no config.toml yet.
func ConfigEditStubView() dlg.Spec {
	return configConfirmSpec("Edit Config", false,
		"config.toml does not exist yet.", "Generate default and open in editor?")
}

// ConfigResetDefaultsView is the "delete config.toml and reset to defaults?" confirmation shown
// when F8 is pressed with a config.toml present.
func ConfigResetDefaultsView() dlg.Spec {
	return configConfirmSpec("Reset To Defaults", true,
		"This deletes config.toml and resets all", "settings to their defaults. Continue?")
}

func configConfirmSpec(title string, destructiveYes bool, line1, line2 string) dlg.Spec {
	return dlg.Spec{
		Title: title, Width: 56, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: line1}, {Kind: dlg.KindLabel, Label: line2},
		}}}},
		Buttons: []dlg.Button{
			{ID: ConfigConfirmYes, Label: "Yes", Mnemonic: 'Y', Destructive: destructiveYes},
			{ID: ConfigConfirmNo, Label: "No", Mnemonic: 'N'},
		},
	}
}
