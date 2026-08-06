package preview

import (
	"context"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

// fakeHost is a minimal Host stub for handler-logic tests that don't need a real *App. Layout
// queries return a plain non-zoom, non-hidden even split computed via the real leaf ui package
// so geometry-dependent assertions (column widths etc.) see representative numbers.
type fakeHost struct {
	model    *ui.Model
	cfg      config.Config
	styles   theme.Theme
	inactive int // inactive panel ID
	messages []string
}

func newFakeHost(model *ui.Model) *fakeHost {
	return &fakeHost{
		model:    model,
		cfg:      config.Default(),
		styles:   theme.Default(),
		inactive: ui.SecondaryPanel,
	}
}

func (f *fakeHost) LayoutForTerminalSize(w, h int) ui.Layout {
	return f.layout(w, h)
}

func (f *fakeHost) LayoutForTerminalSizePreview(w, h int, _ bool) ui.Layout {
	return f.layout(w, h)
}

func (f *fakeHost) layout(w, h int) ui.Layout {
	split := ui.PanelPaneSplit{ActivePanel: f.model.ActivePanel, ActivePercent: 50, InactivePercent: 50}
	return ui.CalculateLayoutWithOrientation(w, h, false, split, ui.SplitHorizontal, 0)
}

func (f *fakeHost) SetTransientMessage(text string, _ ui.MessageUrgency) {
	f.messages = append(f.messages, text)
}
func (f *fakeHost) SetErrorMessage(_ string, _ error)         {}
func (f *fakeHost) HandleQuit() bool                          { return true }
func (f *fakeHost) HandleQuitImmediate() bool                 { return true }
func (f *fakeHost) OpenMenu()                                 {}
func (f *fakeHost) OpenMenuByShortcut(rune) bool              { return false }
func (f *fakeHost) Dispatch(string)                           {}
func (f *fakeHost) TryDispatchAuxiliaryScreens(string) bool   { return false }
func (f *fakeHost) ActionFromKeyEvent(*tcell.EventKey) string { return "" }
func (f *fakeHost) ActivePanel() *panel.State {
	if f.model.ActivePanel == ui.SecondaryPanel {
		return &f.model.Secondary
	}
	return &f.model.Primary
}
func (f *fakeHost) PanelByID(panelID int) *panel.State {
	if panelID == ui.SecondaryPanel {
		return &f.model.Secondary
	}
	return &f.model.Primary
}
func (f *fakeHost) InactivePanelID() int                { return f.inactive }
func (f *fakeHost) ActiveViewportRows() int             { return 20 }
func (f *fakeHost) PanelViewportRows(int) int           { return 20 }
func (f *fakeHost) SelectionsStripViewportRows(int) int { return 0 }
func (f *fakeHost) InQuickFilterUI() bool               { return false }
func (f *fakeHost) SwitchPanel() {
	if f.model.ActivePanel == ui.SecondaryPanel {
		f.model.ActivePanel = ui.PrimaryPanel
		f.inactive = ui.SecondaryPanel
	} else {
		f.model.ActivePanel = ui.SecondaryPanel
		f.inactive = ui.PrimaryPanel
	}
}
func (f *fakeHost) SyncFollowTargetPath(*panel.State) (string, bool)        { return "", false }
func (f *fakeHost) PanelSyncFollowHeldListNav(string, *tcell.EventKey) bool { return false }
func (f *fakeHost) ArmPanelSyncFollowNavCoalesceAfterListNav()              {}
func (f *fakeHost) ClearPanelSyncFollowNavCoalesce()                        {}
func (f *fakeHost) ArmCursorNameHintNavCoalesceAfterListNav()               {}
func (f *fakeHost) PathVolumeContendsWithActiveJob(string) bool             { return false }
func (f *fakeHost) QuickViewGitStatusScheduler() panel.GitStatusScheduler   { return nil }
func (f *fakeHost) EffectivePaneSplitOrientation() ui.SplitOrientation      { return ui.SplitHorizontal }
func (f *fakeHost) PanelPaneSplit(int, bool) ui.PanelPaneSplit {
	return ui.PanelPaneSplit{ActivePanel: f.model.ActivePanel, ActivePercent: 50, InactivePercent: 50}
}
func (f *fakeHost) TerminalLayoutRows() int                   { return 0 }
func (f *fakeHost) BrowserMenuDefinitions() []menu.Definition { return nil }
func (f *fakeHost) CarouselAutohideInactivePanel() bool       { return false }
func (f *fakeHost) EditActiveFile()                           {}
func (f *fakeHost) EditFullscreenPreviewFile()                {}
func (f *fakeHost) OpenDeleteDialogForPreviewedFile()         {}
func (f *fakeHost) OpenPreviewLeaderMenu()                    {}
func (f *fakeHost) HandleFileDialogFieldKey(*tcell.EventKey, *dialog.FileDialogField, func()) bool {
	return false
}
func (f *fakeHost) PersistPartial(map[string]interface{}) error { return nil }
func (f *fakeHost) Config() config.Config                       { return f.cfg }
func (f *fakeHost) Styles() theme.Theme                         { return f.styles }
func (f *fakeHost) SetPreviewStyle(style string)                { f.cfg.Preview.Style = style }
func (f *fakeHost) ApplyPreviewStyle(name string) bool {
	f.cfg.Preview.Style = config.NormalizePreviewStyle(name)
	return true
}
func (f *fakeHost) SyncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range) {
	q := search.Parse(query)
	opts := search.Options{CaseInsensitive: caseInsensitive}
	results := q.Rank(lines, opts)
	ranked = make([]int, len(results))
	matchRanges = make([][]search.Range, matchRangeSlots)
	for i, r := range results {
		ranked[i] = r.Index
		if r.Index >= 0 && r.Index < matchRangeSlots {
			matchRanges[r.Index] = r.Result.Ranges
		}
	}
	return ranked, matchRanges
}
func (f *fakeHost) ClampFilteredListSelection(selected *int, rankedLen int) {
	if *selected >= rankedLen {
		if rankedLen == 0 {
			*selected = 0
		} else {
			*selected = rankedLen - 1
		}
	}
	if *selected < 0 {
		*selected = 0
	}
}
func (f *fakeHost) HandleFilteredListSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool {
	if focus != 0 || rankedLen <= 0 {
		return false
	}
	switch ev.Key() {
	case tcell.KeyUp:
		if *selected > 0 {
			*selected--
		}
		ensureScroll()
		return true
	case tcell.KeyDown:
		if *selected < rankedLen-1 {
			*selected++
		}
		ensureScroll()
		return true
	}
	return false
}

// newTestHandler builds a Handler wired to a fakeHost and a simulation screen, for tests that
// exercise handler logic directly without a full *app.App.
func newTestHandler(t *testing.T, w, h int) (*Handler, *fakeHost) {
	t.Helper()
	screen := uitest.Screen(t, w, h)
	model := &ui.Model{}
	fh := newFakeHost(model)
	handler := New(Deps{
		Host:   fh,
		Screen: screen,
		Model:  model,
		Mu:     &sync.RWMutex{},
		Ctx:    context.Background(),
	})
	return handler, fh
}

var _ Host = (*fakeHost)(nil)
