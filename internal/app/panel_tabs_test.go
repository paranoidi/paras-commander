package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/columnprofile"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// tabsApp builds an app over a directory holding the subdirectories orchard, lantern and willow,
// each with a file.
func tabsApp(t *testing.T) (*App, tcell.SimulationScreen, string) {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"orchard", "lantern", "willow"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, sub, "ember.txt"))
	}
	writeFile(t, filepath.Join(dir, "harbor.txt"))
	screen := newScreen(t, 100, 24)
	return newApp(t, screen, dir), screen, dir
}

func settleTabLoads(t *testing.T, app *App, screen tcell.SimulationScreen) {
	t.Helper()
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
		return !app.model.Primary.ListingPending && !app.model.Secondary.ListingPending
	})
}

func navigateTab(t *testing.T, app *App, screen tcell.SimulationScreen, panelID int, path string) {
	t.Helper()
	if err := app.panelByID(panelID).NavigateTo(path, "", app.panelViewportRows(panelID)); err != nil {
		t.Fatal(err)
	}
	settleTabLoads(t, app, screen)
}

func usedSlots(app *App, panelID int) []int {
	inst := app.panelInst(panelID)
	var slots []int
	for i := range inst.tabs {
		if inst.tabs[i] != nil || i+1 == inst.activeTab {
			slots = append(slots, i+1)
		}
	}
	return slots
}

func ctrlKey(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModCtrl) }

func TestTabCreateSwitchCloseWithGaps(t *testing.T) {
	app, screen, dir := tabsApp(t)
	orchard := filepath.Join(dir, "orchard")

	app.selectTab(ui.PrimaryPanel, 3) // an empty slot: a copy of the current tab
	settleTabLoads(t, app, screen)
	if app.panelInst(0).activeTab != 3 || app.model.Primary.PathString() != dir {
		t.Fatalf("slot %d path %q, want slot 3 at %q", app.panelInst(0).activeTab, app.model.Primary.PathString(), dir)
	}
	navigateTab(t, app, screen, ui.PrimaryPanel, orchard)

	app.selectTab(ui.PrimaryPanel, 1)
	settleTabLoads(t, app, screen)
	if got := app.model.Primary.PathString(); got != dir {
		t.Fatalf("slot 1 shows %q, want the directory it was left in", got)
	}
	app.selectTab(ui.PrimaryPanel, 1) // the current slot is a no-op
	app.selectTab(ui.PrimaryPanel, 3)
	settleTabLoads(t, app, screen)
	if got := app.model.Primary.PathString(); got != orchard {
		t.Fatalf("slot 3 shows %q, want %q", got, orchard)
	}

	app.selectTab(ui.PrimaryPanel, 5)
	if got := usedSlots(app, 0); len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Fatalf("slots = %v, want 1 3 5", got)
	}
	if bar := app.model.Tabs[ui.PrimaryPanel]; bar.Active != 5 || bar.Count() != 3 || bar.Paths[2].IsZero() {
		t.Fatalf("tab bar = %+v", bar)
	}

	app.closeTab(ui.PrimaryPanel) // 5 -> nearest lower used slot: 3
	if app.panelInst(0).activeTab != 3 {
		t.Fatalf("closing 5 went to slot %d, want 3", app.panelInst(0).activeTab)
	}
	app.selectTab(ui.PrimaryPanel, 1)
	app.closeTab(ui.PrimaryPanel) // 1 has no lower slot: the next higher is 3
	if app.panelInst(0).activeTab != 3 {
		t.Fatalf("closing 1 went to slot %d, want 3", app.panelInst(0).activeTab)
	}
	settleTabLoads(t, app, screen)
	app.closeTab(ui.PrimaryPanel)
	if app.panelInst(0).activeTab != 3 || len(usedSlots(app, 0)) != 1 {
		t.Fatalf("the last tab must stay, slots = %v", usedSlots(app, 0))
	}
	if app.model.Message != noOtherTabMessage {
		t.Fatalf("toast = %q, want %q", app.model.Message, noOtherTabMessage)
	}
	if app.model.Tabs[ui.PrimaryPanel].Count() != 1 {
		t.Fatal("one tab shows no bar")
	}
}

func TestTabsAreIndependentPerPanel(t *testing.T) {
	app, _, _ := tabsApp(t)
	app.selectTab(ui.SecondaryPanel, 2)
	if app.model.Tabs[ui.PrimaryPanel].Count() > 1 || app.model.Tabs[ui.SecondaryPanel].Count() != 2 {
		t.Fatalf("bars = %d / %d tabs, want 1 / 2", app.model.Tabs[0].Count(), app.model.Tabs[1].Count())
	}
	if p, s := app.model.TabRows(); p || !s {
		t.Fatalf("TabRows = %v %v, want false true", p, s)
	}
}

func TestTabKeysAndNoOpOutsideTheBrowser(t *testing.T) {
	app, _, _ := tabsApp(t)
	if quit, _ := app.handleKey(ctrlKey('2')); quit {
		t.Fatal("tab key quit")
	}
	if app.panelInst(0).activeTab != 2 {
		t.Fatalf("C-2 left slot %d active", app.panelInst(0).activeTab)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModCtrl))
	if app.panelInst(0).activeTab != 1 || len(usedSlots(app, 0)) != 1 {
		t.Fatalf("C-w left slots %v active %d", usedSlots(app, 0), app.panelInst(0).activeTab)
	}
	app.model.ViewMode = ui.ViewJobs
	app.dispatch(keymap.PanelTabActions[3])
	app.selectTab(ui.PrimaryPanel, 4)
	if len(usedSlots(app, 0)) != 1 {
		t.Fatal("tabs changed outside the browser")
	}
	if p, s := app.model.TabRows(); p || s {
		t.Fatal("tab rows outside the browser")
	}
}

func TestCloseTabIgnoredInQuickFilterAndTabKeyCancelsIt(t *testing.T) {
	app, _, _ := tabsApp(t)
	app.selectTab(ui.PrimaryPanel, 2)
	app.focusedPanel().OpenFilter(app.focusedViewportRows())
	app.focusedPanel().AppendFilterRune('h', app.focusedViewportRows())
	if !app.inQuickFilterUI() {
		t.Fatal("quick filter not open")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModCtrl))
	if len(usedSlots(app, 0)) != 2 || !app.inQuickFilterUI() {
		t.Fatalf("C-w closed a tab (%v) or the filter (%v)", usedSlots(app, 0), app.inQuickFilterUI())
	}
	app.handleKey(ctrlKey('1'))
	if app.panelInst(0).activeTab != 1 || app.inQuickFilterUI() {
		t.Fatalf("C-1: active %d, filter open %v", app.panelInst(0).activeTab, app.inQuickFilterUI())
	}
	if f := app.panelInst(0).tabs[1].state.Filter; f.Active || f.Editing || f.Query != "" {
		t.Fatalf("the tab left keeps the filter: %+v", f)
	}
}

func TestSlowLoadFinishingAfterSwitchNeverLandsInOtherTab(t *testing.T) {
	app, screen, dir := tabsApp(t)
	slow := filepath.Join(dir, "lantern")
	release := make(chan struct{})
	started := make(chan struct{}, 4)
	swapFetchListingForAsyncLoad(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		if snap.Loc.String() == slow {
			started <- struct{}{}
			<-release
		}
		return panel.FetchListing(ctx, snap)
	})

	pan := app.panelByID(ui.PrimaryPanel)
	if err := pan.NavigateTo(slow, "", app.focusedViewportRows()); err != nil {
		t.Fatal(err)
	}
	<-started
	app.selectTab(ui.PrimaryPanel, 2)
	if app.model.Primary.ListingPending && app.model.Primary.ListingPendingPath == slow {
		t.Fatal("the new tab waits for the load of the tab left")
	}
	if len(app.model.Primary.History) > 0 && app.model.Primary.History[0] == slow {
		t.Fatalf("history of the new tab holds the pending visit: %v", app.model.Primary.History)
	}
	close(release)
	time.Sleep(20 * time.Millisecond)
	settleTabLoads(t, app, screen)
	if got := app.model.Primary.PathString(); got != dir {
		t.Fatalf("the slow load landed in the other tab: %q", got)
	}

	app.selectTab(ui.PrimaryPanel, 1) // the target resumes
	settleTabLoads(t, app, screen)
	if got := app.model.Primary.PathString(); got != slow {
		t.Fatalf("returning shows %q, want the pending target %q", got, slow)
	}
	app.selectTab(ui.PrimaryPanel, 2)
	settleTabLoads(t, app, screen)
	if got := app.model.Primary.PathString(); got != dir {
		t.Fatalf("slot 2 shows %q after the resume, want %q", got, dir)
	}
}

func TestTwoTabsOnTheSameDirectoryStayIndependent(t *testing.T) {
	app, screen, dir := tabsApp(t)
	selectPanelEntryByName(t, app.focusedPanel(), "harbor.txt")
	app.focusedPanel().ToggleSelection()
	app.selectTab(ui.PrimaryPanel, 2)
	settleTabLoads(t, app, screen)
	if app.model.Primary.PathString() != dir || app.model.Primary.SelectedPathCount() != 0 {
		t.Fatalf("the copy starts with %d selected at %q", app.model.Primary.SelectedPathCount(), app.model.Primary.PathString())
	}
	other := app.panelInst(0).tabs[0].state.ListingEpoch
	if app.model.Primary.ListingEpoch <= other {
		t.Fatalf("epoch %d must exceed the other tab's %d", app.model.Primary.ListingEpoch, other)
	}
	selectPanelEntryByName(t, app.focusedPanel(), "lantern")
	app.focusedPanel().ToggleSelection()
	app.selectTab(ui.PrimaryPanel, 1)
	settleTabLoads(t, app, screen)
	if got := app.model.Primary.SelectedPathCount(); got != 1 {
		t.Fatalf("slot 1 selection = %d, want its own 1", got)
	}
	if _, ok := app.model.Primary.SelectedPaths[filepath.Join(dir, "harbor.txt")]; !ok {
		t.Fatalf("slot 1 lost its selection: %v", app.model.Primary.SelectedPaths)
	}
}

func TestTabsKeepSortCarouselTreeAndMetaColumnsPerTab(t *testing.T) {
	app, screen, _ := tabsApp(t)
	app.focusedPanel().Sort.Reverse = true
	app.focusedPanel().CarouselMode = true
	app.metaCtrl.ApplyProfile(ui.PrimaryPanel, columnprofile.Profile{Name: "orchard", Columns: []string{"size"}, Meta: []string{"meadow"}})
	if len(app.model.MetaResults[ui.PrimaryPanel]) != 1 {
		t.Fatal("precondition: meta column active in slot 1")
	}

	app.selectTab(ui.PrimaryPanel, 2) // the copy carries all of it ...
	settleTabLoads(t, app, screen)
	if !app.model.Primary.Sort.Reverse || !app.model.Primary.CarouselMode || len(app.model.MetaResults[ui.PrimaryPanel]) != 1 {
		t.Fatalf("copy: reverse %v carousel %v meta %d", app.model.Primary.Sort.Reverse, app.model.Primary.CarouselMode, len(app.model.MetaResults[ui.PrimaryPanel]))
	}
	// ... and the two diverge.
	app.model.Primary.Sort.Reverse = false
	app.model.Primary.CarouselMode = false
	app.model.Primary.SetListLayout(panel.ListLayoutTree, app.focusedViewportRows())
	app.metaCtrl.ApplyProfile(ui.PrimaryPanel, columnprofile.Profile{Name: config.ColumnProfileBrief, Columns: []string{"size"}})
	if app.model.MetaResults[ui.PrimaryPanel] != nil {
		t.Fatal("precondition: no meta column in slot 2")
	}

	app.selectTab(ui.PrimaryPanel, 1)
	if !app.model.Primary.Sort.Reverse || !app.model.Primary.CarouselMode || app.model.Primary.ListLayout != panel.ListLayoutFlat {
		t.Fatalf("slot 1: reverse %v carousel %v layout %v", app.model.Primary.Sort.Reverse, app.model.Primary.CarouselMode, app.model.Primary.ListLayout)
	}
	if len(app.model.MetaResults[ui.PrimaryPanel]) != 1 {
		t.Fatal("slot 1 lost its meta column")
	}
	app.selectTab(ui.PrimaryPanel, 2)
	if app.model.Primary.Sort.Reverse || app.model.Primary.CarouselMode || app.model.Primary.ListLayout != panel.ListLayoutTree {
		t.Fatalf("slot 2: reverse %v carousel %v layout %v", app.model.Primary.Sort.Reverse, app.model.Primary.CarouselMode, app.model.Primary.ListLayout)
	}
	if app.model.MetaResults[ui.PrimaryPanel] != nil {
		t.Fatal("slot 2 gained a meta column")
	}
}

func TestTabSwitchCarriesStashAndHiddenVisibility(t *testing.T) {
	app, screen, dir := tabsApp(t)
	app.selectTab(ui.PrimaryPanel, 2)
	settleTabLoads(t, app, screen)
	selectPanelEntryByName(t, app.focusedPanel(), "harbor.txt")
	app.focusedPanel().ToggleSelection()
	if !app.focusedPanel().StashSaveFromSelection() {
		t.Fatal("stash save failed")
	}
	writeFile(t, filepath.Join(dir, ".secret"))
	if err := app.toggleHiddenGlobal(); err != nil {
		t.Fatal(err)
	}
	settleTabLoads(t, app, screen)

	app.selectTab(ui.PrimaryPanel, 1)
	settleTabLoads(t, app, screen)
	if app.model.Primary.StashEmpty() {
		t.Fatal("the stash must follow the panel into the other tab")
	}
	if app.model.Primary.ShowHidden != app.model.Secondary.ShowHidden || !app.model.Primary.ShowHidden {
		t.Fatalf("hidden visibility = %v / %v, want the global value", app.model.Primary.ShowHidden, app.model.Secondary.ShowHidden)
	}
	found := false
	for _, e := range app.model.Primary.Entries {
		found = found || e.Name == ".secret"
	}
	if !found {
		t.Fatal("slot 1 still lists without hidden files")
	}
}

func TestTabBarShrinksPanesAndHiddenPanelNeverRequestsARow(t *testing.T) {
	app, _, _ := tabsApp(t)
	w, h := app.screen.Size()
	plain := app.layoutForTerminalSize(w, h)
	if plain.PrimaryTabs.Width != 0 {
		t.Fatalf("one tab got a row: %+v", plain.PrimaryTabs)
	}
	app.selectTab(ui.SecondaryPanel, 2)
	l := app.layoutForTerminalSize(w, h)
	if l.SecondaryTabs.Width == 0 || l.Primary.Y != plain.Primary.Y+1 || l.Secondary.Y != plain.Secondary.Y+1 {
		t.Fatalf("panes must move down together: %+v vs %+v", l, plain)
	}
	app.model.ActivePanel = ui.PrimaryPanel
	app.model.HideInactivePanel = true
	hidden := app.layoutForTerminalSize(w, h)
	if hidden.Secondary.Width != 0 || hidden.SecondaryTabs.Width != 0 || hidden.PrimaryTabs.Width != 0 {
		t.Fatalf("a hidden panel's tabs took a row: %+v", hidden)
	}
}

func TestTabClickSwitchesAndFocusesThePanel(t *testing.T) {
	app, _, dir := tabsApp(t)
	app.config.UI.Mouse = true
	app.selectTab(ui.SecondaryPanel, 2)
	navigateTab(t, app, app.screen.(tcell.SimulationScreen), ui.SecondaryPanel, filepath.Join(dir, "willow"))
	app.model.ActivePanel = ui.PrimaryPanel
	w, h := app.screen.Size()
	layout := app.layoutForTerminalSize(w, h)
	// Slot 1 is the first tab: " 1 label ", the click lands on its number.
	x, y := layout.SecondaryTabs.X+1, layout.SecondaryTabs.Y
	app.handleMouse(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	if app.model.ActivePanel != ui.SecondaryPanel || app.panelInst(1).activeTab != 1 {
		t.Fatalf("click: active panel %d slot %d, want secondary slot 1", app.model.ActivePanel, app.panelInst(1).activeTab)
	}
}

func TestQueuedTreeChildResultOfACancelledFetchIsDropped(t *testing.T) {
	app, _, dir := tabsApp(t)
	app.model.Primary.SetListLayout(panel.ListLayoutTree, 10)
	orchard := filepath.Join(dir, "orchard")
	for i := range app.model.Primary.TreeRoots {
		if app.model.Primary.TreeRoots[i].ID == orchard {
			app.model.Primary.TreeRoots[i].Value.Loading = true
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app.treeChildResults.push(treeChildLoadPayload{panelID: ui.PrimaryPanel, dirID: orchard, ctx: ctx})
	app.applyTreeChildResults()
	for _, n := range app.model.Primary.TreeRoots {
		if n.ID == orchard && (!n.Value.Loading || n.Children != nil) {
			t.Fatal("a cancelled fetch's queued result was applied")
		}
	}
}

// Extended-key terminals may report Ctrl+W as the letter with ModCtrl; it must not type into the
// filter either.
func TestCtrlWInQuickFilterTypesNothing(t *testing.T) {
	for _, ev := range []*tcell.EventKey{tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModCtrl), ctrlKey('w')} {
		app, screen, _ := tabsApp(t)
		settleTabLoads(t, app, screen)
		app.selectTab(ui.PrimaryPanel, 2)
		settleTabLoads(t, app, screen)
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone))
		app.handleKey(ev)
		if got := app.model.Primary.Filter.Query; got != "h" {
			t.Errorf("%s: filter query = %q, want \"h\"", ev.Name(), got)
		}
		if got := usedSlots(app, ui.PrimaryPanel); len(got) != 2 {
			t.Errorf("%s: used slots = %v, want two", ev.Name(), got)
		}
	}
}

func TestTabKeyInDialogPathFieldTypesInactivePanelTabPath(t *testing.T) {
	app, screen, dir := tabsApp(t)
	orchard, lantern := filepath.Join(dir, "orchard"), filepath.Join(dir, "lantern")
	// Secondary: slot 1 at orchard (stored), slot 2 at lantern (shown). Primary stays active.
	navigateTab(t, app, screen, ui.SecondaryPanel, orchard)
	app.selectTab(ui.SecondaryPanel, 2)
	settleTabLoads(t, app, screen)
	navigateTab(t, app, screen, ui.SecondaryPanel, lantern)
	p := app.focusedPanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		if e, _, ok := p.VisibleEntry(i); ok && e.Name == "harbor.txt" {
			p.Cursor = i
		}
	}
	slotsBefore := usedSlots(app, ui.PrimaryPanel)

	app.dialogCtrl.OpenCopyDialog()
	if !app.model.TransferDialog.FocusOnDest() {
		t.Fatal("copy dialog should focus the destination")
	}
	for i, want := range []string{orchard, lantern} {
		app.handleKey(ctrlKey('1' + rune(i)))
		if got := app.model.TransferDialog.Destination.Value; got != want+"/" {
			t.Fatalf("C-%d: destination %q, want %q", i+1, got, want+"/")
		}
	}
	app.handleKey(ctrlKey('5')) // unused slot: field unchanged, toast, no digit typed
	if got := app.model.TransferDialog.Destination.Value; got != lantern+"/" {
		t.Fatalf("C-5: destination %q, want it unchanged", got)
	}
	if got := app.model.Message; got != "Inactive panel has no tab 5" || app.model.MessageUrgency != ui.MessageUrgencyWarn {
		t.Fatalf("C-5: message %q urgency %v, want a warning", got, app.model.MessageUrgency)
	}
	// Off the path field the key leaves the destination alone.
	app.model.TransferDialog.Focus = dlg.OK
	app.handleKey(ctrlKey('1'))
	if got := app.model.TransferDialog.Destination.Value; got != lantern+"/" {
		t.Fatalf("C-1 on a button: destination %q, want it unchanged", got)
	}
	app.dialogCtrl.CloseTransferDialog()

	p.SelectedPaths = map[string]bool{orchard: true}
	app.dialogCtrl.OpenFlattenDialog()
	app.handleKey(ctrlKey('1'))
	if got := app.model.FlattenDialog.Destination.Value; got != orchard+"/" {
		t.Fatalf("flatten: destination %q, want %q", got, orchard+"/")
	}
	app.dialogCtrl.CloseFlattenDialog()

	p.SelectedPaths = nil
	app.dispatch(keymap.ActionFileSymlink)
	if !app.model.FileDialog.Open {
		t.Fatalf("symlink dialog should be open, message %q", app.model.Message)
	}
	app.handleKey(ctrlKey('2'))
	if f := app.dialogCtrl.FocusedField(); f == nil || f.Value != lantern+"/" {
		t.Fatalf("symlink: focused field %+v, want %q", f, lantern+"/")
	}
	if got := usedSlots(app, ui.PrimaryPanel); !slices.Equal(got, slotsBefore) {
		t.Fatalf("primary slots %v, want %v", got, slotsBefore)
	}
}
