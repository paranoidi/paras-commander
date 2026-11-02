package find

import (
	"slices"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/findcrit"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func newAdvancedTestHandler(t *testing.T) *Handler {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(120, 40)
	h := newTestFindHandler(&fakeFindHost{}, &ui.Model{})
	h.screen = screen
	h.config = config.Default()
	st := &h.model.FindDialog
	st.Open, st.RootPath, st.Focus = true, t.TempDir(), dialog.FindQuery
	h.startFindIndexer()
	t.Cleanup(h.stopFindIndexer)
	return h
}

func TestCommitAdvancedRestartsOnlyForMissingMetadata(t *testing.T) {
	t.Parallel()
	h := newAdvancedTestHandler(t)
	st := &h.model.FindDialog
	a := &st.Advanced

	gen := h.scanGen
	a.Symlinks = true
	h.commitAdvanced()
	if h.scanGen != gen || h.indexStat != findcrit.StatNone {
		t.Fatalf("symlinks need no metadata: scanGen %d -> %d, IndexStat %d", gen, h.scanGen, h.indexStat)
	}
	if !h.criteria.Symlinks {
		t.Fatal("criteria not applied")
	}

	a.Fields[dialog.FindAdvModifiedNewer].Value = "3d"
	h.commitAdvanced()
	if h.scanGen == gen || h.indexStat != findcrit.StatMeta || h.criteria.Modified.Newer == 0 {
		t.Fatalf("age criterion: want restart at StatMeta, got scanGen %d IndexStat %d criteria %+v", h.scanGen, h.indexStat, h.criteria.Modified)
	}

	gen = h.scanGen
	a.Fields[dialog.FindAdvSizeMin].Value = "1M"
	h.commitAdvanced()
	if h.scanGen != gen || h.criteria.MinSize != 1<<20 {
		t.Fatalf("size needs no more metadata than age: scanGen %d -> %d, MinSize %d", gen, h.scanGen, h.criteria.MinSize)
	}

	a.Empty = true
	h.commitAdvanced()
	if h.scanGen == gen || h.indexStat != findcrit.StatMetaEmptyDirs {
		t.Fatalf("empty: want restart at StatMetaEmptyDirs, got IndexStat %d", h.indexStat)
	}

	gen = h.scanGen
	h.ResetAdvanced()
	if h.scanGen != gen || h.criteria.Active() {
		t.Fatalf("reset: want no restart and no criteria, got scanGen %d -> %d, %+v", gen, h.scanGen, h.criteria)
	}
}

func TestCommitAdvancedExcludeRestarts(t *testing.T) {
	t.Parallel()
	h := newAdvancedTestHandler(t)
	st := &h.model.FindDialog
	field := &st.Advanced.Fields[dialog.FindAdvExclude]

	gen := h.scanGen
	field.Value = "node_modules, *.o"
	h.commitAdvanced()
	if h.scanGen == gen || !slices.Equal(h.indexExclude, findcrit.Excludes{"node_modules", "*.o"}) {
		t.Fatalf("exclude: want restart, got scanGen %d -> %d indexExclude %q", gen, h.scanGen, h.indexExclude)
	}
	gen = h.scanGen
	field.Value = "node_modules   *.o"
	h.commitAdvanced()
	if h.scanGen != gen {
		t.Fatal("same patterns, other separators: want no restart")
	}
	field.Value = "[broken"
	h.commitAdvanced()
	if !field.Invalid || h.exclude != nil {
		t.Fatalf("invalid exclude: Invalid=%v exclude=%v", field.Invalid, h.exclude)
	}
}

func TestParseAdvancedFlagsInvalidFields(t *testing.T) {
	t.Parallel()
	h := newAdvancedTestHandler(t)
	st := &h.model.FindDialog
	a := &st.Advanced
	a.Fields[dialog.FindAdvModifiedOlder].Value = "3x"
	a.Fields[dialog.FindAdvSizeMax].Value = "lots"
	a.Fields[dialog.FindAdvOwner].Value = "nobody-by-that-name-zz"
	gen := h.scanGen
	h.commitAdvanced()
	for _, f := range []dialog.FindAdvField{dialog.FindAdvModifiedOlder, dialog.FindAdvSizeMax, dialog.FindAdvOwner} {
		if !a.Fields[f].Invalid {
			t.Errorf("field %d: want Invalid", f)
		}
	}
	if h.criteria.Active() || h.scanGen != gen {
		t.Fatalf("invalid fields must not apply: criteria %+v, scanGen %d -> %d", h.criteria, gen, h.scanGen)
	}
	if a.Summary(theme.Theme{}, 200) != "" {
		t.Fatal("invalid fields must not show in the summary")
	}
}

func TestMatchModeCycleAndQueryValidation(t *testing.T) {
	t.Parallel()
	h := newAdvancedTestHandler(t)
	st := &h.model.FindDialog
	st.Query = "(lantern"
	h.validateQuery(st)
	if st.QueryInvalid {
		t.Fatal("fuzzy queries are never invalid")
	}
	h.RunAction(keymap.ActionFindCycleMatchMode) // glob
	if st.Advanced.Mode != findcrit.ModeGlob || st.QueryInvalid {
		t.Fatalf("glob: mode %d invalid %v", st.Advanced.Mode, st.QueryInvalid)
	}
	h.RunAction(keymap.ActionFindCycleMatchMode) // regex
	if st.Advanced.Mode != findcrit.ModeRegex || !st.QueryInvalid {
		t.Fatalf("regex: mode %d invalid %v, want an invalid query", st.Advanced.Mode, st.QueryInvalid)
	}
	if h.findFilterEmpty(st) {
		t.Fatal("a regex query filters")
	}
	h.RunAction(keymap.ActionFindCycleMatchMode) // back to fuzzy
	if st.Advanced.Mode != findcrit.ModeFuzzy || st.QueryInvalid {
		t.Fatalf("fuzzy: mode %d invalid %v", st.Advanced.Mode, st.QueryInvalid)
	}
}

func TestToggleAdvancedFocusAndJumpMode(t *testing.T) {
	t.Parallel()
	h := newAdvancedTestHandler(t)
	st := &h.model.FindDialog
	h.RunAction(keymap.ActionFindToggleAdvanced)
	if !st.Advanced.Expanded {
		t.Fatal("toggle did not expand")
	}
	if _, ok := h.DialogSpec().Item(dialog.FindAdvFieldID(dialog.FindAdvOwner)); !ok {
		t.Fatal("expanded spec lacks the owner input")
	}
	st.Focus = dialog.FindAdvFieldID(dialog.FindAdvOwner)
	h.ApplyDialogEvent(dlg.Event{ID: dialog.FindAdvanced})
	if st.Advanced.Expanded || st.Focus != dialog.FindQuery {
		t.Fatalf("collapse: Expanded=%v Focus=%q, want collapsed and focus on the query", st.Advanced.Expanded, st.Focus)
	}

	st.JumpMode = true
	h.RunAction(keymap.ActionFindToggleAdvanced)
	h.RunAction(keymap.ActionFindCycleMatchMode)
	if st.Advanced.Expanded || st.Advanced.Mode != findcrit.ModeFuzzy {
		t.Fatal("jump mode must ignore the advanced actions")
	}
}

func TestAdvancedFieldCommitsWhenFocusLeaves(t *testing.T) {
	t.Parallel()
	h := newAdvancedTestHandler(t)
	st := &h.model.FindDialog
	st.Advanced.Expanded = true
	st.Focus = dialog.FindAdvFieldID(dialog.FindAdvSizeMin)
	st.Advanced.Fields[dialog.FindAdvSizeMin].Value = "10k"
	h.advEditing = st.Focus
	h.HandleDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if st.Focus == dialog.FindAdvFieldID(dialog.FindAdvSizeMin) {
		t.Fatal("Tab did not move focus")
	}
	if h.criteria.MinSize != 10<<10 || h.advEditing != "" {
		t.Fatalf("leaving the field did not commit: MinSize %d advEditing %q", h.criteria.MinSize, h.advEditing)
	}
}

func TestCloseDialogKeepsAdvancedForSession(t *testing.T) {
	t.Parallel()
	h := newAdvancedTestHandler(t)
	st := &h.model.FindDialog
	st.Advanced.Mode = findcrit.ModeRegex
	st.Advanced.Executables = true
	h.CloseDialog()
	if h.savedAdvanced.Mode != findcrit.ModeRegex || !h.savedAdvanced.Executables {
		t.Fatalf("savedAdvanced = %+v, want the closed dialog's section", h.savedAdvanced)
	}
}
