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
	if got := height(base); got != 14 {
		t.Fatalf("base height = %d, want 14", got)
	}

	withPreview := base
	withPreview.PreviewShow = true
	withPreview.PreviewFiles = 2
	if got := height(withPreview); got != 14 {
		t.Fatalf("preview height = %d, want 14", got)
	}

	withMeta := base
	withMeta.MetaColumnCount = 3
	if got := height(withMeta); got != 15 {
		t.Fatalf("meta height = %d, want 15", got)
	}

	withHint := base
	withHint.Text = "["
	withHint.PatternMode = panel.GroupPatternRegex
	if got := height(withHint); got != 14 {
		t.Fatalf("hint height = %d, want 14 (reserved row; no grow)", got)
	}

	withHintAndMeta := withHint
	withHintAndMeta.MetaColumnCount = 3
	if got := height(withHintAndMeta); got != 15 {
		t.Fatalf("hint+meta height = %d, want 15", got)
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
		{"down from files to case", plain, PatternScopeFiles, tcell.KeyDown, PatternCase},
		{"right from files to dirs", plain, PatternScopeFiles, tcell.KeyRight, PatternScopeDirs},
		{"left from dirs to files", plain, PatternScopeDirs, tcell.KeyLeft, PatternScopeFiles},
		{"up from case to files", plain, PatternCase, tcell.KeyUp, PatternScopeFiles},
		{"down from dirs falls back to case without full path", plain, PatternScopeDirs, tcell.KeyDown, PatternCase},
		{"right from case does nothing when full path hidden", plain, PatternCase, tcell.KeyRight, PatternCase},
		{"down from case goes to include meta when present", meta, PatternCase, tcell.KeyDown, PatternIncludeMeta},
		{"down from include meta goes to ok", meta, PatternIncludeMeta, tcell.KeyDown, dlg.OK},
		{"up from ok goes to only meta, drawn above it", meta, dlg.OK, tcell.KeyUp, PatternOnlyMeta},
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
		{"down from dirs goes to full path when shown", PatternScopeDirs, tcell.KeyDown, PatternFullPath},
		{"right from case goes to full path when shown", PatternCase, tcell.KeyRight, PatternFullPath},
		{"left from full path goes to case", PatternFullPath, tcell.KeyLeft, PatternCase},
		{"up from full path goes to dirs", PatternFullPath, tcell.KeyUp, PatternScopeDirs},
		{"down from full path goes to ok", PatternFullPath, tcell.KeyDown, dlg.OK},
		{"up from ok goes to the full path checkbox drawn above it", dlg.OK, tcell.KeyUp, PatternFullPath},
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
		{"right from simple radio is shell radio", PatternSimpleRadio, tcell.KeyRight, PatternShellRadio},
		{"down from simple radio enters types", PatternSimpleRadio, tcell.KeyDown, first},
		{"down within first type column", first, tcell.KeyDown, PatternTypeID(3)},
		{"right to second type column", first, tcell.KeyRight, PatternTypeID(1)},
		{"down from pattern is files only", PatternInput, tcell.KeyDown, PatternScopeFiles},
		{"up from pattern is the second type row", PatternInput, tcell.KeyUp, PatternTypeID(3)},
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

func TestPatternHintSimpleAutoShell(t *testing.T) {
	t.Parallel()
	if got := patternHintText("[", panel.GroupPatternSimple, false, ""); got != "" {
		t.Fatalf("plain simple pattern hint = %q, want none", got)
	}
	if got := patternHintText("*[", panel.GroupPatternSimple, false, ""); got == "" {
		t.Fatal("invalid auto-shell pattern should show a hint")
	}
}
