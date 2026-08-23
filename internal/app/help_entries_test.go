package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func newHelpEntriesApp(t *testing.T) *App {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)
	a, err := New(screen, func() (string, error) { return t.TempDir(), nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

func TestBuildHelpEntriesIncludesCrossPanelOpenActions(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	a, err := New(screen, func() (string, error) { return t.TempDir(), nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	entries := a.buildHelpEntries()
	var keysOpenDir, keysOpenCwd string
	for _, e := range entries {
		switch e.ActionID {
		case "panel.open-dir-in-other":
			keysOpenDir = e.Keys
		case "panel.open-active-path-in-other":
			keysOpenCwd = e.Keys
		}
	}
	if keysOpenDir == "" {
		t.Fatal("help missing panel.open-dir-in-other (expected default Alt+O)")
	}
	if !strings.Contains(keysOpenDir, "Alt") || !strings.Contains(keysOpenDir, "O") {
		t.Fatalf("unexpected keys display for open-dir-in-other: %q", keysOpenDir)
	}
	if keysOpenCwd == "" {
		t.Fatal("help missing panel.open-active-path-in-other (expected default Alt+I)")
	}
	if !strings.Contains(keysOpenCwd, "Alt") || !strings.Contains(keysOpenCwd, "I") {
		t.Fatalf("unexpected keys display for open-active-path-in-other: %q", keysOpenCwd)
	}
}

func TestBrowserHelpExcludesOtherViewAndDialogActions(t *testing.T) {
	a := newHelpEntriesApp(t)
	entries := a.buildHelpEntries()
	byID := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		byID[e.ActionID] = struct{}{}
	}
	if _, ok := byID[keymap.ActionCopy]; !ok {
		t.Fatal("browser help missing file.copy")
	}
	for _, forbidden := range []string{
		keymap.ActionJobsCancel,
		keymap.ActionCommandsTerminate,
		keymap.ActionMessagesClear,
		keymap.ActionFindSelectAll,
		keymap.ActionBookmarkDelete,
		keymap.ActionDestinationActivePanel,
		keymap.ActionAppShowHelp,
		"panel.disk-usage-abort-all",
	} {
		if _, ok := byID[forbidden]; ok {
			t.Fatalf("browser help should not include %q", forbidden)
		}
	}
}

func TestBrowserHelpDiskUsageRowsInViewSection(t *testing.T) {
	a := newHelpEntriesApp(t)
	var diskUsage []dialog.HelpEntry
	for _, e := range a.buildHelpEntries() {
		if e.ActionID != keymap.ActionPanelDiskUsageScan && e.ActionID != keymap.ActionPanelDiskUsageClear {
			continue
		}
		diskUsage = append(diskUsage, e)
	}
	if len(diskUsage) != 2 {
		t.Fatalf("disk usage help rows = %d, want 2 (scan + clear)", len(diskUsage))
	}
	if diskUsage[0].ActionID != keymap.ActionPanelDiskUsageScan {
		t.Fatalf("first disk usage row = %q, want %q", diskUsage[0].ActionID, keymap.ActionPanelDiskUsageScan)
	}
	if diskUsage[1].ActionID != keymap.ActionPanelDiskUsageClear {
		t.Fatalf("second disk usage row = %q, want %q", diskUsage[1].ActionID, keymap.ActionPanelDiskUsageClear)
	}
	if diskUsage[1].Title != "Abort and clear disk usage" {
		t.Fatalf("clear row title = %q, want %q", diskUsage[1].Title, "Abort and clear disk usage")
	}
	for _, e := range diskUsage {
		if e.Section != "View" {
			t.Fatalf("%s section = %q, want %q", e.ActionID, e.Section, "View")
		}
	}
}

func TestCommandsHelpIncludesTerminateWithOverlayKeys(t *testing.T) {
	a := newHelpEntriesApp(t)
	entries := a.buildHelpEntriesForView(ui.ViewCommands)
	idx := helpEntryIndex(entries, keymap.ActionCommandsTerminate)
	if idx < 0 {
		t.Fatal("commands help missing commands.terminate")
	}
	if !strings.Contains(entries[idx].Keys, "F8") {
		t.Fatalf("commands.terminate keys = %q, want F8 from overlay", entries[idx].Keys)
	}
	if helpEntryIndex(entries, keymap.ActionJobsCancel) >= 0 {
		t.Fatal("commands help should not include jobs.cancel")
	}
}

func TestJobsHelpIncludesAnswerBlocker(t *testing.T) {
	a := newHelpEntriesApp(t)
	entries := a.buildHelpEntriesForView(ui.ViewJobs)
	if helpEntryIndex(entries, keymap.ActionJobsAnswerBlocker) < 0 {
		t.Fatal("jobs help missing jobs.answer-blocker")
	}
}

func TestPreviewHelpUsesPreviewSectionAndExcludesBrowserActions(t *testing.T) {
	a := newHelpEntriesApp(t)
	entries := a.buildHelpEntriesForView(ui.ViewFilePreview)
	byID := make(map[string]dialog.HelpEntry, len(entries))
	for _, e := range entries {
		byID[e.ActionID] = e
	}
	for _, id := range []string{
		keymap.ActionFileViewThemePicker,
		keymap.ActionFileViewToggleRaw,
		keymap.ActionFileViewSearchStart,
		keymap.ActionFileEdit,
		keymap.ActionFileQuickViewPreviewPageUp,
	} {
		ent, ok := byID[id]
		if !ok {
			t.Fatalf("preview help missing %q", id)
		}
		if ent.Section != "Preview" {
			t.Fatalf("%q section = %q, want Preview", id, ent.Section)
		}
	}
	if ent, ok := byID[keymap.ActionPanelExternalBrowser]; !ok {
		t.Fatal("preview help missing panel.external-browser")
	} else if ent.Section != "Navigation" {
		t.Fatalf("panel.external-browser section = %q, want Navigation", ent.Section)
	}
	for _, forbidden := range []string{
		keymap.ActionAppUserMenu,
		keymap.ActionAppLeaderMenu,
		keymap.ActionAppUserMenuEdit,
		keymap.ActionPanelRefresh,
		keymap.ActionPanelDiskUsageClear,
		keymap.ActionBookmarkOpen,
		keymap.ActionBookmarkAdd,
		keymap.ActionAppOpenMenu,
		keymap.ActionNavUp,
		keymap.ActionCopy,
	} {
		if _, ok := byID[forbidden]; ok {
			t.Fatalf("preview help should not include %q", forbidden)
		}
	}
}
