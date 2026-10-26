package dialog

import (
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const (
	sortOptDiskUsage = dlg.ID("opt:disk-usage")
	sortOptReverse   = dlg.ID("opt:reverse")
	sortOptDirsFirst = dlg.ID("opt:dirs-first")
	sortModePrefix   = "mode:"
	sortMetaPrefix   = "meta:"
)

// SortDialogView is the sort dialog's widget spec. Group 0 is the two-column radio grid: built-in
// sort modes on the left and the active meta columns (capped, see MetaCount) on the right sharing
// their rows, so the dialog never grows taller. Group 1 is the option checkboxes.
func SortDialogView(s SortDialogState) dlg.Spec {
	var builtin, meta []dlg.Item
	for _, m := range panel.SortDialogRadios() {
		builtin = append(builtin, dlg.Item{
			ID: dlg.ID(sortModePrefix + m.Label), Kind: dlg.KindRadio, Label: m.Label, Mnemonic: m.Shortcut,
			On: s.SortMode == m.Mode,
		})
	}
	for _, r := range s.MetaRadios[:s.MetaCount()] {
		// No mnemonics: shortcut letters are taken by the built-in rows and meta names are user-defined.
		meta = append(meta, dlg.Item{
			ID: dlg.ID(sortMetaPrefix + r.Name), Kind: dlg.KindRadio, Label: r.Title,
			On: s.SortMode == panel.SortMeta && s.MetaColumn == r.Name,
		})
	}
	cols := [][]dlg.Item{builtin}
	if len(meta) > 0 {
		cols = append(cols, meta)
	}
	return dlg.Spec{
		Title: "Sort order", Width: 56, MinWidth: 30, MinHeight: 11, PlainButtonKeys: true,
		Groups: []dlg.Group{
			{Columns: cols},
			{Columns: [][]dlg.Item{{
				{ID: sortOptDiskUsage, Kind: dlg.KindCheckbox, Label: "Disk usage", Mnemonic: 'u', On: s.DiskUsageIdleSizeSort},
				{ID: sortOptReverse, Kind: dlg.KindCheckbox, Label: "Reverse", Mnemonic: 'r', On: s.SortReverse},
				{ID: sortOptDirsFirst, Kind: dlg.KindCheckbox, Label: "Directories first", Mnemonic: 'd', On: s.DirectoriesFirst},
			}}},
		},
		Buttons: []dlg.Button{
			dlg.OKButton,
			dlg.CancelButton,
		},
	}
}

// Apply folds an activation from dlg.HandleKey into the state: radios select their
// sort mode or meta column, checkboxes toggle. OK and Cancel are the caller's to handle.
func (s *SortDialogState) Apply(ev dlg.Event) {
	switch ev.ID {
	case sortOptDiskUsage:
		s.DiskUsageIdleSizeSort = !s.DiskUsageIdleSizeSort
	case sortOptReverse:
		s.SortReverse = !s.SortReverse
	case sortOptDirsFirst:
		s.DirectoriesFirst = !s.DirectoriesFirst
	default:
		for _, m := range panel.SortDialogRadios() {
			if ev.ID == dlg.ID(sortModePrefix+m.Label) {
				s.SortMode = m.Mode
				return
			}
		}
		for _, r := range s.MetaRadios[:s.MetaCount()] {
			if ev.ID == dlg.ID(sortMetaPrefix+r.Name) {
				s.SortMode = panel.SortMeta
				s.MetaColumn = r.Name
				return
			}
		}
	}
}
