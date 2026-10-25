package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func TestTransferViewFocusOrder(t *testing.T) {
	st := transferGoldenState(TransferKindCopy, "dest", 4, "typed", "-", "dest")
	spec := TransferView(st, TransferViewContext{Layout: Layout{Width: 80, Height: 40}})
	focus := dlg.ID("")
	var order []dlg.ID
	for i := 0; i < 7; i++ {
		order = append(order, dlg.Focused(spec, focus))
		dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	want := []dlg.ID{TransferDest, TransferPerm, TransferTimes, TransferDeref, TransferFlatten, dlg.OK, dlg.OK}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("focus after %d Down = %q, want %q (order %v)", i, order[i], want[i], order)
		}
	}
}

func TestTransferApplyToggles(t *testing.T) {
	st := TransferDialogState{Kind: TransferKindCopy}
	st.Apply(dlg.Event{ID: TransferPerm})
	st.Apply(dlg.Event{ID: TransferTimes})
	st.Apply(dlg.Event{ID: TransferFlatten})
	if !st.PreservePermissions || !st.PreserveTimestamps || !st.FlattenIntoDest {
		t.Fatalf("toggles not applied: %+v", st)
	}
	st.Apply(dlg.Event{ID: TransferDeref})
	if !st.DereferenceSymlinks {
		t.Fatal("dereference should toggle on a local copy")
	}
	st.SourceIsRemote = true
	st.Apply(dlg.Event{ID: TransferDeref})
	if !st.DereferenceSymlinks {
		t.Fatal("dereference must stay as is where it is greyed out")
	}
}

// The completion scroll uses the width of the input as painted, in both phases.
func TestTransferDestinationTextWidthMatchesPlacedInput(t *testing.T) {
	layout := Layout{Width: 80, Height: 30}
	for _, st := range []TransferDialogState{
		transferGoldenState(TransferKindCopy, "dest", 0, "typed", "-", "dest"),
		transferGoldenState(TransferKindMove, "self", 0, "typed", "-", "dest"),
		transferGoldenState(TransferKindCopy, "dest", 6, "typed", "-", "dest"),
	} {
		id := TransferDest
		if st.Phase == TransferPhaseSelfCopyRename {
			id = TransferNewName
		}
		p, ok := dlg.Place(layout.Width, layout.Height, TransferView(st, TransferViewContext{Layout: layout}))
		in, found := p.Item(id)
		if !ok || !found {
			t.Fatalf("input %q not placed", id)
		}
		if got := TransferDestinationTextWidth(layout, st, "", 0); got != in.Rect.Width {
			t.Errorf("phase %d multi=%v: width %d, want the placed input width %d", st.Phase, st.MultiLocation(), got, in.Rect.Width)
		}
	}
}
