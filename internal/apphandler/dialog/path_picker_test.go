package dialog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestPathPickerBookmarksLoadOffEventLoop(t *testing.T) {
	h, host := newBookmarkTestHandler(t)
	release := stallBookmarkIO(t)
	harbor := filepath.Join(host.active.PathString(), "harbor")

	done := make(chan struct{})
	go func() {
		h.OpenPathPickerForTransfer(pathPickerListBookmarks)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OpenPathPickerForTransfer blocked on delayed filesystem")
	}

	if !h.model.PathPicker.Open {
		t.Fatal("picker should open before the marks file becomes readable")
	}
	if len(h.model.PathPicker.Items) != 0 {
		t.Fatalf("items = %d, want 0 until the load completes", len(h.model.PathPicker.Items))
	}

	close(release)
	p := waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
	h.ApplyBookmarkIO(p)
	if len(h.model.PathPicker.Items) != 1 || h.model.PathPicker.Items[0].Path != harbor {
		t.Fatalf("items after apply = %+v, want [%s]", h.model.PathPicker.Items, harbor)
	}
}

func TestPathPickerBookmarkLoadStaleCompletionDiscarded(t *testing.T) {
	h, _ := newBookmarkTestHandler(t)
	release := stallBookmarkIO(t)

	h.OpenPathPickerForTransfer(pathPickerListBookmarks)
	if !h.model.PathPicker.Open {
		t.Fatal("expected picker open")
	}
	staleGen := h.pathPickerMissingGen
	h.ClosePathPicker()
	close(release)

	p := waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
	if p.Gen != staleGen {
		t.Fatalf("payload gen = %d, want stalled open gen %d", p.Gen, staleGen)
	}
	h.ApplyBookmarkIO(p)
	if h.model.PathPicker.Open {
		t.Fatal("stale load must not reopen a closed picker")
	}
	if len(h.model.PathPicker.Items) != 0 {
		t.Fatalf("stale load applied %d items, want discarded", len(h.model.PathPicker.Items))
	}
}

func TestPathPickerBookmarksNotFilledByLoadAllOnCallerGoroutine(t *testing.T) {
	h, _ := newBookmarkTestHandler(t)
	release := stallBookmarkIO(t)

	h.OpenPathPickerForTransfer(pathPickerListBookmarks)
	if !h.model.PathPicker.Open {
		t.Fatal("picker should open without waiting for LoadAll")
	}
	if len(h.model.PathPicker.Items) != 0 {
		t.Fatalf("items = %d, want 0; LoadAll must not fill the model on the caller goroutine", len(h.model.PathPicker.Items))
	}

	close(release)
	_ = waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
}

type allPickerHost struct {
	*bookmarkTestHost
	inactive *panel.State
}

func (f allPickerHost) InactivePanel() *panel.State { return f.inactive }

func TestPathPickerAllOpensFromStarKeys(t *testing.T) {
	for _, r := range []rune{'*', '|'} {
		t.Run(string(r), func(t *testing.T) {
			h, host := newBookmarkTestHandler(t)
			root := host.active.PathString()
			orchard := filepath.Join(root, "orchard")
			meadow := filepath.Join(root, "meadow")
			harbor := filepath.Join(root, "harbor") // bookmarked and in history
			for _, d := range []string{orchard, meadow} {
				if err := os.Mkdir(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			host.active.History = []string{meadow, harbor}
			h.host = allPickerHost{bookmarkTestHost: host, inactive: &panel.State{Path: pathloc.FileMust(root)}}
			bundle, err := keymap.DefaultBundle()
			if err != nil {
				t.Fatal(err)
			}
			h.keysGlobal = bundle.Global
			h.keysDialogInput = bundle.DialogInput
			h.model.PinnedItems = []ui.PinnedItem{{Path: orchard, IsDir: true}}
			h.model.TransferDialog = uidialog.TransferDialogState{
				Open:        true,
				Phase:       uidialog.TransferPhaseDestination,
				Destination: uidialog.FileDialogField{Value: "x"},
			}

			h.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
			st := &h.model.PathPicker
			if !st.Open || st.Purpose != uidialog.PathPickerPurposeApplyTransferDestination {
				t.Fatalf("picker open=%v purpose=%v", st.Open, st.Purpose)
			}
			if got := h.model.TransferDialog.Destination.Value; got != "x" {
				t.Fatalf("destination = %q, rune must not be inserted", got)
			}

			sources := map[string]int{}
			paths := map[string]int{}
			for _, it := range st.Items {
				sources[it.Source]++
				paths[it.Path]++
			}
			for _, src := range []string{"fzf-marks", "pinned", "history"} {
				if sources[src] == 0 {
					t.Errorf("no %q item in %+v", src, st.Items)
				}
			}
			if paths[harbor] != 1 {
				t.Errorf("harbor appears %d times, want 1: %+v", paths[harbor], st.Items)
			}

			h.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
			if h.model.PathPicker.Open {
				t.Fatal("picker should close on Enter")
			}
			if got := h.model.TransferDialog.Destination.Value; got == "x" || got == "" {
				t.Fatalf("destination = %q, want selected path", got)
			}
		})
	}
}
