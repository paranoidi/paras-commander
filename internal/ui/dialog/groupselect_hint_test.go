package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestGroupSelectDialogHeight(t *testing.T) {
	t.Parallel()

	base := GroupSelectState{PatternMode: panel.GroupPatternShell}
	if got := groupSelectDialogHeight(base, 40); got != 14 {
		t.Fatalf("base height = %d, want 14", got)
	}

	withPreview := base
	withPreview.PreviewShow = true
	withPreview.PreviewFiles = 2
	if got := groupSelectDialogHeight(withPreview, 40); got != 14 {
		t.Fatalf("preview height = %d, want 14", got)
	}

	withMeta := base
	withMeta.MetaColumnCount = 3
	if got := groupSelectDialogHeight(withMeta, 40); got != 15 {
		t.Fatalf("meta height = %d, want 15", got)
	}

	withHint := base
	withHint.Text = "["
	withHint.PatternMode = panel.GroupPatternRegex
	if got := groupSelectDialogHeight(withHint, 40); got != 14 {
		t.Fatalf("hint height = %d, want 14 (reserved row; no grow)", got)
	}

	withHintAndMeta := withHint
	withHintAndMeta.MetaColumnCount = 3
	if got := groupSelectDialogHeight(withHintAndMeta, 40); got != 15 {
		t.Fatalf("hint+meta height = %d, want 15", got)
	}
}

func TestGroupSelectMoveFocusCheckboxGrid(t *testing.T) {
	t.Parallel()
	formMeta := NewDialogLinearForm(GroupSelectContentCount(1))

	t.Run("down from files to case", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFilesOnly, tcell.KeyDown, 0, false)
		if !ok || got != GroupSelectFocusCase {
			t.Fatalf("Down Files: got %d ok=%v want %d", got, ok, GroupSelectFocusCase)
		}
	})

	t.Run("right from files to dirs", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFilesOnly, tcell.KeyRight, 0, false)
		if !ok || got != GroupSelectFocusDirsOnly {
			t.Fatalf("Right Files: got %d ok=%v want %d", got, ok, GroupSelectFocusDirsOnly)
		}
	})

	t.Run("left from dirs to files", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusDirsOnly, tcell.KeyLeft, 0, false)
		if !ok || got != GroupSelectFocusFilesOnly {
			t.Fatalf("Left Dirs: got %d ok=%v want %d", got, ok, GroupSelectFocusFilesOnly)
		}
	})

	t.Run("up from case to files", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusCase, tcell.KeyUp, 0, false)
		if !ok || got != GroupSelectFocusFilesOnly {
			t.Fatalf("Up Case: got %d ok=%v want %d", got, ok, GroupSelectFocusFilesOnly)
		}
	})

	t.Run("down from dirs falls back to case without full path", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusDirsOnly, tcell.KeyDown, 0, false)
		if !ok || got != GroupSelectFocusCase {
			t.Fatalf("Down Dirs: got %d ok=%v want %d", got, ok, GroupSelectFocusCase)
		}
	})

	t.Run("down from case goes to include meta when present", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusCase, tcell.KeyDown, 1, false)
		if !ok || got != GroupSelectFocusIncludeMeta {
			t.Fatalf("Down Case+meta: got %d ok=%v want %d", got, ok, GroupSelectFocusIncludeMeta)
		}
	})

	t.Run("down from include meta goes to ok", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusIncludeMeta, tcell.KeyDown, 1, false)
		if !ok || got != formMeta.OKIndex() {
			t.Fatalf("Down IncludeMeta: got %d ok=%v want OK %d", got, ok, formMeta.OKIndex())
		}
	})

	t.Run("up from ok goes to include meta when present", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(formMeta.OKIndex(), tcell.KeyUp, 1, false)
		if !ok || got != GroupSelectFocusIncludeMeta {
			t.Fatalf("Up OK+meta: got %d ok=%v want %d", got, ok, GroupSelectFocusIncludeMeta)
		}
	})
}

func TestGroupSelectMoveFocusFullPath(t *testing.T) {
	t.Parallel()
	form := NewDialogLinearForm(GroupSelectContentCount(0))

	t.Run("down from dirs goes to full path when shown", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusDirsOnly, tcell.KeyDown, 0, true)
		if !ok || got != GroupSelectFocusFullPath {
			t.Fatalf("Down Dirs+fullpath: got %d ok=%v want %d", got, ok, GroupSelectFocusFullPath)
		}
	})

	t.Run("right from case goes to full path when shown", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusCase, tcell.KeyRight, 0, true)
		if !ok || got != GroupSelectFocusFullPath {
			t.Fatalf("Right Case+fullpath: got %d ok=%v want %d", got, ok, GroupSelectFocusFullPath)
		}
	})

	t.Run("right from case does nothing when full path hidden", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusCase, tcell.KeyRight, 0, false)
		if ok || got != GroupSelectFocusCase {
			t.Fatalf("Right Case no-fullpath: got %d ok=%v want no-op at %d", got, ok, GroupSelectFocusCase)
		}
	})

	t.Run("left from full path goes to case", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFullPath, tcell.KeyLeft, 0, true)
		if !ok || got != GroupSelectFocusCase {
			t.Fatalf("Left FullPath: got %d ok=%v want %d", got, ok, GroupSelectFocusCase)
		}
	})

	t.Run("up from full path goes to dirs", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFullPath, tcell.KeyUp, 0, true)
		if !ok || got != GroupSelectFocusDirsOnly {
			t.Fatalf("Up FullPath: got %d ok=%v want %d", got, ok, GroupSelectFocusDirsOnly)
		}
	})

	t.Run("down from full path goes to ok", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(GroupSelectFocusFullPath, tcell.KeyDown, 0, true)
		if !ok || got != form.OKIndex() {
			t.Fatalf("Down FullPath: got %d ok=%v want OK %d", got, ok, form.OKIndex())
		}
	})

	t.Run("up from ok goes to full path when shown", func(t *testing.T) {
		got, ok := GroupSelectMoveFocus(form.OKIndex(), tcell.KeyUp, 0, true)
		if !ok || got != GroupSelectFocusFullPath {
			t.Fatalf("Up OK+fullpath: got %d ok=%v want %d", got, ok, GroupSelectFocusFullPath)
		}
	})

	t.Run("GroupSelectLastContentFocus returns full path when shown", func(t *testing.T) {
		if got := GroupSelectLastContentFocus(0, true); got != GroupSelectFocusFullPath {
			t.Fatalf("LastContentFocus = %d, want %d", got, GroupSelectFocusFullPath)
		}
	})
}

func TestGroupSelectMoveFocusTypes(t *testing.T) {
	t.Parallel()

	first := GroupSelectFocusTypeFirst
	cases := []struct {
		name  string
		focus int
		key   tcell.Key
		want  int
	}{
		{"right from shell radio enters types", GroupSelectFocusShellRadio, tcell.KeyRight, first},
		{"down within first type column", first, tcell.KeyDown, first + 1},
		{"right to second type column", first, tcell.KeyRight, first + 3},
		{"down from pattern is files only", GroupSelectFocusPattern, tcell.KeyDown, GroupSelectFocusFilesOnly},
		{"up from pattern is simple radio", GroupSelectFocusPattern, tcell.KeyUp, GroupSelectFocusSimpleRadio},
		{"tab from radios is first type", GroupSelectFocusRegexRadio, tcell.KeyTab, first},
		{"tab from types is pattern", first + 2, tcell.KeyTab, GroupSelectFocusPattern},
		{"backtab from pattern is first type", GroupSelectFocusPattern, tcell.KeyBacktab, first},
	}
	for _, tc := range cases {
		got, ok := GroupSelectMoveFocus(tc.focus, tc.key, 0, false)
		if !ok || got != tc.want {
			t.Errorf("%s: got %d ok=%v want %d", tc.name, got, ok, tc.want)
		}
	}
}
