package keymap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
)

func TestAllShortcutTablePathsMatchOverlayRegistry(t *testing.T) {
	overlaySet := make(map[string]struct{}, len(overlayRegistry))
	for _, spec := range overlayRegistry {
		overlaySet[spec.TableName] = struct{}{}
	}
	for _, path := range AllShortcutTablePaths() {
		if path == MainShortcutsTable {
			continue
		}
		if _, ok := overlaySet[path]; !ok {
			t.Fatalf("shortcut table %q missing from overlay registry", path)
		}
	}
	for _, name := range OverlayTableNames() {
		found := false
		for _, path := range AllShortcutTablePaths() {
			if path == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("overlay table %q missing from AllShortcutTablePaths", name)
		}
	}
}

func TestParseKeyRejectsMultiStroke(t *testing.T) {
	_, err := ParseKey("C-x c")
	if err == nil {
		t.Fatal("ParseKey() error = nil, want error")
	}
}

func TestFormatChordCtrlSpace(t *testing.T) {
	ch, err := ParseKey("C-space")
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	if got := FormatChord(ch); got != "Ctrl+Space" {
		t.Fatalf("FormatChord(C-space) = %q, want Ctrl+Space", got)
	}
}

func TestFormatChordAltLetterUppercase(t *testing.T) {
	ch, err := ParseKey("M-m")
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	if got := FormatChord(ch); got != "Alt+M" {
		t.Fatalf("FormatChord(M-m) = %q, want Alt+M", got)
	}
	ch2, err := ParseKey("M-M")
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	if got := FormatChord(ch2); got != "Alt+M" {
		t.Fatalf("FormatChord(M-M) = %q, want Alt+M", got)
	}
}

func TestParseKeyNamedAndModifiers(t *testing.T) {
	tests := []struct {
		input string
		want  Chord
	}{
		{input: "F10", want: Chord{Key: tcell.KeyF10}},
		{input: "S-F6", want: Chord{Key: tcell.KeyF6, Mod: tcell.ModShift}},
		{input: "S-F10", want: Chord{Key: tcell.KeyF10, Mod: tcell.ModShift}},
		{input: "pgup", want: Chord{Key: tcell.KeyPgUp}},
		{input: "C-d", want: Chord{Key: tcell.KeyCtrlD}},
		{input: "C-space", want: Chord{Key: tcell.KeyCtrlSpace}},
		{input: "C-M-d", want: Chord{Key: tcell.KeyCtrlD, Mod: tcell.ModAlt}},
		{input: "M-left", want: Chord{Key: tcell.KeyLeft, Mod: tcell.ModAlt}},
		{input: "M-C-left", want: Chord{Key: tcell.KeyLeft, Mod: tcell.ModAlt | tcell.ModCtrl}},
		{input: "M-C-right", want: Chord{Key: tcell.KeyRight, Mod: tcell.ModAlt | tcell.ModCtrl}},
		{input: "C-up", want: Chord{Key: tcell.KeyUp, Mod: tcell.ModCtrl}},
		{input: "C-down", want: Chord{Key: tcell.KeyDown, Mod: tcell.ModCtrl}},
		{input: "S-tab", want: Chord{Key: tcell.KeyBacktab}},
		{input: "delete", want: Chord{Key: tcell.KeyDelete}},
	}
	for _, tt := range tests {
		got, err := ParseKey(tt.input)
		if err != nil {
			t.Fatalf("ParseKey(%q) error = %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseKey(%q) = %+v, want %+v", tt.input, got, tt.want)
		}
	}
}

func TestBuildDetectsCrossActionConflict(t *testing.T) {
	_, err := Build(map[string][]string{
		ActionAppQuit:     {"F10"},
		ActionAppOpenMenu: {"F10"},
	})
	if err == nil {
		t.Fatal("Build() error = nil, want conflict")
	}
}

func TestBuildDetectsDuplicateWithinAction(t *testing.T) {
	_, err := Build(map[string][]string{
		ActionAppQuit: {"F10", "F10"},
	})
	if err == nil {
		t.Fatal("Build() error = nil, want duplicate")
	}
}

func TestBuildRejectsUnknownAction(t *testing.T) {
	_, err := Build(map[string][]string{
		"not.real.action": {"F1"},
	})
	if err == nil {
		t.Fatal("Build() error = nil, want unknown action")
	}
}

func TestDefaultLookupMatchesSimulationKeys(t *testing.T) {
	m, err := Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}
	tests := []struct {
		ev       *tcell.EventKey
		wantID   string
		wantBool bool
	}{
		{tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone), ActionAppQuit, true},
		{tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModShift), ActionAppQuitImmediate, true},
		{tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModNone), ActionPanelDiskUsageScan, true},
		{tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModCtrl), ActionPanelDiskUsageScan, true},
		{tcell.NewEventKey(tcell.KeyRune, 0x04, tcell.ModCtrl), ActionPanelDiskUsageScan, true},
		{tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModCtrl), ActionPanelDiskUsageScan, true},
		{tcell.NewEventKey(tcell.KeyRune, 'D', tcell.ModCtrl), ActionPanelDiskUsageScan, true},
		{tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModAlt), ActionPanelDiskUsageAbortAll, true},
		{tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModAlt|tcell.ModCtrl), ActionPanelDiskUsageAbortAll, true},
		{tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModAlt), ActionPanelDiskUsageClear, true},
		{tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt|tcell.ModCtrl), ActionNavForward, true},
		{tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt|tcell.ModCtrl), ActionNavBackward, true},
		{tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModAlt), ActionPanelHistoryDialog, true},
		{tcell.NewEventKey(tcell.KeyCtrlH, 0, tcell.ModCtrl), ActionPanelHistoryDialog, true},
		{tcell.NewEventKey(tcell.KeyCtrlF, 0, tcell.ModCtrl), ActionPanelFindDialog, true},
		{tcell.NewEventKey(tcell.KeyCtrlF, 0, tcell.ModAlt), ActionFileFlatten, true},
		{tcell.NewEventKey(tcell.KeyRune, 0x06, tcell.ModCtrl), ActionPanelFindDialog, true},
		{tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone), ActionNavParent, true},
		{tcell.NewEventKey(tcell.KeyRune, '-', tcell.ModNone), ActionPanelUnselectGroup, true},
		{tcell.NewEventKey(tcell.KeyRune, '+', tcell.ModNone), ActionPanelSelectGroup, true},
		{tcell.NewEventKey(tcell.KeyRune, '+', tcell.ModShift), ActionPanelSelectGroup, true},
		{tcell.NewEventKey(tcell.KeyRune, '*', tcell.ModNone), ActionPanelInvertSelection, true},
		{tcell.NewEventKey(tcell.KeyRune, '*', tcell.ModShift), ActionPanelInvertSelection, true},
		{tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone), ActionFileView, true},
		{tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModShift), ActionFileQuickView, true},
		{tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone), ActionAppUserMenu, true},
		{tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModShift), ActionAppUserMenuEdit, true},
		{tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone), ActionFileEdit, true},
		{tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModShift), ActionFileCopyHere, true},
		{tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone), ActionMove, true},
		{tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModShift), ActionFileRename, true},
		{tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone), ActionFileMkdir, true},
		{tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModShift), ActionFileMkdirOpenInOther, true},
		{tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone), ActionFileDelete, true},
		{tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone), ActionFileDelete, true},
		{tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModAlt), ActionPanelExternalBrowser, true},
		{tcell.NewEventKey(tcell.KeyCtrlO, 0, tcell.ModNone), ActionAppDropToShell, true},
		{tcell.NewEventKey(tcell.KeyCtrlO, 0, tcell.ModCtrl), ActionAppDropToShell, true},
		{tcell.NewEventKey(tcell.KeyRune, 0x0f, tcell.ModCtrl), ActionAppDropToShell, true},
		{tcell.NewEventKey(tcell.KeyCtrlO, 0, tcell.ModAlt), ActionPanelToggleSync, true},
		{tcell.NewEventKey(tcell.KeyCtrlO, 0, tcell.ModAlt|tcell.ModCtrl), ActionPanelToggleSync, true},
		{tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModMeta), ActionPanelOpenDirInOther, true},
		{tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModMeta), ActionPanelOpenActivePathInOther, true},
		{tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModMeta|tcell.ModAlt), ActionPanelOpenDirInOther, true},
		{tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift), ActionPanelOpenDirInOther, true},
		{tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift), ActionPanelOpenActivePathInOther, true},
		{tcell.NewEventKey(tcell.KeyRune, ',', tcell.ModAlt|tcell.ModShift), ActionPanelMetaEdit, true},
		{tcell.NewEventKey(tcell.KeyRune, ';', tcell.ModAlt), ActionPanelMetaEdit, true},
		{tcell.NewEventKey(tcell.KeyRune, ',', tcell.ModAlt), ActionPanelMeta, true},
		{tcell.NewEventKey(tcell.KeyCtrlSpace, 0, tcell.ModNone), ActionPanelToggleSplitOrientation, true},
		{tcell.NewEventKey(tcell.KeyCtrlSpace, 0, tcell.ModCtrl), ActionPanelToggleSplitOrientation, true},
		{tcell.NewEventKey(tcell.KeyNUL, 0, tcell.ModNone), ActionPanelToggleSplitOrientation, true},
		{tcell.NewEventKey(tcell.KeyRune, 0, tcell.ModNone), ActionPanelToggleSplitOrientation, true},
		{tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModCtrl), ActionPanelToggleSplitOrientation, true},
	}
	for _, tt := range tests {
		id, ok := m.Lookup(tt.ev)
		if ok != tt.wantBool || id != tt.wantID {
			t.Fatalf("Lookup() = %q %v, want %q %v", id, ok, tt.wantID, tt.wantBool)
		}
	}
}

func TestDefaultJobsOverlayMapsF8ToTerminate(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	id, ok := bundle.Jobs.Lookup(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !ok || id != ActionJobsTerminate {
		t.Fatalf("jobs overlay F8 = %q %v, want jobs.terminate", id, ok)
	}
	id, ok = bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !ok || id != ActionFileDelete {
		t.Fatalf("global F8 = %q %v, want file.delete", id, ok)
	}
}

func TestDefaultOverlayMapsLeftToCloseView(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	left := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
	cases := []struct {
		name string
		m    *Map
		want string
	}{
		{"jobs", bundle.Jobs, ActionJobsClose},
		{"commands", bundle.Commands, ActionCommandsClose},
		{"messages", bundle.Messages, ActionMessagesClose},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := tc.m.Lookup(left)
			if !ok || id != tc.want {
				t.Fatalf("overlay left = %q %v, want %q", id, ok, tc.want)
			}
		})
	}
}

func TestDefaultMessagesOverlayMapsF8ToClear(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	id, ok := bundle.Messages.Lookup(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !ok || id != ActionMessagesClear {
		t.Fatalf("messages overlay F8 = %q %v, want messages.clear", id, ok)
	}
	id, ok = bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !ok || id != ActionFileDelete {
		t.Fatalf("global F8 = %q %v, want file.delete", id, ok)
	}
}

func TestLoadFromPathsUsesDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	bundle, err := LoadFromPaths(config.Paths{ConfigDir: dir, KeybindingsFile: filepath.Join(dir, "absent.toml")})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	// Ctrl+R defaults to jobs.resume — a jobs-view-only chord that
	// lives in the overlay map only after the unification refactor.
	id, ok := bundle.Jobs.Lookup(tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone))
	if !ok || id != ActionJobsResume {
		t.Fatalf("Jobs.Lookup(ctrl-r) = %q %v, want jobs.resume", id, ok)
	}
	id, ok = bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone))
	if !ok || id != ActionRemoteSFTPLink {
		t.Fatalf("Global.Lookup(ctrl-r) = %q %v, want %q", id, ok, ActionRemoteSFTPLink)
	}
	// Alt+Ctrl+R remains panel.refresh on the global map.
	id, ok = bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModAlt))
	if !ok || id != ActionPanelRefresh {
		t.Fatalf("Lookup(alt+ctrl+r) = %q %v, want panel.refresh", id, ok)
	}
}

func TestDialogInputOverlayDefaultsResolveCtrlRAndCtrlD(t *testing.T) {
	dir := t.TempDir()
	bundle, err := LoadFromPaths(config.Paths{ConfigDir: dir, KeybindingsFile: filepath.Join(dir, "absent.toml")})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	if bundle.DialogInput == nil {
		t.Fatal("bundle.DialogInput is nil, want populated overlay")
	}
	cases := []struct {
		name string
		ev   *tcell.EventKey
	}{
		{"ctrl-r", tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone)},
		{"ctrl-d", tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModNone)},
		{"rune-r-ctrl", tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModCtrl)},
		{"rune-d-ctrl", tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModCtrl)},
	}
	for _, tc := range cases {
		id, ok := bundle.DialogInput.Lookup(tc.ev)
		if !ok || id != ActionDialogInputRestoreDefault {
			t.Fatalf("DialogInput.Lookup(%s) = %q %v, want %q", tc.name, id, ok, ActionDialogInputRestoreDefault)
		}
	}
	wordCases := []struct {
		name string
		ev   *tcell.EventKey
		want string
	}{
		{"ctrl-w", tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone), ActionDialogInputKillWordBackward},
		{"alt-b", tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModAlt), ActionDialogInputBackwardWord},
		{"alt-f", tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt), ActionDialogInputForwardWord},
	}
	for _, tc := range wordCases {
		id, ok := bundle.DialogInput.Lookup(tc.ev)
		if !ok || id != tc.want {
			t.Fatalf("DialogInput.Lookup(%s) = %q %v, want %q", tc.name, id, ok, tc.want)
		}
	}
}

func TestDialogInputOverlayRejectsNonInputActions(t *testing.T) {
	dir := t.TempDir()
	keybindings := filepath.Join(dir, "keybindings.toml")
	body := "[dialog.input]\n" +
		"jobs.cancel = [\"C-r\"]\n"
	if err := os.WriteFile(keybindings, []byte(body), 0o600); err != nil {
		t.Fatalf("write keybindings: %v", err)
	}
	_, err := LoadFromPaths(config.Paths{ConfigDir: dir, KeybindingsFile: keybindings})
	if err == nil {
		t.Fatal("LoadFromPaths: want error for non-ui.input.* action in [dialog.input]")
	}
}

func TestRenameDialogOverlayRejectsNonRenameActions(t *testing.T) {
	dir := t.TempDir()
	keybindings := filepath.Join(dir, "keybindings.toml")
	body := "[dialog.rename]\n" +
		"jobs.cancel = [\"C-r\"]\n"
	if err := os.WriteFile(keybindings, []byte(body), 0o600); err != nil {
		t.Fatalf("write keybindings: %v", err)
	}
	_, err := LoadFromPaths(config.Paths{ConfigDir: dir, KeybindingsFile: keybindings})
	if err == nil {
		t.Fatal("LoadFromPaths: want error for invalid action in [dialog.rename]")
	}
}

func TestMkdirDialogOverlayRejectsNonMkdirActions(t *testing.T) {
	dir := t.TempDir()
	keybindings := filepath.Join(dir, "keybindings.toml")
	body := "[dialog.mkdir]\n" +
		"jobs.cancel = [\"C-r\"]\n"
	if err := os.WriteFile(keybindings, []byte(body), 0o600); err != nil {
		t.Fatalf("write keybindings: %v", err)
	}
	_, err := LoadFromPaths(config.Paths{ConfigDir: dir, KeybindingsFile: keybindings})
	if err == nil {
		t.Fatal("LoadFromPaths: want error for invalid action in [dialog.mkdir]")
	}
}

func TestParseKeybindingsRejectsLegacyActionKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	if err := writeFile(path, `[action_keys]
"app.quit" = ["F10"]
`); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFromPaths(config.Paths{KeybindingsFile: path})
	if err == nil {
		t.Fatal("LoadFromPaths: want error for legacy [action_keys] section")
	}
}

func TestDefaultBundleRenameDialogOverlayF2F3(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	id, ok := bundle.RenameDialog.Lookup(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	if !ok || id != ActionFileRenameOpenSanitize {
		t.Fatalf("RenameDialog F2 = %q %v, want %q", id, ok, ActionFileRenameOpenSanitize)
	}
	id, ok = bundle.RenameDialog.Lookup(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if !ok || id != ActionFileRenameOpenSlugify {
		t.Fatalf("RenameDialog F3 = %q %v, want %q", id, ok, ActionFileRenameOpenSlugify)
	}
}

func TestDefaultBundleMkdirDialogOverlayF7(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	id, ok := bundle.MkdirDialog.Lookup(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone))
	if !ok || id != ActionFileMkdirExtractCommonName {
		t.Fatalf("MkdirDialog F7 = %q %v, want %q", id, ok, ActionFileMkdirExtractCommonName)
	}
}

func TestDefaultBundleBookmarkDialogOverlayF8(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	id, ok := bundle.BookmarkDialog.Lookup(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !ok || id != ActionBookmarkDelete {
		t.Fatalf("BookmarkDialog F8 = %q %v, want %q", id, ok, ActionBookmarkDelete)
	}
}

func TestDefaultBundleHistoryDialogOverlayF5(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	if got := bundle.HistoryDialog.MenuBindingLabel(ActionPanelHistoryBothPanels); got != "F5" {
		t.Fatalf("HistoryDialog MenuBindingLabel = %q, want F5", got)
	}
	id, ok := bundle.HistoryDialog.Lookup(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if !ok || id != ActionPanelHistoryBothPanels {
		t.Fatalf("HistoryDialog F5 = %q %v, want %q", id, ok, ActionPanelHistoryBothPanels)
	}
}

func TestDefaultBundleFlattenDialogOverlayF5F6(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	if got := bundle.FlattenDialog.MenuBindingLabel(ActionFlattenDestinationActive); got != "F5" {
		t.Fatalf("FlattenDialog Active MenuBindingLabel = %q, want F5", got)
	}
	id, ok := bundle.FlattenDialog.Lookup(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if !ok || id != ActionFlattenDestinationActive {
		t.Fatalf("FlattenDialog F5 = %q %v, want %q", id, ok, ActionFlattenDestinationActive)
	}
	if got := bundle.FlattenDialog.MenuBindingLabel(ActionFlattenDestinationInactive); got != "F6" {
		t.Fatalf("FlattenDialog Inactive MenuBindingLabel = %q, want F6", got)
	}
	id, ok = bundle.FlattenDialog.Lookup(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone))
	if !ok || id != ActionFlattenDestinationInactive {
		t.Fatalf("FlattenDialog F6 = %q %v, want %q", id, ok, ActionFlattenDestinationInactive)
	}
}

func TestDefaultBundleFindDialogOverlayF5CtrlA(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	if got := bundle.FindDialog.MenuBindingLabel(ActionFindUnselectAll); got != "F4" {
		t.Fatalf("FindDialog UnselectAll MenuBindingLabel = %q, want F4", got)
	}
	id, ok := bundle.FindDialog.Lookup(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))
	if !ok || id != ActionFindUnselectAll {
		t.Fatalf("FindDialog F4 = %q %v, want %q", id, ok, ActionFindUnselectAll)
	}
	if got := bundle.FindDialog.MenuBindingLabel(ActionFindSelectAll); got != "F5" {
		t.Fatalf("FindDialog MenuBindingLabel = %q, want F5", got)
	}
	id, ok = bundle.FindDialog.Lookup(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if !ok || id != ActionFindSelectAll {
		t.Fatalf("FindDialog F5 = %q %v, want %q", id, ok, ActionFindSelectAll)
	}
	id, ok = bundle.FindDialog.Lookup(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl))
	if !ok || id != ActionFindSelectAll {
		t.Fatalf("FindDialog Ctrl+A = %q %v, want %q", id, ok, ActionFindSelectAll)
	}
	id, ok = bundle.FindDialog.Lookup(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone))
	if !ok || id != ActionFindSelectGroup {
		t.Fatalf("FindDialog F6 = %q %v, want %q", id, ok, ActionFindSelectGroup)
	}
	id, ok = bundle.FindDialog.Lookup(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone))
	if !ok || id != ActionFindUnselectGroup {
		t.Fatalf("FindDialog F7 = %q %v, want %q", id, ok, ActionFindUnselectGroup)
	}
}

func TestParseKeyAltBang(t *testing.T) {
	if _, err := ParseKey("M-!"); err != nil {
		t.Fatalf("ParseKey(M-!): %v", err)
	}
}

func TestMenuBindingLabelUsesDefaultsAndPreferredKey(t *testing.T) {
	m, err := Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}
	if got := m.MenuBindingLabel(ActionPanelSortDialog); got != "C-s" {
		t.Fatalf("sort dialog = %q, want C-s", got)
	}
	if got := m.MenuBindingLabel(ActionRemoteSFTPLink); got != "C-r" {
		t.Fatalf("SFTP dialog = %q, want C-r", got)
	}
	if got := m.MenuBindingLabel(ActionPanelToggleHidden); got != "M-." {
		t.Fatalf("toggle hidden = %q, want M-.", got)
	}
	if got := m.MenuBindingLabel(ActionPanelCycleListingFormat); got != "M-t" {
		t.Fatalf("cycle listing format = %q, want M-t", got)
	}
	if got := m.MenuBindingLabel(ActionPanelToggleZoomActivePanel); got != "M-z" {
		t.Fatalf("toggle zoom active panel = %q, want M-z", got)
	}
	if got := m.MenuBindingLabel(ActionPanelHistoryDialog); got != "M-h" {
		t.Fatalf("history = %q, want preferred M-h", got)
	}
	var nilMap *Map
	if got := nilMap.MenuBindingLabel(ActionPanelRefresh); got != "M-C-r" {
		t.Fatalf("nil map refresh = %q, want default M-C-r", got)
	}
	if got := m.MenuBindingLabel(ActionFileView); got != "F3" {
		t.Fatalf("menu file view = %q, want F3", got)
	}
	if got := m.MenuBindingLabel(ActionFileQuickView); got != "S-F3" {
		t.Fatalf("menu quick view = %q, want S-F3", got)
	}
}

func TestMenuBindingLabelUsesUserRemap(t *testing.T) {
	m, err := Build(map[string][]string{
		ActionPanelSortDialog: {"F2"},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := m.MenuBindingLabel(ActionPanelSortDialog); got != "F2" {
		t.Fatalf("sort dialog = %q, want F2", got)
	}
}

func TestLoadFromPathsMergesUserFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	content := `
[main]
app.quit = ["F12"]
panel.refresh = ["F11"]
`
	if err := writeFile(path, content); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	bundle, err := LoadFromPaths(config.Paths{KeybindingsFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	m := bundle.Global
	id, ok := m.Lookup(tcell.NewEventKey(tcell.KeyF12, 0, tcell.ModNone))
	if !ok || id != ActionAppQuit {
		t.Fatalf("quit = %q %v, want app.quit", id, ok)
	}
	_, okF10 := m.Lookup(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if okF10 {
		t.Fatal("F10 should not quit after user remapped quit to F12")
	}
	id, ok = m.Lookup(tcell.NewEventKey(tcell.KeyF11, 0, tcell.ModNone))
	if !ok || id != ActionPanelRefresh {
		t.Fatalf("F11 = %q %v, want panel.refresh", id, ok)
	}
}

func TestLoadFromPathsRejectsUnknownTOMLField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	if err := writeFile(path, `oops = true`); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	_, err := LoadFromPaths(config.Paths{KeybindingsFile: path})
	if err == nil {
		t.Fatal("LoadFromPaths() error = nil, want unknown field")
	}
}

func TestConfigLoadRejectsShortcutTablesInConfigToml(t *testing.T) {
	dir := t.TempDir()
	for _, table := range AllShortcutTablePaths() {
		t.Run(table, func(t *testing.T) {
			configPath := filepath.Join(dir, table+".toml")
			content := shortcutTableFixtureInConfig(table)
			if err := writeFile(configPath, content); err != nil {
				t.Fatalf("writeFile: %v", err)
			}
			_, err := config.LoadFromPaths(config.Paths{ConfigFile: configPath})
			if err == nil {
				t.Fatalf("config.LoadFromPaths() error = nil, want unknown field for [%s]", table)
			}
			if !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("config.LoadFromPaths() error = %v, want unknown field", err)
			}
		})
	}
}

func shortcutTableFixtureInConfig(table string) string {
	switch table {
	case JobsShortcutsTable:
		return "theme = \"default\"\n[jobs]\n\"jobs.clear-finished\" = [\"C-k\"]\n"
	case MainShortcutsTable:
		return "theme = \"default\"\n[main]\n\"app.quit\" = [\"F12\"]\n"
	case DialogInputShortcutsTable:
		return "theme = \"default\"\n[dialog.input]\n\"ui.input.backward-word\" = [\"M-B\"]\n"
	case DialogRenameShortcutsTable:
		return "theme = \"default\"\n[dialog.rename]\n\"file.rename.open-sanitize\" = [\"C-s\"]\n"
	default:
		return "theme = \"default\"\n[" + table + "]\n\"noop.action\" = [\"F1\"]\n"
	}
}

// TestLoadFromPathsUsesKeybindingsJobsShortcuts verifies jobs-view overlay
// shortcuts load from keybindings.toml ([jobs] cannot live in config.toml
// because that table holds JobsConfig settings).
func TestLoadFromPathsUsesKeybindingsJobsShortcuts(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := writeFile(configPath, `theme = "default"
[jobs]
keep_finished = 25
`); err != nil {
		t.Fatalf("writeFile config: %v", err)
	}
	keybindingsPath := filepath.Join(dir, "keybindings.toml")
	if err := writeFile(keybindingsPath, `[jobs]
"jobs.clear-finished" = ["C-k"]
`); err != nil {
		t.Fatalf("writeFile keybindings: %v", err)
	}

	bundle, err := LoadFromPaths(config.Paths{ConfigFile: configPath, KeybindingsFile: keybindingsPath})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	id, ok := bundle.Jobs.Lookup(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone))
	if !ok || id != ActionJobsClearFinished {
		t.Fatalf("C-k -> %q %v, want jobs.clear-finished from keybindings.toml", id, ok)
	}
	// F8 is now jobs.terminate by default; clearing the clear-finished binding does not free F8.
	id, ok = bundle.Jobs.Lookup(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !ok || id != ActionJobsTerminate {
		t.Fatalf("F8 jobs overlay -> %q %v, want jobs.terminate (default unaffected by clear-finished rebind)", id, ok)
	}
}

// TestLoadFromPathsRejectsNonJobsActionInJobsOverlay guarantees the jobs.*
// restriction is enforced in keybindings.toml.
func TestLoadFromPathsRejectsNonJobsActionInJobsOverlayFromKeybindings(t *testing.T) {
	dir := t.TempDir()
	keybindingsPath := filepath.Join(dir, "keybindings.toml")
	if err := writeFile(keybindingsPath, `[jobs]
"file.delete" = ["F8"]
`); err != nil {
		t.Fatalf("writeFile keybindings: %v", err)
	}
	if _, err := LoadFromPaths(config.Paths{KeybindingsFile: keybindingsPath}); err == nil {
		t.Fatal("expected error: non-jobs.* action under [jobs]")
	}
}

// TestDefaultBundleJobsOpenGlobalRestJobsOverlay verifies semantics:
// jobs.open is bound on the global map ([main]); other jobs.*
// defaults bind only in the overlay ([jobs]).
func TestDefaultBundleJobsOpenGlobalRestJobsOverlay(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}

	id, ok := bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModAlt))
	if !ok || id != ActionJobsOpen {
		t.Fatalf("global Alt+J = %q %v, want jobs.open", id, ok)
	}
	id, ok = bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModNone))
	if !ok || id != ActionFileQuickViewPreviewPageDown {
		t.Fatalf("global Ctrl+J = %q %v, want file.quick-view.preview-page-down", id, ok)
	}
	id, ok = bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone))
	if !ok || id != ActionFileQuickViewPreviewPageUp {
		t.Fatalf("global Ctrl+K = %q %v, want file.quick-view.preview-page-up", id, ok)
	}
	id, ok = bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone))
	if !ok || id != ActionJobsAnswerBlocker {
		t.Fatalf("global Ctrl+Q = %q %v, want jobs.answer-blocker", id, ok)
	}
	id, ok = bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModAlt))
	if !ok || id != ActionJobsAnswerBlocker {
		t.Fatalf("global Alt+Q = %q %v, want jobs.answer-blocker", id, ok)
	}
	if _, ok := bundle.Jobs.Lookup(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModAlt)); ok {
		t.Fatal("Alt+J must not bind in jobs overlay (jobs.open is global-only)")
	}

	cases := []struct {
		ev       *tcell.EventKey
		wantID   string
		humanKey string
	}{
		{tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone), ActionJobsCancel, "Ctrl+C"},
		{tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone), ActionJobsPause, "Ctrl+P"},
		{tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone), ActionJobsResume, "Ctrl+R"},
		{tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModCtrl), ActionJobsQueueUp, "Ctrl+Up"},
		{tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModCtrl), ActionJobsQueueDown, "Ctrl+Down"},
		{tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone), ActionJobsTerminate, "F8"},
		{tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModShift), ActionJobsKill, "Shift+F8"},
	}
	for _, tc := range cases {
		id, ok := bundle.Jobs.Lookup(tc.ev)
		if !ok || id != tc.wantID {
			t.Fatalf("overlay %s = %q %v, want %q", tc.humanKey, id, ok, tc.wantID)
		}
		if got, ok := bundle.Global.Lookup(tc.ev); ok && got == tc.wantID {
			t.Fatalf("global %s should not bind %s (overlay-only)", tc.humanKey, tc.wantID)
		}
	}

	if id, ok := bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)); !ok || id != ActionFileDelete {
		t.Fatalf("global F8 = %q %v, want file.delete", id, ok)
	}
}

// TestExplicitJobsOpenInMainOverridesDefaultChord verifies that an
// explicit [main] binding for jobs.open replaces the built-in C-j.
func TestExplicitJobsOpenInMainOverridesDefaultChord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	if err := writeFile(path, `[main]
"jobs.open" = ["F11"]
`); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	bundle, err := LoadFromPaths(config.Paths{KeybindingsFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	id, ok := bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyF11, 0, tcell.ModNone))
	if !ok || id != ActionJobsOpen {
		t.Fatalf("F11 = %q %v, want jobs.open from explicit [main]", id, ok)
	}
	if _, ok := bundle.Global.Lookup(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModAlt)); ok {
		t.Fatal("Alt+J should not bind: [main] replaced default jobs.open chords")
	}
}

func TestEncodeDefaultStubRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stub.toml")
	if err := WriteDefaultStub(path); err != nil {
		t.Fatalf("WriteDefaultStub: %v", err)
	}
	bundle, err := LoadFromPaths(config.Paths{KeybindingsFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths(stub): %v", err)
	}
	def, err := Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	for _, ev := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone),
	} {
		a1, ok1 := bundle.Global.Lookup(ev)
		a2, ok2 := def.Lookup(ev)
		if ok1 != ok2 || a1 != a2 {
			t.Fatalf("stub vs default: ev %v -> %q %v vs %q %v", ev, a1, ok1, a2, ok2)
		}
	}
	defBundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	f8 := tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)
	j1, okJ1 := bundle.Jobs.Lookup(f8)
	j2, okJ2 := defBundle.Jobs.Lookup(f8)
	if okJ1 != okJ2 || j1 != j2 {
		t.Fatalf("stub jobs overlay F8 -> %q %v vs default bundle %q %v", j1, okJ1, j2, okJ2)
	}
}

func TestLoadFromPathsMergesJobsShortcuts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	content := `
[jobs]
jobs.clear-finished = ["C-k"]
`
	if err := writeFile(path, content); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	bundle, err := LoadFromPaths(config.Paths{KeybindingsFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	id, ok := bundle.Jobs.Lookup(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone))
	if !ok || id != ActionJobsClearFinished {
		t.Fatalf("jobs overlay C-k = %q %v", id, ok)
	}
	// F8 is jobs.terminate by default; rebinding clear-finished does not free F8.
	id, okF8 := bundle.Jobs.Lookup(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !okF8 || id != ActionJobsTerminate {
		t.Fatalf("jobs overlay F8 = %q %v, want jobs.terminate", id, okF8)
	}
}

func TestLoadFromPathsRejectsNonJobsActionInJobsOverlay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	content := `
[jobs]
file.delete = ["F8"]
`
	if err := writeFile(path, content); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	_, err := LoadFromPaths(config.Paths{KeybindingsFile: path})
	if err == nil {
		t.Fatal("expected error for non-jobs action under [jobs]")
	}
}

func TestReadMainShortcutsReturnsNilWhenMissing(t *testing.T) {
	keys, err := ReadMainShortcuts(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("ReadMainShortcuts() error = %v", err)
	}
	if keys != nil {
		t.Fatalf("ReadMainShortcuts = %v, want nil for missing file", keys)
	}
}

func TestReadMainShortcutsReturnsNilWhenTableAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	if err := writeFile(path, `theme = "default"`); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	keys, err := ReadMainShortcuts(path)
	if err != nil {
		t.Fatalf("ReadMainShortcuts() error = %v", err)
	}
	if keys != nil {
		t.Fatalf("ReadMainShortcuts = %v, want nil when [main] missing", keys)
	}
}

func TestReadMainShortcutsExtractsDottedActionIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	if err := writeFile(path, `[main]
"app.quit" = ["F12"]
"panel.refresh" = ["F2", "C-r"]
`); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	keys, err := ReadMainShortcuts(path)
	if err != nil {
		t.Fatalf("ReadMainShortcuts() error = %v", err)
	}
	if got, want := keys["app.quit"], []string{"F12"}; !equalStringSlice(got, want) {
		t.Fatalf("app.quit = %v, want %v", got, want)
	}
	if got, want := keys["panel.refresh"], []string{"F2", "C-r"}; !equalStringSlice(got, want) {
		t.Fatalf("panel.refresh = %v, want %v", got, want)
	}
}

func TestReadJobsShortcutsReturnsNilWhenMissing(t *testing.T) {
	keys, err := ReadJobsShortcuts(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("ReadJobsShortcuts() error = %v", err)
	}
	if keys != nil {
		t.Fatalf("ReadJobsShortcuts = %v, want nil for missing file", keys)
	}
}

func TestReadJobsShortcutsExtractsTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	if err := writeFile(path, `[jobs]
"jobs.clear-finished" = ["C-k"]
`); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	keys, err := ReadJobsShortcuts(path)
	if err != nil {
		t.Fatalf("ReadJobsShortcuts() error = %v", err)
	}
	if got, want := keys["jobs.clear-finished"], []string{"C-k"}; !equalStringSlice(got, want) {
		t.Fatalf("jobs.clear-finished = %v, want %v", got, want)
	}
	plainKeys, err := ReadMainShortcuts(path)
	if err != nil {
		t.Fatalf("ReadMainShortcuts() error = %v", err)
	}
	if plainKeys != nil {
		t.Fatalf("ReadMainShortcuts = %v, want nil when only [jobs] present", plainKeys)
	}
}

func TestReadDialogInputShortcutsResolvesNestedTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	if err := writeFile(path, `[dialog.input]
"ui.input.forward-word" = ["M-f", "C-M-f"]
`); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	keys, err := ReadDialogInputShortcuts(path)
	if err != nil {
		t.Fatalf("ReadDialogInputShortcuts() error = %v", err)
	}
	if got, want := keys["ui.input.forward-word"], []string{"M-f", "C-M-f"}; !equalStringSlice(got, want) {
		t.Fatalf("ui.input.forward-word = %v, want %v", got, want)
	}
}

func TestReadDialogRenameShortcutsExtractsTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	if err := writeFile(path, `[dialog.rename]
"file.rename.open-slugify" = ["C-g"]
`); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	keys, err := ReadDialogRenameShortcuts(path)
	if err != nil {
		t.Fatalf("ReadDialogRenameShortcuts() error = %v", err)
	}
	if got, want := keys["file.rename.open-slugify"], []string{"C-g"}; !equalStringSlice(got, want) {
		t.Fatalf("file.rename.open-slugify = %v, want %v", got, want)
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMenuBindingLabelStableAcrossCalls(t *testing.T) {
	m, err := Build(DefaultCompareOverlayKeys())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	first := m.MenuBindingLabel(ActionCompareClose)
	if first != "Esc" {
		t.Fatalf("first label = %q, want Esc", first)
	}
	for i := 0; i < 100; i++ {
		if got := m.MenuBindingLabel(ActionCompareClose); got != first {
			t.Fatalf("iteration %d: label = %q, want %q", i, got, first)
		}
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
