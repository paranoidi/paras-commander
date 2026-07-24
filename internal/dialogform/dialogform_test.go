package dialogform

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestHandleKeyEnterAppliesOrCancels(t *testing.T) {
	form := dialog.NewDialogLinearForm(1).WithSegments(0, 1) // content(0) | buttons(1=OK,2=Cancel)
	focus := form.OKIndex()
	applied, cancelled := false, false
	h := Handlers{
		Focus:    &focus,
		OnApply:  func() { applied = true },
		OnCancel: func() { cancelled = true },
	}
	if !HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), form, h) {
		t.Fatal("expected Enter to be consumed")
	}
	if !applied || cancelled {
		t.Fatalf("Enter on OK: applied=%v cancelled=%v, want true false", applied, cancelled)
	}

	focus = form.CancelIndex()
	applied, cancelled = false, false
	if !HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), form, h) {
		t.Fatal("expected Enter to be consumed")
	}
	if applied || !cancelled {
		t.Fatalf("Enter on Cancel: applied=%v cancelled=%v, want false true", applied, cancelled)
	}
}

func TestHandleKeyEscCancels(t *testing.T) {
	form := dialog.NewDialogLinearForm(1).WithSegments(0, 1)
	focus := 0
	cancelled := false
	h := Handlers{
		Focus:    &focus,
		OnApply:  func() {},
		OnCancel: func() { cancelled = true },
	}
	if !HandleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone), form, h) {
		t.Fatal("expected Esc to be consumed")
	}
	if !cancelled {
		t.Fatal("expected Esc to cancel")
	}
}

func TestHandleKeyPlainOKCancelRunes(t *testing.T) {
	form := dialog.NewDialogLinearForm(1).WithSegments(0, 1)
	focus := 0
	applied, cancelled := false, false
	h := Handlers{
		Focus:              &focus,
		OnApply:            func() { applied = true },
		OnCancel:           func() { cancelled = true },
		AllowPlainOKCancel: true,
	}
	if !HandleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone), form, h) {
		t.Fatal("expected 'o' to be consumed")
	}
	if !applied {
		t.Fatal("expected 'o' to apply")
	}
	if !HandleKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone), form, h) {
		t.Fatal("expected 'c' to be consumed")
	}
	if !cancelled {
		t.Fatal("expected 'c' to cancel")
	}
}

func TestHandleKeySpaceCallsOnSpace(t *testing.T) {
	form := dialog.NewDialogLinearForm(1).WithSegments(0, 1)
	focus := 0
	gotFocus := -1
	h := Handlers{
		Focus:    &focus,
		OnApply:  func() {},
		OnCancel: func() {},
		OnSpace: func(f int) bool {
			gotFocus = f
			return true
		},
	}
	if !HandleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone), form, h) {
		t.Fatal("expected space to be consumed")
	}
	if gotFocus != 0 {
		t.Fatalf("OnSpace got focus %d, want 0", gotFocus)
	}
}
