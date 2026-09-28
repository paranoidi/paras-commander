package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestGroupSelectDialogHeight(t *testing.T) {
	t.Parallel()

	base := GroupSelectState{PatternMode: panel.GroupPatternShell}
	if got := groupSelectDialogHeight(base, 40); got != 13 {
		t.Fatalf("base height = %d, want 13", got)
	}

	withPreview := base
	withPreview.PreviewShow = true
	withPreview.PreviewFiles = 2
	if got := groupSelectDialogHeight(withPreview, 40); got != 13 {
		t.Fatalf("preview height = %d, want 13", got)
	}

	withMeta := base
	withMeta.MetaColumnCount = 3
	if got := groupSelectDialogHeight(withMeta, 40); got != 14 {
		t.Fatalf("meta height = %d, want 14", got)
	}

	withHint := base
	withHint.Text = "["
	withHint.PatternMode = panel.GroupPatternRegex
	if got := groupSelectDialogHeight(withHint, 40); got != 13 {
		t.Fatalf("hint height = %d, want 13 (reserved row; no grow)", got)
	}

	withHintAndMeta := withHint
	withHintAndMeta.MetaColumnCount = 3
	if got := groupSelectDialogHeight(withHintAndMeta, 40); got != 14 {
		t.Fatalf("hint+meta height = %d, want 14", got)
	}
}

func TestGroupSelectMoveFocusCheckboxGrid(t *testing.T) {
	t.Parallel()
	form := NewDialogLinearForm(8)
	formMeta := NewDialogLinearForm(10)

	t.Run("down from files to case", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFilesOnly, tcell.KeyDown, panel.GroupPatternShell, 0, false)
		if !ok || got != GroupSelectFocusCase {
			t.Fatalf("Down Files: got %d ok=%v want %d", got, ok, GroupSelectFocusCase)
		}
	})

	t.Run("right from files to dirs", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFilesOnly, tcell.KeyRight, panel.GroupPatternShell, 0, false)
		if !ok || got != GroupSelectFocusDirsOnly {
			t.Fatalf("Right Files: got %d ok=%v want %d", got, ok, GroupSelectFocusDirsOnly)
		}
	})

	t.Run("left from dirs to files", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusDirsOnly, tcell.KeyLeft, panel.GroupPatternShell, 0, false)
		if !ok || got != GroupSelectFocusFilesOnly {
			t.Fatalf("Left Dirs: got %d ok=%v want %d", got, ok, GroupSelectFocusFilesOnly)
		}
	})

	t.Run("up from case to files", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusCase, tcell.KeyUp, panel.GroupPatternShell, 0, false)
		if !ok || got != GroupSelectFocusFilesOnly {
			t.Fatalf("Up Case: got %d ok=%v want %d", got, ok, GroupSelectFocusFilesOnly)
		}
	})

	t.Run("down from files skips case in regex mode", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFilesOnly, tcell.KeyDown, panel.GroupPatternRegex, 0, false)
		if !ok || got != form.OKIndex() {
			t.Fatalf("Down Files regex: got %d ok=%v want OK %d", got, ok, form.OKIndex())
		}
	})

	t.Run("down from dirs goes to ok without full path", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusDirsOnly, tcell.KeyDown, panel.GroupPatternShell, 0, false)
		if !ok || got != form.OKIndex() {
			t.Fatalf("Down Dirs: got %d ok=%v want OK %d", got, ok, form.OKIndex())
		}
	})

	t.Run("down from case goes to include meta when present", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusCase, tcell.KeyDown, panel.GroupPatternShell, 1, false)
		if !ok || got != GroupSelectFocusIncludeMeta {
			t.Fatalf("Down Case+meta: got %d ok=%v want %d", got, ok, GroupSelectFocusIncludeMeta)
		}
	})

	t.Run("down from include meta goes to ok", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusIncludeMeta, tcell.KeyDown, panel.GroupPatternShell, 1, false)
		if !ok || got != formMeta.OKIndex() {
			t.Fatalf("Down IncludeMeta: got %d ok=%v want OK %d", got, ok, formMeta.OKIndex())
		}
	})

	t.Run("up from ok goes to include meta when present", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(formMeta.OKIndex(), tcell.KeyUp, panel.GroupPatternShell, 1, false)
		if !ok || got != GroupSelectFocusIncludeMeta {
			t.Fatalf("Up OK+meta: got %d ok=%v want %d", got, ok, GroupSelectFocusIncludeMeta)
		}
	})
}

func TestGroupSelectMoveFocusFullPath(t *testing.T) {
	t.Parallel()
	form := NewDialogLinearForm(8)

	t.Run("down from dirs goes to full path when shown", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusDirsOnly, tcell.KeyDown, panel.GroupPatternShell, 0, true)
		if !ok || got != GroupSelectFocusFullPath {
			t.Fatalf("Down Dirs+fullpath: got %d ok=%v want %d", got, ok, GroupSelectFocusFullPath)
		}
	})

	t.Run("right from case goes to full path when shown", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusCase, tcell.KeyRight, panel.GroupPatternShell, 0, true)
		if !ok || got != GroupSelectFocusFullPath {
			t.Fatalf("Right Case+fullpath: got %d ok=%v want %d", got, ok, GroupSelectFocusFullPath)
		}
	})

	t.Run("right from case does nothing when full path hidden", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusCase, tcell.KeyRight, panel.GroupPatternShell, 0, false)
		if ok || got != GroupSelectFocusCase {
			t.Fatalf("Right Case no-fullpath: got %d ok=%v want no-op at %d", got, ok, GroupSelectFocusCase)
		}
	})

	t.Run("left from full path goes to case", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFullPath, tcell.KeyLeft, panel.GroupPatternShell, 0, true)
		if !ok || got != GroupSelectFocusCase {
			t.Fatalf("Left FullPath: got %d ok=%v want %d", got, ok, GroupSelectFocusCase)
		}
	})

	t.Run("left from full path does nothing when case hidden by regex", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFullPath, tcell.KeyLeft, panel.GroupPatternRegex, 0, true)
		if ok || got != GroupSelectFocusFullPath {
			t.Fatalf("Left FullPath regex: got %d ok=%v want no-op at %d", got, ok, GroupSelectFocusFullPath)
		}
	})

	t.Run("up from full path goes to dirs", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFullPath, tcell.KeyUp, panel.GroupPatternShell, 0, true)
		if !ok || got != GroupSelectFocusDirsOnly {
			t.Fatalf("Up FullPath: got %d ok=%v want %d", got, ok, GroupSelectFocusDirsOnly)
		}
	})

	t.Run("down from full path goes to ok", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFullPath, tcell.KeyDown, panel.GroupPatternShell, 0, true)
		if !ok || got != form.OKIndex() {
			t.Fatalf("Down FullPath: got %d ok=%v want OK %d", got, ok, form.OKIndex())
		}
	})

	t.Run("up from ok goes to full path when shown", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(form.OKIndex(), tcell.KeyUp, panel.GroupPatternShell, 0, true)
		if !ok || got != GroupSelectFocusFullPath {
			t.Fatalf("Up OK+fullpath: got %d ok=%v want %d", got, ok, GroupSelectFocusFullPath)
		}
	})

	t.Run("GroupSelectLastContentFocus returns full path when shown", func(t *testing.T) {
		if got := GroupSelectLastContentFocus(panel.GroupPatternShell, 0, true); got != GroupSelectFocusFullPath {
			t.Fatalf("LastContentFocus = %d, want %d", got, GroupSelectFocusFullPath)
		}
	})
}
