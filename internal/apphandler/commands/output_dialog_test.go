package commands

import (
	"context"
	"runtime"
	"slices"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestRunUserMenuCommandDialogSuccessKeepsStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh not on PATH on Windows")
	}
	h, screen := newOutputDialogHandler(t)
	h.RunUserMenuCommandDialog(context.Background(),
		[]string{"sh", "-c", "echo stdout-line; echo stderr-line >&2"},
		t.TempDir(), "Show", "", "")

	st := pollOutputDialog(t, screen)
	if st.Title != "Show" {
		t.Fatalf("Title = %q, want Show", st.Title)
	}
	want := []string{"stdout-line", "--- stderr ---", "stderr-line"}
	if !slices.Equal(st.Lines, want) {
		t.Fatalf("Lines = %#v, want %#v", st.Lines, want)
	}
}

func TestRunUserMenuCommandDialogFailureKeepsStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh not on PATH on Windows")
	}
	h, screen := newOutputDialogHandler(t)
	h.RunUserMenuCommandDialog(context.Background(),
		[]string{"sh", "-c", "echo stdout-line; echo stderr-line >&2; exit 2"},
		t.TempDir(), "Show", "", "")

	st := pollOutputDialog(t, screen)
	if st.Title != "Show (exit 2)" {
		t.Fatalf("Title = %q, want Show (exit 2)", st.Title)
	}
	want := []string{"stdout-line", "--- stderr ---", "stderr-line"}
	if !slices.Equal(st.Lines, want) {
		t.Fatalf("Lines = %#v, want %#v", st.Lines, want)
	}
}

func newOutputDialogHandler(t *testing.T) (*Handler, tcell.SimulationScreen) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	return &Handler{
		screen: screen,
		model:  &ui.Model{},
	}, screen
}

func pollOutputDialog(t *testing.T, screen tcell.Screen) dialog.CommandOutputDialogState {
	t.Helper()
	ev := screen.PollEvent()
	interrupt, ok := ev.(*tcell.EventInterrupt)
	if !ok {
		t.Fatalf("event = %T, want *tcell.EventInterrupt", ev)
	}
	p, ok := interrupt.Data().(WakePayload)
	if !ok {
		t.Fatalf("interrupt data = %T, want WakePayload", interrupt.Data())
	}
	if p.OpenOutputDialog == nil {
		t.Fatal("OpenOutputDialog is nil")
	}
	return *p.OpenOutputDialog
}
