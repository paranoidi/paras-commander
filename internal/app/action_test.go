package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func defaultKeymap(tb testing.TB) *keymap.Map {
	tb.Helper()
	m, err := keymap.Default()
	if err != nil {
		tb.Fatalf("keymap.Default() error = %v", err)
	}
	return m
}

func TestActionFromKeyMapsNavigationKeys(t *testing.T) {
	km := defaultKeymap(t)
	tests := []struct {
		name string
		key  tcell.Key
		want string
	}{
		{name: "up", key: tcell.KeyUp, want: keymap.ActionNavUp},
		{name: "down", key: tcell.KeyDown, want: keymap.ActionNavDown},
		{name: "page up", key: tcell.KeyPgUp, want: keymap.ActionNavPageUp},
		{name: "page down", key: tcell.KeyPgDn, want: keymap.ActionNavPageDown},
		{name: "home", key: tcell.KeyHome, want: keymap.ActionNavTop},
		{name: "end", key: tcell.KeyEnd, want: keymap.ActionNavBottom},
		{name: "insert", key: tcell.KeyInsert, want: keymap.ActionPanelSelectToggle},
		{name: "enter", key: tcell.KeyEnter, want: keymap.ActionNavOpen},
		{name: "left", key: tcell.KeyLeft, want: keymap.ActionNavParent},
		{name: "right", key: tcell.KeyRight, want: keymap.ActionNavOpen},
		{name: "backspace", key: tcell.KeyBackspace, want: keymap.ActionNavParent},
		{name: "tab", key: tcell.KeyTab, want: keymap.ActionPanelSwitch},
		{name: "hide inactive panel", key: tcell.KeyBacktab, want: keymap.ActionPanelToggleHideInactive},
		{name: "delete", key: tcell.KeyCtrlD, want: keymap.ActionFileDelete},
		{name: "open menu", key: tcell.KeyF9, want: keymap.ActionAppOpenMenu},
		{name: "quit", key: tcell.KeyF10, want: keymap.ActionAppQuit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tt.key, 0, tcell.ModNone)
			got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
			if got != tt.want {
				t.Fatalf("actionFromKeyEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestActionFromKeyMapsQuitImmediate(t *testing.T) {
	km := defaultKeymap(t)
	ev := tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModShift)
	got := lookupActionForView(ev, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionAppQuitImmediate {
		t.Fatalf("Shift+F10 = %v, want %v", got, keymap.ActionAppQuitImmediate)
	}
}

// TestActionFromKeyMapsCtrlCToJobsCancel verifies that Ctrl+C triggers
// jobs.cancel only while the jobs view is focused. In the file browser,
// Ctrl+C copies (copy).
func TestActionFromKeyMapsCtrlCToJobsCancel(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	event := tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone)
	if got := lookupActionForView(event, bundle.Global, bundle.Jobs, bundle.Commands, bundle.Messages, bundle.FilePreview, bundle.Compare, bundle.Dedup, ui.ViewJobs); got != keymap.ActionJobsCancel {
		t.Fatalf("jobs view Ctrl+C = %v, want ActionJobsCancel", got)
	}
	if got := lookupActionForView(event, bundle.Global, bundle.Jobs, bundle.Commands, bundle.Messages, bundle.FilePreview, bundle.Compare, bundle.Dedup, ui.ViewBrowser); got != keymap.ActionCopy {
		t.Fatalf("browser Ctrl+C = %q, want %s", got, keymap.ActionCopy)
	}
}

func TestActionFromKeyColonOpensLeaderMenuInBrowser(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionAppLeaderMenu {
		t.Fatalf("actionFromKeyEvent(:) = %q, want %s", got, keymap.ActionAppLeaderMenu)
	}
}

func TestActionFromKeyEscDoesNotOpenLeaderMenuInBrowser(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got == keymap.ActionAppLeaderMenu {
		t.Fatalf("actionFromKeyEvent(Esc) = %q, want not %s", got, keymap.ActionAppLeaderMenu)
	}
}

func TestActionFromKeyMapsCtrlAltLeftToTreeCollapseAll(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt|tcell.ModCtrl)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelTreeCollapseAll {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelTreeCollapseAll", got)
	}
}

func TestActionFromKeyMapsAltShiftLeftToTreeCollapseAllFull(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt|tcell.ModShift)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelTreeCollapseAllFull {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelTreeCollapseAllFull", got)
	}
}

func TestActionFromKeyMapsCtrlAltRightToTreeExpandAllShallow(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt|tcell.ModCtrl)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelTreeExpandAllShallow {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelTreeExpandAllShallow", got)
	}
}

func TestActionFromKeyMapsAltLeftToTreeCollapse(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelTreeCollapse {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelTreeCollapse", got)
	}
}

func TestActionFromKeyMapsAltRightToTreeExpand(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelTreeExpand {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelTreeExpand", got)
	}
}

func TestActionFromKeyMapsAltUpToTreePrevSiblingDir(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelTreePrevSiblingDir {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelTreePrevSiblingDir", got)
	}
}

func TestActionFromKeyMapsAltDownToTreeNextSiblingDir(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelTreeNextSiblingDir {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelTreeNextSiblingDir", got)
	}
}

func TestActionFromKeyMapsCtrlArrowsToDirectoryHistory(t *testing.T) {
	km := defaultKeymap(t)
	fwd := tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl)
	if got := lookupActionForView(fwd, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser); got != keymap.ActionNavForward {
		t.Fatalf("actionFromKeyEvent(C-right) = %v, want ActionNavForward", got)
	}
	back := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModCtrl)
	if got := lookupActionForView(back, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser); got != keymap.ActionNavBackward {
		t.Fatalf("actionFromKeyEvent(C-left) = %v, want ActionNavBackward", got)
	}
}

func TestActionFromKeyMapsAltXToExternalBrowser(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelExternalBrowser {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelExternalBrowser", got)
	}
}

func TestActionFromKeyMapsAltEToEdit(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionFileEdit {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionFileEdit", got)
	}
}

func TestActionFromKeyMapsAltZToToggleZoomActivePanel(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelToggleZoomActivePanel {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelToggleZoomActivePanel", got)
	}
}

func TestActionFromKeyMapsAltOToOpenDirInOtherPanel(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelOpenDirInOther {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelOpenDirInOther", got)
	}
}

func TestActionFromKeyMapsAltIToOpenActivePathInOtherPanel(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelOpenActivePathInOther {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelOpenActivePathInOther", got)
	}
}

func TestActionFromKeyMapsMetaIToOpenActivePathInOtherPanel(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModMeta)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelOpenActivePathInOther {
		t.Fatalf("Meta+i = %v, want ActionPanelOpenActivePathInOther", got)
	}
}

func TestActionFromKeyMapsAltYToToggleSync(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelToggleSync {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelToggleSync", got)
	}
}

func TestActionFromKeyMapsCtrlAltPToTerminalTogglePanel(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModAlt|tcell.ModCtrl)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionTerminalTogglePanel {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionTerminalTogglePanel", got)
	}
}

func TestActionFromKeyMapsAltPToTerminalFocus(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionTerminalFocus {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionTerminalFocus", got)
	}
}

func TestActionFromKeyMapsAltHToHistoryDialog(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelHistoryDialog {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelHistoryDialog", got)
	}
}

func TestActionFromKeyMapsCtrlHToHistoryDialog(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyCtrlH, 0, tcell.ModCtrl)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelHistoryDialog {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelHistoryDialog", got)
	}
}

func TestActionFromKeyMapsCtrlFToFindDialog(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyCtrlF, 0, tcell.ModCtrl)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelFindDialog {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelFindDialog", got)
	}
}

func TestActionFromKeyMapsShiftAltDToClearDiskUsageData(t *testing.T) {
	km := defaultKeymap(t)
	ev := tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModAlt|tcell.ModShift)
	got := lookupActionForView(ev, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelDiskUsageClear {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelDiskUsageClear", got)
	}
}

func TestActionFromKeyMapsCtrlAltDToDuplicate(t *testing.T) {
	km := defaultKeymap(t)
	ev := tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModAlt)
	got := lookupActionForView(ev, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionFileDuplicate {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionFileDuplicate", got)
	}
}

func TestActionFromKeyMapsCtrlLToFlatten(t *testing.T) {
	km := defaultKeymap(t)
	ev := tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModCtrl)
	got := lookupActionForView(ev, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionFileFlatten {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionFileFlatten", got)
	}
}

func TestActionFromKeyMapsAltDToDiskUsageScan(t *testing.T) {
	km := defaultKeymap(t)
	ev := tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModAlt)
	got := lookupActionForView(ev, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelDiskUsageScan {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelDiskUsageScan", got)
	}
}

func TestActionFromKeyIgnoresAltF(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != "" {
		t.Fatalf("actionFromKeyEvent() = %v, want empty string", got)
	}
}

func TestActionFromKeyIgnoresUnknownKey(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone)
	got := lookupActionForView(event, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != "" {
		t.Fatalf("actionFromKeyEvent() = %v, want empty string", got)
	}
}

func TestActionFromKeyMapsF3F4(t *testing.T) {
	km := defaultKeymap(t)
	tests := []struct {
		name string
		ev   *tcell.EventKey
		want string
	}{
		{"F3 view", tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone), keymap.ActionFileView},
		{"S-F3 quick view", tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModShift), keymap.ActionFileQuickView},
		{"F4 edit", tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone), keymap.ActionFileEdit},
		{"F5 copy", tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone), keymap.ActionCopy},
		{"S-F5 duplicate", tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModShift), keymap.ActionFileDuplicate},
		{"F7 mkdir", tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone), keymap.ActionFileMkdir},
		{"S-F7 mkdir in other", tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModShift), keymap.ActionFileMkdirOpenInOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lookupActionForView(tt.ev, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLookupF8BrowserVsJobsOverlay(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	f8 := tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)
	if got := lookupActionForView(f8, bundle.Global, bundle.Jobs, bundle.Commands, bundle.Messages, bundle.FilePreview, bundle.Compare, bundle.Dedup, ui.ViewBrowser); got != keymap.ActionFileDelete {
		t.Fatalf("browser F8 = %q, want file.delete", got)
	}
	if got := lookupActionForView(f8, bundle.Global, bundle.Jobs, bundle.Commands, bundle.Messages, bundle.FilePreview, bundle.Compare, bundle.Dedup, ui.ViewJobs); got != keymap.ActionJobsClearFinished {
		t.Fatalf("jobs view F8 = %q, want jobs.clear-finished", got)
	}
}

func TestLookupF8BrowserVsMessagesOverlay(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	f8 := tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)
	if got := lookupActionForView(f8, bundle.Global, bundle.Jobs, bundle.Commands, bundle.Messages, bundle.FilePreview, bundle.Compare, bundle.Dedup, ui.ViewBrowser); got != keymap.ActionFileDelete {
		t.Fatalf("browser F8 = %q, want file.delete", got)
	}
	if got := lookupActionForView(f8, bundle.Global, bundle.Jobs, bundle.Commands, bundle.Messages, bundle.FilePreview, bundle.Compare, bundle.Dedup, ui.ViewMessages); got != keymap.ActionMessagesClear {
		t.Fatalf("messages view F8 = %q, want messages.clear", got)
	}
}
