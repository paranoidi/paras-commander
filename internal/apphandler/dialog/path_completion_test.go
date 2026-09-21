package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func newPathCompletionHandler(t *testing.T, termW, termH int) *Handler {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(termW, termH)

	model := &ui.Model{}
	host := &identityTestHost{model: model, cfg: config.Default()}
	return New(Deps{Host: host, Screen: screen, Model: model})
}

func multiLocationWideTransfer(dest string) uidialog.TransferDialogState {
	root := "/home/user/" + strings.Repeat("meadow-", 10) + "thicket"
	entry := strings.Repeat("harborlantern", 8) + ".txt"
	return uidialog.TransferDialogState{
		Open:        true,
		Kind:        uidialog.TransferKindCopy,
		Phase:       uidialog.TransferPhaseDestination,
		CommonRoot:  root,
		Destination: uidialog.FileDialogField{Value: dest, Cursor: len([]rune(dest))},
		Entries: []uidialog.DeleteListEntry{
			{Name: entry, Path: root + "/" + entry, Type: localfs.EntryFile},
		},
	}
}

func TestTransferDestinationTextWidthUsesPaintedMultiLocationWidth(t *testing.T) {
	t.Parallel()
	const termW, termH = 120, 40
	h := newPathCompletionHandler(t, termW, termH)

	preferredText := uidialog.PreferredFormDialogWidth - 4 - 2
	dest := "/home/user/" + strings.Repeat("x", preferredText)
	h.model.TransferDialog = multiLocationWideTransfer(dest)

	got := h.TransferDestinationTextWidth()
	layout := h.host.LayoutForTerminalSize(termW, termH)
	want := uidialog.TransferDestinationTextWidth(
		layout,
		h.model.TransferDialog,
		h.model.UserHomeDir,
		ui.DialogListIconLeadingWidth(h.model.UseNerdfontIcons),
	)
	if got != want {
		t.Fatalf("TransferDestinationTextWidth() = %d, want painted %d", got, want)
	}
	if got <= preferredText {
		t.Fatalf("TransferDestinationTextWidth() = %d, want greater than preferred %d", got, preferredText)
	}

	destLen := len([]rune(dest))
	_, prefScroll := uidialog.EnsurePathInputScroll(destLen, destLen, 0, preferredText, 0)
	if prefScroll == 0 {
		t.Fatal("preferred width unexpectedly fits dest (test setup)")
	}

	h.SyncPathFieldCompletion(&h.model.TransferDialog.Destination, got)
	if h.model.TransferDialog.Destination.Scroll != 0 {
		t.Fatalf("scroll = %d, want 0 when dest fits painted multi-location width", h.model.TransferDialog.Destination.Scroll)
	}
}

func TestTransferDestinationTextWidthPlainUsesPreferredWidth(t *testing.T) {
	t.Parallel()
	const termW, termH = 120, 24
	h := newPathCompletionHandler(t, termW, termH)
	h.model.TransferDialog = uidialog.TransferDialogState{
		Open:  true,
		Kind:  uidialog.TransferKindCopy,
		Phase: uidialog.TransferPhaseDestination,
	}

	got := h.TransferDestinationTextWidth()
	want := uidialog.PreferredFormDialogWidth - 4 - 2
	if got != want {
		t.Fatalf("plain TransferDestinationTextWidth() = %d, want %d", got, want)
	}
}
