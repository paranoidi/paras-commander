package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func groupSelectTestView(st GroupSelectState) dlg.Spec {
	return GroupSelectView(st, Layout{Width: 80, Height: 40}, theme.Default())
}

func TestGroupSelectDialogHeight(t *testing.T) {
	t.Parallel()
	height := func(st GroupSelectState) int {
		p, ok := dlg.Place(80, 40, groupSelectTestView(st))
		if !ok {
			t.Fatal("dialog does not fit")
		}
		return p.Rect.Height
	}

	base := GroupSelectState{PatternMode: panel.GroupPatternShell}
	if got := height(base); got != 13 {
		t.Fatalf("base height = %d, want 13", got)
	}

	withPreview := base
	withPreview.PreviewShow = true
	withPreview.PreviewFiles = 2
	if got := height(withPreview); got != 13 {
		t.Fatalf("preview height = %d, want 13", got)
	}

	withMeta := base
	withMeta.MetaColumnCount = 3
	if got := height(withMeta); got != 14 {
		t.Fatalf("meta height = %d, want 14", got)
	}

	withHint := base
	withHint.Text = "["
	withHint.PatternMode = panel.GroupPatternRegex
	if got := height(withHint); got != 13 {
		t.Fatalf("hint height = %d, want 13 (reserved row; no grow)", got)
	}

	withHintAndMeta := withHint
	withHintAndMeta.MetaColumnCount = 3
	if got := height(withHintAndMeta); got != 14 {
		t.Fatalf("hint+meta height = %d, want 14", got)
	}
}

func TestGroupSelectFocusCheckboxGrid(t *testing.T) {
	t.Parallel()
	plain := groupSelectTestView(GroupSelectState{})
	meta := groupSelectTestView(GroupSelectState{MetaColumnCount: 1})

	cases := []struct {
		name string
		spec dlg.Spec
		from dlg.ID
		key  tcell.Key
		want dlg.ID
	}{
		{"down from files to case", plain, PatternFilesOnly, tcell.KeyDown, PatternCase},
		{"right from files to dirs", plain, PatternFilesOnly, tcell.KeyRight, PatternDirsOnly},
		{"left from dirs to files", plain, PatternDirsOnly, tcell.KeyLeft, PatternFilesOnly},
		{"up from case to files", plain, PatternCase, tcell.KeyUp, PatternFilesOnly},
		{"down from dirs falls back to case without full path", plain, PatternDirsOnly, tcell.KeyDown, PatternCase},
		{"right from case does nothing when full path hidden", plain, PatternCase, tcell.KeyRight, PatternCase},
		{"down from case goes to include meta when present", meta, PatternCase, tcell.KeyDown, PatternIncludeMeta},
		{"down from include meta goes to ok", meta, PatternIncludeMeta, tcell.KeyDown, dlg.OK},
		{"up from ok goes to include meta when present", meta, dlg.OK, tcell.KeyUp, PatternIncludeMeta},
	}
	for _, tc := range cases {
		if got := patternStep(tc.spec, tc.from, tc.key); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestGroupSelectFocusFullPath(t *testing.T) {
	t.Parallel()
	find := groupSelectTestView(GroupSelectState{Context: "find"})

	cases := []struct {
		name string
		from dlg.ID
		key  tcell.Key
		want dlg.ID
	}{
		{"down from dirs goes to full path when shown", PatternDirsOnly, tcell.KeyDown, PatternFullPath},
		{"right from case goes to full path when shown", PatternCase, tcell.KeyRight, PatternFullPath},
		{"left from full path goes to case", PatternFullPath, tcell.KeyLeft, PatternCase},
		{"up from full path goes to dirs", PatternFullPath, tcell.KeyUp, PatternDirsOnly},
		{"down from full path goes to ok", PatternFullPath, tcell.KeyDown, dlg.OK},
		{"up from ok goes to full path when shown", dlg.OK, tcell.KeyUp, PatternFullPath},
		{"up from cancel goes to full path when shown", dlg.Cancel, tcell.KeyUp, PatternFullPath},
	}
	for _, tc := range cases {
		if got := patternStep(find, tc.from, tc.key); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestGroupSelectFocusTypes(t *testing.T) {
	t.Parallel()
	spec := groupSelectTestView(GroupSelectState{})
	first := PatternTypeID(0)
	cases := []struct {
		name string
		from dlg.ID
		key  tcell.Key
		want dlg.ID
	}{
		{"right from shell radio enters types", PatternShellRadio, tcell.KeyRight, first},
		{"down within first type column", first, tcell.KeyDown, PatternTypeID(1)},
		{"right to second type column", first, tcell.KeyRight, PatternTypeID(3)},
		{"down from pattern is files only", PatternInput, tcell.KeyDown, PatternFilesOnly},
		{"up from pattern is simple radio", PatternInput, tcell.KeyUp, PatternSimpleRadio},
		{"tab from radios is first type", PatternRegexRadio, tcell.KeyTab, first},
		{"tab from types is pattern", PatternTypeID(2), tcell.KeyTab, PatternInput},
		{"backtab from pattern is first type", PatternInput, tcell.KeyBacktab, first},
	}
	for _, tc := range cases {
		if got := patternStep(spec, tc.from, tc.key); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}
