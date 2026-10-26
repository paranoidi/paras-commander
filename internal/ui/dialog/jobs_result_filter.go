package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// JobResultFilter is the set of operation results the jobs view Operations pane shows. Its bits
// are oplog.Mask bits, so it converts with a cast.
type JobResultFilter oplog.Mask

const (
	JobResultSuccess = JobResultFilter(1 << oplog.Done)
	JobResultSkipped = JobResultFilter(1 << oplog.Skipped)
	JobResultFailed  = JobResultFilter(1 << oplog.Failed)

	JobResultNone JobResultFilter = 0
	JobResultAll                  = JobResultFilter(oplog.MaskAll)
)

// Mask is f as an oplog view mask.
func (f JobResultFilter) Mask() oplog.Mask { return oplog.Mask(f) }

// jobResultKinds are the result checkboxes in display order, after the All master row.
var jobResultKinds = []struct {
	Filter   JobResultFilter
	Label    string
	Shortcut rune
}{
	{JobResultSuccess, "Success", 's'},
	{JobResultFailed, "Failed", 'f'},
	{JobResultSkipped, "Skipped", 'k'},
}

// Label names f for the Operations pane border: All, None, or the shown kinds joined by " + ".
func (f JobResultFilter) Label() string {
	switch f & JobResultAll {
	case JobResultAll:
		return "All"
	case JobResultNone:
		return "None"
	}
	var names []string
	for _, k := range jobResultKinds {
		if f&k.Filter != 0 {
			names = append(names, k.Label)
		}
	}
	return strings.Join(names, " + ")
}

// JobsResultFilterDialogState is the jobs view Operations result picker. Focus is the focused
// widget (a checkbox, dlg.OK or dlg.Cancel); Filter is the pending selection, applied only on OK.
type JobsResultFilterDialogState struct {
	Open   bool
	Focus  dlg.ID
	Filter JobResultFilter
}

func jobResultItemID(f JobResultFilter) dlg.ID { return dlg.IndexedID("result:", int(f)) }

// JobsResultFilterView is the result picker's widget spec: an All master checkbox and one
// checkbox per result above OK / Cancel.
func JobsResultFilterView(s JobsResultFilterDialogState) dlg.Spec {
	items := []dlg.Item{{
		ID: jobResultItemID(JobResultAll), Kind: dlg.KindCheckbox, Label: "All", Mnemonic: 'a',
		On: s.Filter&JobResultAll == JobResultAll,
	}}
	for _, k := range jobResultKinds {
		items = append(items, dlg.Item{
			ID: jobResultItemID(k.Filter), Kind: dlg.KindCheckbox, Label: k.Label, Mnemonic: k.Shortcut,
			On: s.Filter&k.Filter != 0,
		})
	}
	return dlg.Spec{
		Title: "Result", Width: 32, MinWidth: 24, MinHeight: len(items) + 4, // border, rows, separator, buttons, border
		Groups: []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{
			dlg.OKButton,
			dlg.CancelButton,
		},
	}
}

// Apply folds a checkbox activation from dlg.HandleKey into the pending filter. The All row is
// the master toggle: it clears everything when checked and sets everything otherwise.
func (s *JobsResultFilterDialogState) Apply(ev dlg.Event) {
	if ev.ID == jobResultItemID(JobResultAll) {
		if s.Filter&JobResultAll == JobResultAll {
			s.Filter = JobResultNone
		} else {
			s.Filter = JobResultAll
		}
		return
	}
	for _, k := range jobResultKinds {
		if ev.ID == jobResultItemID(k.Filter) {
			s.Filter ^= k.Filter
			return
		}
	}
}

// DrawJobsResultFilterDialog paints the result picker from its widget spec.
func DrawJobsResultFilterDialog(screen tcell.Screen, layout Layout, state JobsResultFilterDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, JobsResultFilterView(state), state.Focus, styles)
}
