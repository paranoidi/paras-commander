package dialog

import (
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// CompareFilterDialogMode picks what OK does with the chosen kind.
type CompareFilterDialogMode int

const (
	CompareFilterModeCategory CompareFilterDialogMode = iota // set the view filter
	CompareFilterModeSelect                                  // select matching rows
	CompareFilterModeUnselect                                // unselect matching rows
)

// CompareFilterDialogState is the compare category picker modal. Mode switches it
// from the view filter to select/unselect by kind (OK (un)selects matching rows instead).
// Focus is the focused widget: one of the Radios rows (see CompareFilterView), dlg.OK or dlg.Cancel.
// Filter is the pending radio selection (updated by Space/Enter/Alt mnemonic;
// applied only on OK).
type CompareFilterDialogState struct {
	Open   bool
	Mode   CompareFilterDialogMode
	Focus  dlg.ID
	Filter comparepkg.Filter
	// FocusSecondary marks the right column as focused (select/unselect "this side").
	FocusSecondary bool
}

// Radios returns the radio list for the dialog's mode.
func (s CompareFilterDialogState) Radios() []comparepkg.FilterDialogRadio {
	if s.Mode != CompareFilterModeCategory {
		return comparepkg.SelectDialogRadios(s.FocusSecondary)
	}
	return comparepkg.FilterDialogRadios()
}

// Title returns the dialog frame title.
func (s CompareFilterDialogState) Title() string {
	switch s.Mode {
	case CompareFilterModeSelect:
		return "Select"
	case CompareFilterModeUnselect:
		return "Unselect"
	}
	return "Category"
}

// FocusForFilter is the widget ID of the radio whose filter is exactly f. It is empty (the first
// widget) when f matches none, e.g. a combination of kinds.
func (s CompareFilterDialogState) FocusForFilter(f comparepkg.Filter) dlg.ID {
	for _, r := range s.Radios() {
		if r.Filter == f {
			return compareFilterItemID(r.Filter)
		}
	}
	return ""
}
