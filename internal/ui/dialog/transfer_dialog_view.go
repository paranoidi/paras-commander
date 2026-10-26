package dialog

import (
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Transfer dialog widget IDs. The buttons are dlg.OK, TransferAddPaused and dlg.Cancel.
const (
	TransferDest      = dlg.ID("dest")
	TransferNewName   = dlg.ID("new-name")
	TransferPerm      = dlg.ID("preserve-permissions")
	TransferTimes     = dlg.ID("preserve-timestamps")
	TransferDeref     = dlg.ID("dereference-symlinks")
	TransferFlatten   = dlg.ID("flatten-into-destination")
	TransferAddPaused = dlg.ID("add-paused")
	// transferResult is the multi-location Result preview, a ScrollOnly list.
	transferResult = dlg.ID("result")
)

// TransferViewContext carries what TransferView needs besides the state: the screen layout (the
// preview list height follows it), the home directory used to abbreviate the common root, and the
// cells kept free before each preview row for an icon (0 = none).
type TransferViewContext struct {
	Layout      Layout
	UserHomeDir string
	IconLead    int
	// ScrollbarStyle draws the Result preview's scrollbar.
	ScrollbarStyle uiscrollbar.Style
}

// TransferView is the copy / move dialog's widget spec. The self-copy phase asks for a new name.
// The destination phase has the destination input, for a copy the preserve / dereference
// checkboxes, and for a multi-location selection the Source header, the Flatten into destination
// checkbox and the Result preview, a ScrollOnly list whose per-type devicons DrawTransferDialog
// paints into the cells each row keeps free; the destination's completion dropdown is painted from
// the input's placed rect. Buttons are OK,
// Add paused and Cancel in both phases.
func TransferView(st TransferDialogState, ctx TransferViewContext) dlg.Spec {
	title := "Copy"
	if st.Kind == TransferKindMove {
		title = "Move"
	}
	spec := dlg.Spec{
		Width: transferDestinationFrameWidth(ctx.Layout, st, ctx.UserHomeDir, ctx.IconLead), Unclamped: true,
		Buttons: []dlg.Button{
			dlg.OKButton,
			{ID: TransferAddPaused, Label: "Add paused", Mnemonic: 'p'},
			dlg.CancelButton,
		},
	}
	if st.Phase == TransferPhaseSelfCopyRename {
		reason := "Cannot copy onto itself."
		spec.Title = title + " — New name"
		if st.Kind == TransferKindMove {
			reason = "Cannot move onto itself."
		}
		spec.Groups = []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: reason},
			{Kind: dlg.KindLabel},
			{Kind: dlg.KindLabel, Label: "New name:"},
			fileFieldInput(TransferNewName, st.SelfCopyNewName, st.SelfCopyNewName.InputInvalid),
		}}}}
		return spec
	}
	spec.Title = title
	var head []dlg.Item
	if st.MultiLocation() {
		head = append(head,
			dlg.Item{Kind: dlg.KindLabel, Label: "Source:"},
			dlg.Item{Kind: dlg.KindLabel, Label: primitive.PathWithHomeTilde(st.CommonRoot, ctx.UserHomeDir), Fit: dlg.FitPath},
			dlg.Item{Kind: dlg.KindLabel})
	}
	destInvalid := st.DestPathInvalid && !st.DestPathCheckPending
	head = append(head, dlg.Item{Kind: dlg.KindLabel, Label: "Destination:"}, fileFieldInput(TransferDest, st.Destination, destInvalid))
	var options []dlg.Item
	if st.Kind == TransferKindCopy {
		options = append(options,
			dlg.Item{ID: TransferPerm, Kind: dlg.KindCheckbox, Label: "Preserve permissions", Mnemonic: 'r', On: st.PreservePermissions},
			dlg.Item{ID: TransferTimes, Kind: dlg.KindCheckbox, Label: "Preserve timestamps", Mnemonic: 't', On: st.PreserveTimestamps},
			dlg.Item{ID: TransferDeref, Kind: dlg.KindCheckbox, Label: "Dereference symlinks", Mnemonic: 'd', On: st.DereferenceSymlinks, Inert: st.DereferenceUnsupported()})
	}
	if st.MultiLocation() {
		options = append(options, dlg.Item{ID: TransferFlatten, Kind: dlg.KindCheckbox, Label: "Flatten into destination", Mnemonic: 'i', On: st.FlattenIntoDest})
	}
	if len(options) > 0 {
		head = append(head, dlg.Item{Kind: dlg.KindSeparator})
	}
	spec.Groups = []dlg.Group{{Columns: [][]dlg.Item{head}}}
	if len(options) > 0 {
		spec.Groups = append(spec.Groups, dlg.Group{Columns: [][]dlg.Item{options}})
	}
	if st.MultiLocation() {
		icon := listIconPad(ctx.IconLead)
		result := scrollList(transferResult, len(st.Entries), TransferListViewportRows(ctx.Layout, st), st.EntriesScroll,
			func(i int) dlg.ListRow {
				e := st.Entries[i]
				return dlg.ListRow{Text: deleteListRowText(transferEntryLabel(e, st.FlattenIntoDest), e.Path), Icon: icon}
			})
		result.Fit = dlg.FitPath
		spec.ScrollbarStyle = ctx.ScrollbarStyle
		spec.Groups = append(spec.Groups, dlg.Group{Columns: [][]dlg.Item{{
			{Kind: dlg.KindSeparator, Label: "Result"},
			result,
		}}})
	}
	return spec
}

// Apply folds an activation from dlg.HandleKey into the state: the checkboxes toggle
// (Dereference symlinks does nothing where it is greyed out). The inputs and buttons are the
// caller's to handle.
func (st *TransferDialogState) Apply(ev dlg.Event) {
	if ev.ID == transferResult && ev.Kind == dlg.EventListMove {
		st.EntriesScroll = ev.Offset
		return
	}
	switch ev.ID {
	case TransferPerm:
		st.PreservePermissions = !st.PreservePermissions
	case TransferTimes:
		st.PreserveTimestamps = !st.PreserveTimestamps
	case TransferDeref:
		if !st.DereferenceUnsupported() {
			st.DereferenceSymlinks = !st.DereferenceSymlinks
		}
	case TransferFlatten:
		st.FlattenIntoDest = !st.FlattenIntoDest
	}
}
