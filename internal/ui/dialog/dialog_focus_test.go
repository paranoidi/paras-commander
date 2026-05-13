package dialog

import "testing"

func TestDialogLinearForm(t *testing.T) {
	t.Parallel()
	form := NewDialogLinearForm(3) // copy dialog layout
	if g, w := form.TotalFocus(), 5; g != w {
		t.Fatalf("TotalFocus: got %d want %d", g, w)
	}
	if g, w := form.Down(2), 3; g != w {
		t.Fatalf("Down last content -> OK: got %d want %d", g, w)
	}
	if g, w := form.Down(3), 3; g != w {
		t.Fatalf("Down on OK: got %d want %d", g, w)
	}
	if g, w := form.Right(3), 4; g != w {
		t.Fatalf("Right OK->Cancel: got %d want %d", g, w)
	}
	if g, w := form.Up(4), 2; g != w {
		t.Fatalf("Up Cancel->last content: got %d want %d", g, w)
	}
	if g, w := form.Up(0), 0; g != w {
		t.Fatalf("Up first content: got %d want %d", g, w)
	}
}

func TestDialogTrailingButtonsFormTwoButtons(t *testing.T) {
	t.Parallel()
	form := NewDialogTrailingButtonsForm(3, 2)
	if g, w := form.TotalFocus(), 5; g != w {
		t.Fatalf("TotalFocus: got %d want %d", g, w)
	}
	if g, w := form.MiddleButtonIndex(), -1; g != w {
		t.Fatalf("MiddleButtonIndex 2-button: got %d want %d", g, w)
	}
	if g, w := form.Down(2), 3; g != w {
		t.Fatalf("Down last content -> OK: got %d want %d", g, w)
	}
	if g, w := form.Left(4), 3; g != w {
		t.Fatalf("Left Cancel->OK: got %d want %d", g, w)
	}
}

func TestDialogTrailingButtonsFormThreeButtons(t *testing.T) {
	t.Parallel()
	form := NewDialogTrailingButtonsForm(3, 3)
	if g, w := form.TotalFocus(), 6; g != w {
		t.Fatalf("TotalFocus: got %d want %d", g, w)
	}
	if g, w := form.MiddleButtonIndex(), 4; g != w {
		t.Fatalf("MiddleButtonIndex: got %d want %d", g, w)
	}
	if g, w := form.Right(form.OKIndex()), form.MiddleButtonIndex(); g != w {
		t.Fatalf("Right OK->middle: got %d want %d", g, w)
	}
	if g, w := form.Right(form.MiddleButtonIndex()), form.CancelIndex(); g != w {
		t.Fatalf("Right middle->Cancel: got %d want %d", g, w)
	}
	if g, w := form.Left(form.CancelIndex()), form.MiddleButtonIndex(); g != w {
		t.Fatalf("Left Cancel->middle: got %d want %d", g, w)
	}
}

func TestTransferDialogLinearForm(t *testing.T) {
	t.Parallel()
	form := NewTransferDialogLinearForm(3)
	if g, w := form.TotalFocus(), 6; g != w {
		t.Fatalf("TotalFocus: got %d want %d", g, w)
	}
	if g, w := form.AddPausedIndex(), 4; g != w {
		t.Fatalf("AddPausedIndex: got %d want %d", g, w)
	}
	if g, w := form.Right(form.OKIndex()), form.AddPausedIndex(); g != w {
		t.Fatalf("Right OK->Add paused: got %d want %d", g, w)
	}
	if g, w := form.Right(form.AddPausedIndex()), form.CancelIndex(); g != w {
		t.Fatalf("Right Add paused->Cancel: got %d want %d", g, w)
	}
	if g, w := form.Left(form.CancelIndex()), form.AddPausedIndex(); g != w {
		t.Fatalf("Left Cancel->Add paused: got %d want %d", g, w)
	}
	if g, w := form.Down(2), 3; g != w {
		t.Fatalf("Down last content -> OK: got %d want %d", g, w)
	}
	if g, w := form.Up(form.CancelIndex()), 2; g != w {
		t.Fatalf("Up Cancel->last content: got %d want %d", g, w)
	}
}

func TestDialogPairLeftRight(t *testing.T) {
	t.Parallel()
	if g, w := DialogPairLeftRight(0, false), 0; g != w {
		t.Fatalf("Left from first: got %d want %d", g, w)
	}
	if g, w := DialogPairLeftRight(0, true), 1; g != w {
		t.Fatalf("Right from first: got %d want %d", g, w)
	}
	if g, w := DialogPairLeftRight(1, true), 1; g != w {
		t.Fatalf("Right from second: got %d want %d", g, w)
	}
	if g, w := DialogPairLeftRight(1, false), 0; g != w {
		t.Fatalf("Left from second: got %d want %d", g, w)
	}
}
