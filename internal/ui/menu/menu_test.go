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
		"View":             'V',
		"View file...":     'w',
		"Chmod":            'h',
		"Relative symlink": 'k',
		"Edit symlink":     'y',
		"Chown":            'o',
		"Chattr":           't',
		"Select group":     'g',
		"Unselect group":   'n',
		"Exit":             'x',
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
			if def.PanelScope != PanelScopeLeft && def.PanelScope != PanelScopeRight {
				t.Fatalf("%s: want PanelScopeLeft or PanelScopeRight, got %d", def.ID, def.PanelScope)
			}
			continue
		}
		if def.PanelScope != PanelScopeNone {
			t.Fatalf("%s must use PanelScopeNone (%d), got %d (omit/zero clashes with PanelScopeLeft)", def.ID, PanelScopeNone, def.PanelScope)
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

func TestBrowserDefinitionsFillsMenuKeyLabels(t *testing.T) {
	km, err := keymap.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	defs := BrowserDefinitions(km)

	assertLabels := func(t *testing.T, def *Definition, want map[string]string) {
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

	var left, file, cmd *Definition
	for i := range defs {
		switch defs[i].ID {
		case TopPanelLeft:
			left = &defs[i]
		case TopFile:
			file = &defs[i]
		case TopCommand:
			cmd = &defs[i]
		}
	}
	if left == nil || file == nil || cmd == nil {
		t.Fatalf("missing menu: left=%v file=%v cmd=%v", left != nil, file != nil, cmd != nil)
	}

	assertLabels(t, left, map[string]string{
		"Quick view":    "S-F3",
		"Sort...":       "C-s",
		"Toggle hidden": "M-.",
		"Refresh":       "M-C-r",
		"Disk usage":    "C-d",
		"History...":    "M-h",
	})
	assertLabels(t, file, map[string]string{
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
	assertLabels(t, cmd, map[string]string{
		"Commands":     "C-k",
		"Messages":     "C-M-l",
		"Jobs":         "C-j",
		"Bookmarks":    "C-g",
		"Add bookmark": "M-m",
		"Refresh":      "M-C-r",
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
		"Cancel job":         "C-c",
		"Pause queued job":   "C-p",
		"Resume paused job":  "C-r",
		"Move up in queue":   "C-up",
		"Move down in queue": "C-down",
		"Clear finished":     "F8",
		"Back to file view":  "",
	}
	for _, item := range defs[0].Items {
		w := want[item.Label]
		if item.KeyLabel != w {
			t.Fatalf("%q KeyLabel = %q, want %q", item.Label, item.KeyLabel, w)
		}
	}
	if got := defs[1].Items[0].KeyLabel; got != "F10" {
		t.Fatalf("jobs File Exit KeyLabel = %q, want F10", got)
	}
}

func TestLeftAndRightPanelMenusShareItemActions(t *testing.T) {
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
	options := Definitions()[3]
	if options.ID != TopOptions {
		t.Fatalf("index 3 ID = %q, want TopOptions", options.ID)
	}
	if options.Label != "Options" {
		t.Fatalf("index 3 label = %q, want Options", options.Label)
	}
	items := options.Items
	if len(items) != 2 {
		t.Fatalf("options items len = %d, want only configuration and theme entries", len(items))
	}
	if items[0].Action != keymap.ActionUIOpenConfig || items[1].Action != keymap.ActionUIOpenTheme {
		t.Fatalf("unexpected Options actions: %+v / %+v", items[0].Action, items[1].Action)
	}
}

func TestFunctionKeysFilePreviewViewExcludesMenuF9(t *testing.T) {
	t.Parallel()
	keys := FunctionKeysFilePreviewView()
	for _, fk := range keys {
		if fk.Key == tcell.KeyF9 {
			t.Fatalf("fullscreen file preview footer must not advertise F9 Menu, got %+v", keys)
		}
	}
	if len(keys) != 2 {
		t.Fatalf("FunctionKeysFilePreviewView len = %d, want Esc + F10 only", len(keys))
	}
}
