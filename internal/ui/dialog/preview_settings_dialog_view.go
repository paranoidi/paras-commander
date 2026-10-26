package dialog

import (
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Widget IDs of the preview settings dialog. The capability radios are named by their row and
// value (PreviewCapabilityID), the protocol and metadata radios by their value.
const (
	PreviewSixel       = "sixel"
	PreviewKitty       = "kitty"
	PreviewPlaceholder = "placeholder"

	PreviewVideoMetadata = dlg.ID("video")

	previewProtocolPrefix = "protocol:"
	previewMetadataPrefix = "metadata:"
)

// PreviewCapabilityID is the ID of the radio for value in the capability row (PreviewSixel,
// PreviewKitty or PreviewPlaceholder).
func PreviewCapabilityID(row, value string) dlg.ID { return dlg.ID(row + ":" + value) }

// PreviewProtocolID is the ID of the Active protocol radio for value.
func PreviewProtocolID(value string) dlg.ID { return dlg.ID(previewProtocolPrefix + value) }

// PreviewMetadataID is the ID of the Image metadata radio for value.
func PreviewMetadataID(value string) dlg.ID { return dlg.ID(previewMetadataPrefix + value) }

// previewCapabilityRows are the horizontal capability rows in display order.
var previewCapabilityRows = []struct {
	id, label string
	mnemonic  rune // Alt+letter steps the row to its next value
}{
	{PreviewSixel, "Sixel", 's'},
	{PreviewKitty, "Kitty", 'k'},
	{PreviewPlaceholder, "Placeholder", 'p'},
}

// previewCapabilityLabelW is the label box of the capability rows: the longest label
// ("Placeholder") plus the cell its text starts in, so the radios line up.
const previewCapabilityLabelW = len("Placeholder") + 1

// previewRadioItems lists radios as one item per row of a column; selected is the persisted value.
func previewRadioItems(radios []PreviewSettingsDialogRadio, selected string, id func(string) dlg.ID) []dlg.Item {
	items := make([]dlg.Item, len(radios))
	for i, r := range radios {
		items[i] = dlg.Item{ID: id(r.Value), Kind: dlg.KindRadio, Label: r.Label, Mnemonic: r.Shortcut, On: selected == r.Value}
	}
	return items
}

// PreviewSettingsDialogView is the preview settings dialog's widget spec: the three capability
// rows (a label and its auto / yes / no radios on one row, each row a Tab stop; the label's
// mnemonic steps the row to its next value), the Active
// protocol and Image metadata radio groups and the Video metadata checkbox.
func PreviewSettingsDialogView(s PreviewSettingsDialogState, markerWidth int) dlg.Spec {
	values := map[string]string{PreviewSixel: s.Sixel, PreviewKitty: s.Kitty, PreviewPlaceholder: s.KittyPlaceholder}
	caps := []dlg.Item{{Kind: dlg.KindLabel, Label: "Confirm terminal capabilities:"}}
	for seg, row := range previewCapabilityRows {
		cells := []dlg.Item{{Kind: dlg.KindLabel, Label: row.label, Mnemonic: row.mnemonic, Width: previewCapabilityLabelW}}
		for _, r := range PreviewSettingsDialogCapabilityRadios() {
			cells = append(cells, dlg.Item{
				ID: PreviewCapabilityID(row.id, r.Value), Kind: dlg.KindRadio, Label: r.Label, Mnemonic: r.Shortcut,
				On: values[row.id] == r.Value, Seg: seg, Gap: 1,
			})
		}
		caps = append(caps, dlg.Item{Kind: dlg.KindRow, Cells: cells})
	}
	caps = append(caps, dlg.Item{Kind: dlg.KindSeparator})
	protocol := append([]dlg.Item{{Kind: dlg.KindLabel, Label: "Active protocol:"}},
		previewRadioItems(PreviewSettingsDialogProtocolRadios(), s.Protocol, PreviewProtocolID)...)
	protocol = append(protocol, dlg.Item{Kind: dlg.KindSeparator})
	metadata := append([]dlg.Item{{Kind: dlg.KindLabel, Label: "Image metadata:"}},
		previewRadioItems(PreviewSettingsDialogImageMetadataRadios(), s.ImageMetadata, PreviewMetadataID)...)
	metadata = append(metadata, dlg.Item{Kind: dlg.KindSeparator})
	return dlg.Spec{
		Title: "Preview settings", Width: 46, MinWidth: 42, Unclamped: true, PlainButtonKeys: true,
		MarkerWidth: markerWidth, // the capability rows follow the theme's marker icons (dlg.MarkerWidth)
		Groups: []dlg.Group{
			{Columns: [][]dlg.Item{caps}},
			{Columns: [][]dlg.Item{protocol}},
			{Columns: [][]dlg.Item{metadata}},
			{Columns: [][]dlg.Item{{
				{ID: PreviewVideoMetadata, Kind: dlg.KindCheckbox, Label: "Video metadata", Mnemonic: 'v', On: s.VideoMetadata},
			}}},
		},
		Buttons: []dlg.Button{
			dlg.OKButton,
			dlg.CancelButton,
		},
	}
}

// SetKitty writes the Kitty radio and, since Unicode-placeholder display requires Kitty protocol
// support, drops a "yes" placeholder whenever Kitty is not "yes".
func (s *PreviewSettingsDialogState) SetKitty(v string) {
	s.Kitty = v
	if v != config.PreviewTerminalCapabilityYes && s.KittyPlaceholder == config.PreviewTerminalCapabilityYes {
		s.KittyPlaceholder = v
	}
}

// SetKittyPlaceholder writes the placeholder radio and, since it implies Kitty protocol support,
// forces Kitty to "yes" whenever placeholder is "yes".
func (s *PreviewSettingsDialogState) SetKittyPlaceholder(v string) {
	s.KittyPlaceholder = v
	if v == config.PreviewTerminalCapabilityYes {
		s.Kitty = config.PreviewTerminalCapabilityYes
	}
}

// Apply folds an activation from dlg.HandleKey into the state: a radio selects its value (Kitty
// and placeholder keep each other consistent), the checkbox toggles. OK and Cancel are the
// caller's to handle.
func (s *PreviewSettingsDialogState) Apply(ev dlg.Event) {
	if ev.ID == PreviewVideoMetadata {
		s.VideoMetadata = !s.VideoMetadata
		return
	}
	for _, r := range PreviewSettingsDialogCapabilityRadios() {
		switch ev.ID {
		case PreviewCapabilityID(PreviewSixel, r.Value):
			s.Sixel = r.Value
		case PreviewCapabilityID(PreviewKitty, r.Value):
			s.SetKitty(r.Value)
		case PreviewCapabilityID(PreviewPlaceholder, r.Value):
			s.SetKittyPlaceholder(r.Value)
		}
	}
	for _, r := range PreviewSettingsDialogProtocolRadios() {
		if ev.ID == PreviewProtocolID(r.Value) {
			s.Protocol = r.Value
		}
	}
	for _, r := range PreviewSettingsDialogImageMetadataRadios() {
		if ev.ID == PreviewMetadataID(r.Value) {
			s.ImageMetadata = r.Value
		}
	}
}
