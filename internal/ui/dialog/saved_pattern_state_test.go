package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/filterpattern"
	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestFilterSavedPatternRoundTrip(t *testing.T) {
	st := FilterDialogState{Text: "^otter", PatternMode: panel.GroupPatternRegex, FilesOnly: true, CaseSensitive: true,
		Invert: true, MetaColumnCount: 2, IncludeMetaColumns: true, OnlyMetaColumns: true}
	st.Types[FilterTypeAudio], st.Types[FilterTypeDocuments] = true, true
	p := st.SavedPattern("walrus", "harbor")
	want := filterpattern.Pattern{Name: "walrus", Description: "harbor", Pattern: "^otter", Mode: filterpattern.ModeRegex,
		Types: []string{"audio", "documents"}, FilesOnly: true, CaseSensitive: true, Invert: true, IncludeMeta: true, OnlyMeta: true}
	if !p.Equal(want) {
		t.Fatalf("SavedPattern = %+v, want %+v", p, want)
	}
	loaded := FilterDialogState{MetaColumnCount: 1, PrefillPending: true}
	loaded.LoadPattern(p)
	if loaded.Text != st.Text || loaded.TextCursor != 6 || loaded.PrefillPending || loaded.PatternMode != st.PatternMode ||
		loaded.Types != st.Types || !loaded.FilesOnly || !loaded.CaseSensitive || !loaded.Invert ||
		!loaded.IncludeMetaColumns || !loaded.OnlyMetaColumns {
		t.Fatalf("LoadPattern = %+v", loaded)
	}
}

func TestFilterSavedPatternWithoutMetaStoresDefault(t *testing.T) {
	p := FilterDialogState{Text: "x"}.SavedPattern("", "")
	if !p.IncludeMeta || p.OnlyMeta {
		t.Fatalf("no meta columns: IncludeMeta %v OnlyMeta %v, want the open default (on, off)", p.IncludeMeta, p.OnlyMeta)
	}
}

func TestFilterLoadPatternDropsHiddenOptions(t *testing.T) {
	p := filterpattern.Pattern{Pattern: "*.jpg", Mode: "bogus", Types: []string{"Pictures", "sculptures"}, DirsOnly: true, IncludeMeta: true, OnlyMeta: true}
	st := FilterDialogState{Target: FilterTargetCompare}
	st.LoadPattern(p)
	if st.DirsOnly || st.FilesOnly || st.IncludeMetaColumns || st.OnlyMetaColumns {
		t.Fatalf("compare target kept panel-only options: %+v", st)
	}
	if st.PatternMode != panel.GroupPatternSimple {
		t.Fatalf("unknown mode = %v, want simple", st.PatternMode)
	}
	var want [FilterTypeCount]bool
	want[FilterTypePictures] = true
	if st.Types != want {
		t.Fatalf("types = %v, want only Pictures", st.Types)
	}
}

func TestSavedPatternLine(t *testing.T) {
	for _, c := range []struct {
		p    filterpattern.Pattern
		want string
	}{
		{filterpattern.Pattern{Name: "cedar", Description: "maple"}, "cedar — maple"},
		{filterpattern.Pattern{Name: "cedar"}, "cedar"},
		{filterpattern.Pattern{Pattern: "*.log", Mode: "shell", Types: []string{"videos"}, Invert: true}, "shell: *.log · Videos · invert"},
		{filterpattern.Pattern{Types: []string{"videos"}, DirsOnly: true}, "directories only"},
	} {
		if got := SavedPatternLine(c.p); got != c.want {
			t.Errorf("SavedPatternLine(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}

func TestGroupSelectSavedPatternRoundTrip(t *testing.T) {
	st := GroupSelectState{Context: "find", Text: "meadow/", PatternMode: panel.GroupPatternShell, DirsOnly: true, FullPath: true}
	p := st.SavedPattern("otter", "")
	if !p.FullPath || p.Invert || !p.DirsOnly || p.Mode != filterpattern.ModeShell {
		t.Fatalf("SavedPattern = %+v", p)
	}
	panelCtx := GroupSelectState{Context: "panel"}
	panelCtx.LoadPattern(filterpattern.Pattern{Pattern: "x", FullPath: true, Invert: true, FilesOnly: true})
	if panelCtx.FullPath || !panelCtx.FilesOnly {
		t.Fatalf("panel context: FullPath %v FilesOnly %v, want no full path (not shown), files only kept", panelCtx.FullPath, panelCtx.FilesOnly)
	}
	if got := (GroupSelectState{Context: "panel", FullPath: true}).SavedPattern("", ""); got.FullPath {
		t.Fatal("a hidden Match full path must not be saved")
	}
	findCtx := GroupSelectState{Context: "find"}
	findCtx.LoadPattern(p)
	if !findCtx.FullPath || !findCtx.DirsOnly || findCtx.Text != "meadow/" {
		t.Fatalf("find context load = %+v", findCtx)
	}
	filter := FilterDialogState{}
	filter.LoadPattern(filterpattern.Pattern{Invert: true, FullPath: true})
	if !filter.Invert {
		t.Fatal("Filter keeps Invert")
	}
}
