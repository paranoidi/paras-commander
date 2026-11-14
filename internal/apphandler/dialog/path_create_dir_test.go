package dialog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func typeDestination(h *Handler, value string) *uidialog.FileDialogField {
	h.model.TransferDialog = uidialog.TransferDialogState{
		Open:        true,
		Kind:        uidialog.TransferKindCopy,
		Phase:       uidialog.TransferPhaseDestination,
		Destination: uidialog.FileDialogField{Value: value, Cursor: len([]rune(value))},
	}
	f := &h.model.TransferDialog.Destination
	h.SyncPathFieldCompletion(f, h.TransferDestinationTextWidth())
	return f
}

func TestDestinationOffersCreateDirAfterSeparator(t *testing.T) {
	t.Parallel()
	h := newPathCompletionHandler(t, 120, 40)
	root := t.TempDir()

	f := typeDestination(h, filepath.Join(root, "lantern", "orchard"))
	if len(f.Completion.Items) != 0 || f.Completion.Open {
		t.Fatalf("no create row while typing a segment, got %+v", f.Completion.Items)
	}

	f = typeDestination(h, filepath.Join(root, "lantern", "orchard")+"/")
	c := f.Completion
	if !c.Open || len(c.Items) != 1 || !c.Items[0].Create || c.Items[0].Name != uidialog.PathCompletionCreateDirLabel {
		t.Fatalf("want open create row, got open=%v items=%+v", c.Open, c.Items)
	}
}

func TestDestinationCreateDirRowCreatesDirectory(t *testing.T) {
	t.Parallel()
	h := newPathCompletionHandler(t, 120, 40)
	root := t.TempDir()
	target := filepath.Join(root, "meadow", "thicket")
	value := target + "/"
	f := typeDestination(h, value)

	armed := false
	if !h.DestFieldTryCompletionKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), f, true, func() { armed = true }) {
		t.Fatal("Enter on the create row should be handled")
	}
	if fi, err := os.Stat(target); err != nil || !fi.IsDir() {
		t.Fatalf("directory not created: %v", err)
	}
	if f.Value != value {
		t.Fatalf("value changed to %q", f.Value)
	}
	if !armed {
		t.Fatal("validation should be re-armed after creating")
	}
	if len(f.Completion.Items) != 0 || f.Completion.Open {
		t.Fatalf("create row should be gone, got %+v", f.Completion.Items)
	}
	host := h.host.(*identityTestHost)
	if len(host.messages) == 0 || !strings.Contains(host.messages[len(host.messages)-1], target) {
		t.Fatalf("want created message naming %s, got %v", target, host.messages)
	}
}

func TestDestinationCreateDirEscClosesWithoutCreating(t *testing.T) {
	t.Parallel()
	h := newPathCompletionHandler(t, 120, 40)
	root := t.TempDir()
	target := filepath.Join(root, "harbor")
	f := typeDestination(h, target+"/")

	if !h.DestFieldTryCompletionKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone), f, true, nil) {
		t.Fatal("Esc should close the dropdown")
	}
	if f.Completion.Open {
		t.Fatal("dropdown still open after Esc")
	}
	h.SyncPathFieldCompletion(f, h.TransferDestinationTextWidth())
	if f.Completion.Open {
		t.Fatal("a same-value resync should keep the dropdown closed")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("directory should not exist: %v", err)
	}
}
