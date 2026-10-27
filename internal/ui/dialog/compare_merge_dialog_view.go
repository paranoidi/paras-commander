package dialog

import (
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Compare merge widget IDs. The buttons are dlg.OK and dlg.Cancel.
const (
	CompareMergeLeft     = dlg.ID("left")
	CompareMergeRight    = dlg.ID("right")
	CompareMergeMissing  = dlg.ID("missing")
	CompareMergeModified = dlg.ID("modified")
	CompareMergeCopy     = dlg.ID("copy")
	CompareMergeMove     = dlg.ID("move")
	CompareMergeKeepName = dlg.ID("keep-name")
	CompareMergeSrcName  = dlg.ID("source-name")
)

// compareMergePathIndent is how far the destination paths sit in from the radio labels above them.
const compareMergePathIndent = 4

// CompareMergeView is the merge dialog's widget spec: the destination radios with their paths
// below (home-tilde, shared prefix on its own row), the Transfer checkboxes, the Copy / Move
// radios, the Relocated files name radios and the preview line, above OK / Cancel.
func CompareMergeView(s CompareMergeDialogState, userHomeDir string) dlg.Spec {
	sharedPrefix, leftPath, rightPath := FormatCompareMergePaths(s.PrimaryPath, s.SecondaryPath, userHomeDir)
	dest := []dlg.Item{{Kind: dlg.KindLabel, Label: "Destination:"}}
	if sharedPrefix != "" {
		dest = append(dest, dlg.Item{Kind: dlg.KindLabel, Prefix: "Shared: ", Label: sharedPrefix, Fit: dlg.FitPath})
	}
	dest = append(dest,
		dlg.Item{ID: CompareMergeLeft, Kind: dlg.KindRadio, Label: "Left side", Mnemonic: 'L', On: s.Direction == comparepkg.MergeTowardPrimary},
		dlg.Item{Kind: dlg.KindLabel, Label: leftPath, Fit: dlg.FitPath, Indent: compareMergePathIndent},
		dlg.Item{ID: CompareMergeRight, Kind: dlg.KindRadio, Label: "Right side", Mnemonic: 'R', On: s.Direction == comparepkg.MergeTowardSecondary},
		dlg.Item{Kind: dlg.KindLabel, Label: rightPath, Fit: dlg.FitPath, Indent: compareMergePathIndent},
		dlg.Item{Kind: dlg.KindSeparator},
	)
	return dlg.Spec{
		Title: "Merge", Width: PreferredFormDialogWidth, Unclamped: true,
		Groups: []dlg.Group{
			{Columns: [][]dlg.Item{dest}},
			{Columns: [][]dlg.Item{{
				{Kind: dlg.KindLabel, Label: "Transfer:"},
				{ID: CompareMergeMissing, Kind: dlg.KindCheckbox, Label: "Missing files", Mnemonic: 'M', On: s.CopyMissing},
				{ID: CompareMergeModified, Kind: dlg.KindCheckbox, Label: "Modified files (content differs)", Mnemonic: 'F', On: s.CopyModified},
				{Kind: dlg.KindSeparator},
			}}},
			{Columns: [][]dlg.Item{{
				{Kind: dlg.KindLabel, Label: "Operation:"},
				{ID: CompareMergeCopy, Kind: dlg.KindRadio, Label: "Copy (keep source files)", Mnemonic: 'K', On: !s.MoveMode},
				{ID: CompareMergeMove, Kind: dlg.KindRadio, Label: "Move (delete source after transfer)", Mnemonic: 'D', On: s.MoveMode},
				{Kind: dlg.KindSeparator},
			}}},
			{Columns: [][]dlg.Item{{
				{Kind: dlg.KindLabel, Label: "Relocated files:"},
				{ID: CompareMergeKeepName, Kind: dlg.KindRadio, Label: "Keep destination names", Mnemonic: 'E', On: !s.RelocatedUseSource},
				{ID: CompareMergeSrcName, Kind: dlg.KindRadio, Label: "Rename to source names", Mnemonic: 'S', On: s.RelocatedUseSource},
				{Kind: dlg.KindSeparator},
			}}},
			{Columns: [][]dlg.Item{{{Kind: dlg.KindLabel, Label: s.PreviewText}}}},
		},
		Buttons: []dlg.Button{
			dlg.OKButton,
			dlg.CancelButton,
		},
	}
}

// Apply folds an activation from dlg.HandleKey into the state: the radios pick the
// direction and the operation, the checkboxes toggle. OK and Cancel are the caller's to handle.
func (s *CompareMergeDialogState) Apply(ev dlg.Event) {
	switch ev.ID {
	case CompareMergeLeft:
		s.Direction = comparepkg.MergeTowardPrimary
	case CompareMergeRight:
		s.Direction = comparepkg.MergeTowardSecondary
	case CompareMergeMissing:
		s.CopyMissing = !s.CopyMissing
	case CompareMergeModified:
		s.CopyModified = !s.CopyModified
	case CompareMergeCopy:
		s.MoveMode = false
	case CompareMergeMove:
		s.MoveMode = true
	case CompareMergeKeepName:
		s.RelocatedUseSource = false
	case CompareMergeSrcName:
		s.RelocatedUseSource = true
	}
}
