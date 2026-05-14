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
		{name: "focus selections", key: tcell.KeyBacktab, want: keymap.ActionPanelFocusSelections},
		{name: "disk usage", key: tcell.KeyCtrlD, want: keymap.ActionPanelDiskUsageScan},
		{name: "open menu", key: tcell.KeyF9, want: keymap.ActionAppOpenMenu},
		{name: "quit", key: tcell.KeyF10, want: keymap.ActionAppQuit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tt.key, 0, tcell.ModNone)
			got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
			if got != tt.want {
				t.Fatalf("actionFromKeyEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestActionFromKeyMapsCtrlCToJobsCancel verifies that Ctrl+C triggers
// jobs.cancel only while the jobs view is focused. After the unification
// of jobs.* shortcuts under [jobs_action_keys], Ctrl+C is no longer in
// the global map: it must resolve via the overlay when viewJobs=true,
// and stay unbound in browser mode (so a stray Ctrl+C never silently
// triggers a job cancel from outside the jobs screen).
func TestActionFromKeyMapsCtrlCToJobsCancel(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	event := tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone)
	if got := lookupActionForView(event, bundle.Global, bundle.Jobs, bundle.Commands, ui.ViewJobs); got != keymap.ActionJobsCancel {
		t.Fatalf("jobs view Ctrl+C = %v, want ActionJobsCancel", got)
	}
	if got := lookupActionForView(event, bundle.Global, bundle.Jobs, bundle.Commands, ui.ViewBrowser); got != "" {
		t.Fatalf("browser Ctrl+C = %q, want unbound", got)
	}
}

func TestActionFromKeyEscDoesNotMapToQuit(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != "" {
		t.Fatalf("actionFromKeyEvent(Esc) = %q, want unbound (dialogs/filter use Esc explicitly)", got)
	}
}

func TestActionFromKeyMapsCtrlAltLeftToForwardHistory(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt|tcell.ModCtrl)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionNavForward {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionNavForward", got)
	}
}

func TestActionFromKeyMapsCtrlAltRightToBackwardHistory(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt|tcell.ModCtrl)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionNavBackward {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionNavBackward", got)
	}
}

func TestActionFromKeyMapsAltEToExternalBrowser(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelExternalBrowser {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelExternalBrowser", got)
	}
}

func TestActionFromKeyMapsAltZToToggleZoomActivePanel(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelToggleZoomActivePanel {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelToggleZoomActivePanel", got)
	}
}

func TestActionFromKeyMapsAltOToOpenDirInOtherPanel(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelOpenDirInOther {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelOpenDirInOther", got)
	}
}

func TestActionFromKeyMapsAltIToOpenActivePathInOtherPanel(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelOpenActivePathInOther {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelOpenActivePathInOther", got)
	}
}

func TestActionFromKeyMapsMetaIToOpenActivePathInOtherPanel(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModMeta)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelOpenActivePathInOther {
		t.Fatalf("Meta+i = %v, want ActionPanelOpenActivePathInOther", got)
	}
}

func TestActionFromKeyMapsCtrlAltOToToggleSync(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyCtrlO, 0, tcell.ModAlt|tcell.ModCtrl)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelToggleSync {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelToggleSync", got)
	}
}

func TestActionFromKeyMapsAltHToHistoryDialog(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelHistoryDialog {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelHistoryDialog", got)
	}
}

func TestActionFromKeyMapsCtrlHToHistoryDialog(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyCtrlH, 0, tcell.ModCtrl)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelHistoryDialog {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelHistoryDialog", got)
	}
}

func TestActionFromKeyMapsCtrlAltDToAbortDiskUsageScans(t *testing.T) {
	km := defaultKeymap(t)
	ev := tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModAlt)
	got := lookupActionForView(ev, km, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelDiskUsageAbortAll {
		t.Fatalf("actionFromKeyEvent() = %v, want ActionPanelDiskUsageAbortAll", got)
	}
}

func TestActionFromKeyIgnoresAltF(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != "" {
		t.Fatalf("actionFromKeyEvent() = %v, want empty string", got)
	}
}

func TestActionFromKeyIgnoresUnknownKey(t *testing.T) {
	km := defaultKeymap(t)
	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	got := lookupActionForView(event, km, nil, nil, ui.ViewBrowser)
	if got != "" {
		t.Fatalf("actionFromKeyEvent() = %v, want empty string", got)
	}
}

func TestActionFromKeyMapsF3F4AndFilteredViewChord(t *testing.T) {
	km := defaultKeymap(t)
	tests := []struct {
		name string
		ev   *tcell.EventKey
		want string
	}{
		{"F3 view", tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone), keymap.ActionFileView},
		{"F4 edit", tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone), keymap.ActionFileEdit},
		{"M-! filtered", tcell.NewEventKey(tcell.KeyRune, '!', tcell.ModAlt), keymap.ActionMenuFileFilteredView},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lookupActionForView(tt.ev, km, nil, nil, ui.ViewBrowser); got != tt.want {
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
	if got := lookupActionForView(f8, bundle.Global, bundle.Jobs, bundle.Commands, ui.ViewBrowser); got != keymap.ActionFileDelete {
		t.Fatalf("browser F8 = %q, want file.delete", got)
	}
	if got := lookupActionForView(f8, bundle.Global, bundle.Jobs, bundle.Commands, ui.ViewJobs); got != keymap.ActionJobsClearFinished {
		t.Fatalf("jobs view F8 = %q, want jobs.clear-finished", got)
	}
}
