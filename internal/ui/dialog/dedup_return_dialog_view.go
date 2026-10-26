package dialog

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Dedup return dialog button IDs. The third button is dlg.Cancel. In scope mode First is
// "Selected" and Second "Directory"; otherwise First is "Show" and Second "Rescan".
const (
	DedupReturnFirst  = dlg.ID("first")
	DedupReturnSecond = dlg.ID("second")
)

// dedupReturnWidth is the dialog width; the message path is fitted to its inner width.
const dedupReturnWidth = max(PreferredFormDialogWidth, 52)

// DedupReturnView is the "kept results" / scan-scope modal's widget spec. root, groups and wasted
// summarise the kept results (already filtered by the view's ignore-empty toggle) and only shape
// the message lines, never the buttons, so key handling may pass zeros. In scope mode
// (state.ScopeDirs) the dialog asks to search state.Dir or only the selection.
func DedupReturnView(s DedupReturnDialogState, root string, groups int, wasted int64, userHomeDir string) dlg.Spec {
	lead, tail := "Results for ", " are kept."
	line2 := fmt.Sprintf("%d groups, %s wasted", groups, formatDedupByteSize(wasted))
	first, second := "Show", "Rescan"
	if n := len(s.ScopeDirs); n > 0 {
		root, lead, tail = s.Dir, "Search ", ""
		line2 = fmt.Sprintf("or only the %d selected directories?", n)
		if n == 1 {
			line2 = "or only the selected directory?"
		}
		first, second = "Selected", "Directory"
	}
	path := primitive.FitPathForWidth(primitive.PathWithHomeTilde(root, userHomeDir), max(dedupReturnWidth-4-len(lead)-len(tail), 1))
	return dlg.Spec{
		Title: "Find Duplicates", Width: dedupReturnWidth, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: lead + path + tail},
			{Kind: dlg.KindLabel, Label: line2},
		}}}},
		Buttons: []dlg.Button{
			{ID: DedupReturnFirst, Label: first, Mnemonic: rune(first[0])},
			{ID: DedupReturnSecond, Label: second, Mnemonic: rune(second[0])},
			dlg.CancelButton,
		},
	}
}
