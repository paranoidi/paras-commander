package menu

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

func TestMenuDefinitionsHaveShortcutKeys(t *testing.T) {
	for _, menu := range Definitions() {
		if menu.Shortcut == 0 {
			t.Fatalf("menu %q has no shortcut", menu.Label)
		}
	}
}

func TestMenuDefinitionShortcutsAreUnique(t *testing.T) {
	seen := make(map[rune]string)
	for _, menu := range Definitions() {
		if label, ok := seen[menu.Shortcut]; ok {
			t.Fatalf("shortcut %q used by both %q and %q", menu.Shortcut, label, menu.Label)
		}
		seen[menu.Shortcut] = menu.Label
	}
}

func TestMenuItemsHaveShortcutKeys(t *testing.T) {
	for _, menu := range Definitions() {
		t.Run(menu.Label, func(t *testing.T) {
			for _, item := range menu.Items {
				if item.Separator {
					continue
				}
				if item.Shortcut == 0 {
					t.Fatalf("%s menu item %q has no shortcut", menu.Label, item.Label)
				}
			}
		})
	}
}

func TestMenuShortcutKeysAreUniqueWithinMenu(t *testing.T) {
	for _, menu := range Definitions() {
		t.Run(menu.Label, func(t *testing.T) {
			seen := make(map[rune]string)
			for _, item := range menu.Items {
				if item.Separator {
					continue
				}
				if label, ok := seen[item.Shortcut]; ok {
					t.Fatalf("shortcut %q is used by both %q and %q", item.Shortcut, label, item.Label)
				}
				seen[item.Shortcut] = item.Label
			}
		})
	}
}

func TestFileMenuItemsHaveShortcutKeys(t *testing.T) {
	fileMenu := Definitions()[DefaultIndex()]
	for _, item := range fileMenu.Items {
		if item.Separator {
			continue
		}
		if item.Shortcut == 0 {
			t.Fatalf("file menu item %q has no shortcut", item.Label)
		}
	}
}

func TestFileMenuShortcutExceptions(t *testing.T) {
	tests := map[string]rune{
		"Extract":          'x',
		"View":             'V',
		"Chmod":            'h',
		"Chown":            'o',
		"Chattr":           't',
		"Select group":     'g',
		"Unselect group":   'n',
		"Invert selection": 'I',
		"Exit":             'i',
		"Flatten":          'F',
	}

	fileMenu := Definitions()[DefaultIndex()]
	for label, want := range tests {
		t.Run(label, func(t *testing.T) {
			for _, item := range fileMenu.Items {
				if item.Label == label {
					if item.Shortcut != want {
						t.Fatalf("shortcut = %q, want %q", item.Shortcut, want)
					}
					return
				}
			}
			t.Fatalf("file menu item %q not found", label)
		})
	}
}

func TestNonPanelMenusUseScopeNoneSentinel(t *testing.T) {
	for _, def := range Definitions() {
		if def.ID == TopPanelLeft || def.ID == TopPanelRight {
			if def.PanelScope != PanelScopePrimary && def.PanelScope != PanelScopeSecondary {
				t.Fatalf("%s: want PanelScopePrimary or PanelScopeSecondary, got %d", def.ID, def.PanelScope)
			}
			continue
		}
		if def.PanelScope != PanelScopeNone {
			t.Fatalf("%s must use PanelScopeNone (%d), got %d (omit/zero clashes with PanelScopePrimary)", def.ID, PanelScopeNone, def.PanelScope)
		}
	}
	for _, def := range DefinitionsJobs() {
		if def.PanelScope != PanelScopeNone {
			t.Fatalf("jobs menu %s: want PanelScopeNone=%d, got %d", def.ID, PanelScopeNone, def.PanelScope)
		}
	}
}

func TestMenuItemsHaveStableActions(t *testing.T) {
	checkDefs := func(t *testing.T, defs []Definition, ctx string) {
		t.Helper()
		for _, def := range defs {
			for _, item := range def.Items {
				if item.Separator {
					if item.Action != "" {
						t.Fatalf("%s menu %s: separator must have empty Action", ctx, def.ID)
					}
					continue
				}
				if item.Action == "" {
					t.Fatalf("%s menu %s item %q has empty Action", ctx, def.ID, item.Label)
				}
			}
		}
	}
	t.Run("browser", func(t *testing.T) {
		checkDefs(t, Definitions(), "browser")
	})
	t.Run("jobs", func(t *testing.T) {
		checkDefs(t, DefinitionsJobs(), "jobs")
	})
}

func assertMenuItemKeyLabels(t *testing.T, def *Definition, want map[string]string) {
	t.Helper()
	for _, item := range def.Items {
		if item.Separator {
			continue
		}
		w, ok := want[item.Label]
		if !ok {
			continue
		}
		if item.KeyLabel != w {
			t.Fatalf("%s / %q KeyLabel = %q, want %q", def.Label, item.Label, item.KeyLabel, w)
		}
	}
}

func TestBrowserDefinitionsFillsMenuKeyLabels(t *testing.T) {
	km, err := keymap.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	defs := BrowserDefinitions(km, false)

	var left, file, cmd, display *Definition
	for i := range defs {
		switch defs[i].ID {
		case TopPanelLeft:
			left = &defs[i]
		case TopFile:
			file = &defs[i]
		case TopCommand:
			cmd = &defs[i]
		case TopDisplay:
			display = &defs[i]
		}
	}
	if left == nil || file == nil || cmd == nil || display == nil {
		t.Fatalf("missing menu: left=%v file=%v cmd=%v display=%v", left != nil, file != nil, cmd != nil, display != nil)
	}

	assertMenuItemKeyLabels(t, left, map[string]string{
		"Quick view":    "S-F3",
		"Sort...":       "C-s",
		"Toggle hidden": "M-.",
		"Refresh":       "M-C-r",
		"Disk usage":    "C-d",
		"History...":    "M-h",
	})
	assertMenuItemKeyLabels(t, file, map[string]string{
		"View":             "F3",
		"Edit":             "F4",
		"Copy":             "F5",
		"Rename/Move":      "F6",
		"Mkdir":            "F7",
		"Select group":     "+",
		"Unselect group":   "-",
		"Invert selection": "*",
		"Exit":             "F10",
	})
	for _, item := range file.Items {
		if item.Label == "Delete" {
			if item.KeyLabel != "F8" && item.KeyLabel != "delete" {
				t.Fatalf("Delete KeyLabel = %q, want F8 or delete", item.KeyLabel)
			}
			break
		}
	}
	assertMenuItemKeyLabels(t, cmd, map[string]string{
		"Bookmarks":    "C-g",
		"Add bookmark": "C-b",
		"Refresh":      "M-C-r",
	})
	assertMenuItemKeyLabels(t, display, map[string]string{
		"Commands": "M-c",
		"Messages": "M-m",
		"Jobs":     "M-j",
	})
}

func TestJobsDefinitionsFillsMenuKeyLabels(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	defs := JobsDefinitions(bundle.Global, bundle.Jobs)
	if defs[0].ID != TopJobs {
		t.Fatalf("first menu = %s, want TopJobs", defs[0].ID)
	}
	want := map[string]string{
		"Terminate job":      "F8",
		"Kill job":           "S-F8",
		"Cancel job":         "C-c",
		"Pause queued job":   "C-p",
		"Resume paused job":  "C-r",
		"Move up in queue":   "C-up",
		"Move down in queue": "C-down",
		"Clear finished":     "",
		"Back to file view":  "left",
	}
	for _, item := range defs[0].Items {
		w := want[item.Label]
		if item.KeyLabel != w {
			t.Fatalf("%q KeyLabel = %q, want %q", item.Label, item.KeyLabel, w)
		}
	}
	var display, file *Definition
	for i := range defs {
		switch defs[i].ID {
		case TopDisplay:
			display = &defs[i]
		case TopFile:
			file = &defs[i]
		}
	}
	if display == nil {
		t.Fatal("jobs view menus missing Display")
	}
	assertMenuItemKeyLabels(t, display, map[string]string{
		"Commands": "M-c",
		"Messages": "M-m",
		"Jobs":     "M-j",
	})
	if file == nil {
		t.Fatal("jobs view menus missing File")
	}
	if file.Items[0].KeyLabel != "F10" {
		t.Fatalf("jobs File Exit KeyLabel = %q, want F10", file.Items[0].KeyLabel)
	}
}

func TestMessagesDefinitionsFillsMenuKeyLabels(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	defs := MessagesDefinitions(bundle.Global, bundle.Messages)
	if defs[0].ID != TopMessages {
		t.Fatalf("first menu = %s, want TopMessages", defs[0].ID)
	}
	want := map[string]string{
		"Clear messages":    "F8",
		"Back to file view": "left",
	}
	for _, item := range defs[0].Items {
		w := want[item.Label]
		if item.KeyLabel != w {
			t.Fatalf("%q KeyLabel = %q, want %q", item.Label, item.KeyLabel, w)
		}
	}
}

func TestAuxiliaryViewDefinitionsIncludeDisplay(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	for name, defs := range map[string][]Definition{
		"jobs":     JobsDefinitions(bundle.Global, bundle.Jobs),
		"commands": CommandsDefinitions(bundle.Global, bundle.Commands),
		"messages": MessagesDefinitions(bundle.Global, bundle.Messages),
	} {
		t.Run(name, func(t *testing.T) {
			var display *Definition
			for i := range defs {
				if defs[i].ID == TopDisplay {
					display = &defs[i]
					break
				}
			}
			if display == nil {
				t.Fatalf("%s view menus missing Display", name)
			}
			if display.Shortcut != 'd' {
				t.Fatalf("Display shortcut = %q, want d", display.Shortcut)
			}
			assertMenuItemKeyLabels(t, display, map[string]string{
				"Commands": "M-c",
				"Messages": "M-m",
				"Jobs":     "M-j",
			})
		})
	}
}

func TestLeftAndSecondaryPanelMenusShareItemActions(t *testing.T) {
	defs := Definitions()
	left, right := defs[0], defs[len(defs)-1]
	if left.ID != TopPanelLeft || right.ID != TopPanelRight {
		t.Fatalf("expected Left then Right outer menus: got %s / %s", left.ID, right.ID)
	}
	if len(left.Items) != len(right.Items) {
		t.Fatalf("left len=%d right len=%d", len(left.Items), len(right.Items))
	}
	for i := range left.Items {
		if left.Items[i].Separator != right.Items[i].Separator {
			t.Fatalf("item %d separator mismatch", i)
		}
		if left.Items[i].Separator {
			continue
		}
		if left.Items[i].Action != right.Items[i].Action {
			t.Fatalf("item %d: action %q vs %q", i, left.Items[i].Action, right.Items[i].Action)
		}
	}
}

func TestOptionsMenuKeepsThemeChoicesOutOfPulldown(t *testing.T) {
	options := Definitions()[4]
	if options.ID != TopOptions {
		t.Fatalf("index 3 ID = %q, want TopOptions", options.ID)
	}
	if options.Label != "Options" {
		t.Fatalf("index 3 label = %q, want Options", options.Label)
	}
	items := options.Items
	if len(items) != 3 {
		t.Fatalf("options items len = %d, want configuration, calibrate debounce, and theme entries", len(items))
	}
	if items[0].Action != keymap.ActionUIOpenConfig || items[1].Action != keymap.ActionUICalibrateDebounce || items[2].Action != keymap.ActionUIOpenTheme {
		t.Fatalf("unexpected Options actions: %+v / %+v / %+v", items[0].Action, items[1].Action, items[2].Action)
	}
}

func TestFunctionKeysFilePreviewStylePickerShowsEnterSave(t *testing.T) {
	t.Parallel()
	keys := FunctionKeysFilePreviewStylePicker()
	if len(keys) != 3 {
		t.Fatalf("FunctionKeysFilePreviewStylePicker len = %d, want Esc + Enter Save + F10", len(keys))
	}
	if keys[0] != FooterEscClose {
		t.Fatalf("footer[0] = %+v, want Esc Close", keys[0])
	}
	if keys[1].Key != tcell.KeyEnter || keys[1].KeyLabel != "Enter" || keys[1].Hint != "Save" {
		t.Fatalf("footer[1] = %+v, want Enter Save", keys[1])
	}
	if keys[2].Key != tcell.KeyF10 || keys[2].Hint != "Quit" {
		t.Fatalf("footer[2] = %+v, want F10 Quit", keys[2])
	}
}

func TestFunctionKeysFilePreviewViewShowsStyleF9(t *testing.T) {
	t.Parallel()
	keys := FunctionKeysFilePreviewView()
	if len(keys) != 4 {
		t.Fatalf("FunctionKeysFilePreviewView len = %d, want Esc + F4 Edit + F9 Style + F10", len(keys))
	}
	var f4, f9 *FunctionKey
	for i := range keys {
		switch keys[i].Key {
		case tcell.KeyF4:
			f4 = &keys[i]
		case tcell.KeyF9:
			f9 = &keys[i]
		}
	}
	if f4 == nil || f4.Hint != "Edit" {
		t.Fatalf("fullscreen file preview footer must advertise F4 Edit, got %+v", keys)
	}
	if f9 == nil {
		t.Fatalf("fullscreen file preview footer must advertise F9 Style, got %+v", keys)
	}
	if f9.Hint != "Style" {
		t.Fatalf("F9 hint = %q, want Style", f9.Hint)
	}
}

func TestFunctionKeysSelectionsStripView(t *testing.T) {
	t.Parallel()
	keys := FunctionKeysSelectionsStripView("C-u")
	if len(keys) != 3 {
		t.Fatalf("len = %d, want F1 + clear + F10", len(keys))
	}
	if keys[0].KeyLabel != "F1" || keys[0].Hint != "Help" {
		t.Fatalf("F1 = %+v", keys[0])
	}
	if keys[1].KeyLabel != "C-u" || keys[1].Hint != "Unselect all" {
		t.Fatalf("clear selection = %+v", keys[1])
	}
	if keys[2].KeyLabel != "F10" || keys[2].Hint != "Quit" {
		t.Fatalf("F10 = %+v", keys[2])
	}
	for _, fk := range keys {
		if fk.Key == tcell.KeyEsc || fk.Key == tcell.KeyF9 || fk.Key == tcell.KeyF4 {
			t.Fatalf("strip footer must not list Esc/F9/F4, got %+v", keys)
		}
	}
	empty := FunctionKeysSelectionsStripView("")
	if len(empty) != 2 {
		t.Fatalf("empty clear label len = %d, want F1 + F10 only", len(empty))
	}
}
