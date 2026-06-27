package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestGroupSelectMoveFocusCheckboxGrid(t *testing.T) {
	t.Parallel()
	form := NewDialogLinearForm(7)
	formMeta := NewDialogLinearForm(8)

	t.Run("down from files to case", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFilesOnly, tcell.KeyDown, panel.GroupPatternShell, 0)
		if !ok || got != GroupSelectFocusCase {
			t.Fatalf("Down Files: got %d ok=%v want %d", got, ok, GroupSelectFocusCase)
		}
	})

	t.Run("right from files to dirs", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFilesOnly, tcell.KeyRight, panel.GroupPatternShell, 0)
		if !ok || got != GroupSelectFocusDirsOnly {
			t.Fatalf("Right Files: got %d ok=%v want %d", got, ok, GroupSelectFocusDirsOnly)
		}
	})

	t.Run("left from dirs to files", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusDirsOnly, tcell.KeyLeft, panel.GroupPatternShell, 0)
		if !ok || got != GroupSelectFocusFilesOnly {
			t.Fatalf("Left Dirs: got %d ok=%v want %d", got, ok, GroupSelectFocusFilesOnly)
		}
	})

	t.Run("up from case to files", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusCase, tcell.KeyUp, panel.GroupPatternShell, 0)
		if !ok || got != GroupSelectFocusFilesOnly {
			t.Fatalf("Up Case: got %d ok=%v want %d", got, ok, GroupSelectFocusFilesOnly)
		}
	})

	t.Run("down from files skips case in regex mode", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFilesOnly, tcell.KeyDown, panel.GroupPatternRegex, 0)
		if !ok || got != form.OKIndex() {
			t.Fatalf("Down Files regex: got %d ok=%v want OK %d", got, ok, form.OKIndex())
		}
	})

	t.Run("down from dirs goes to ok", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusDirsOnly, tcell.KeyDown, panel.GroupPatternShell, 0)
		if !ok || got != form.OKIndex() {
			t.Fatalf("Down Dirs: got %d ok=%v want OK %d", got, ok, form.OKIndex())
		}
	})

	t.Run("down from case goes to include meta when present", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusCase, tcell.KeyDown, panel.GroupPatternShell, 1)
		if !ok || got != GroupSelectFocusIncludeMeta {
			t.Fatalf("Down Case+meta: got %d ok=%v want %d", got, ok, GroupSelectFocusIncludeMeta)
		}
	})

	t.Run("down from include meta goes to ok", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusIncludeMeta, tcell.KeyDown, panel.GroupPatternShell, 1)
		if !ok || got != formMeta.OKIndex() {
			t.Fatalf("Down IncludeMeta: got %d ok=%v want OK %d", got, ok, formMeta.OKIndex())
		}
	})

	t.Run("up from ok goes to include meta when present", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(formMeta.OKIndex(), tcell.KeyUp, panel.GroupPatternShell, 1)
		if !ok || got != GroupSelectFocusIncludeMeta {
			t.Fatalf("Up OK+meta: got %d ok=%v want %d", got, ok, GroupSelectFocusIncludeMeta)
		}
	})
}
