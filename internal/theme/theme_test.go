package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestDefaultFilelistSymbols(t *testing.T) {
	th := Default()
	if g := th.SymbolFilelistSelectionSubtree(); g != '\u25cb' {
		t.Fatalf("SymbolFilelistSelectionSubtree = %q, want ○", string(g))
	}
	if g := th.SymbolFilelistNew(); g != '\uea7f' {
		t.Fatalf("SymbolFilelistNew = %U, want U+EA7F", g)
	}
	if g := th.SymbolFoldersFolder(); g != "\U000F024B" {
		t.Fatalf("SymbolFoldersFolder = %q, want U+F024B", g)
	}
	if g := th.SymbolFoldersOpen(); g != "\U000F0770" {
		t.Fatalf("SymbolFoldersOpen = %q, want U+F0770", g)
	}
	if g := th.SymbolFoldersScanning(); g != "\U000F0D0B" {
		t.Fatalf("SymbolFoldersScanning = %q, want U+F0D0B", g)
	}
	if g := th.SymbolFoldersMount(); g != "\U000F0256" {
		t.Fatalf("SymbolFoldersMount = %q, want U+F0256", g)
	}
	if g := th.SymbolScrollbarThumb(); g != '\u2503' {
		t.Fatalf("SymbolScrollbarThumb = %q, want ┃", string(g))
	}
	newFG, _, _ := th.PanelRowIndicatorNew.Decompose()
	if newFG != tcell.PaletteColor(10) {
		t.Fatalf("panel.row.indicator.new fg = %v, want bright_green (index 10)", newFG)
	}
	prevFG, _, _ := th.PanelRowIndicatorNewPrevious.Decompose()
	if prevFG != tcell.PaletteColor(11) {
		t.Fatalf("panel.row.indicator.new.previous fg = %v, want bright_yellow (index 11)", prevFG)
	}
	openFG, _, _ := th.PanelRowFolderOpen.Decompose()
	if openFG != tcell.PaletteColor(14) {
		t.Fatalf("panel.row.folder.open fg = %v, want bright_cyan (index 14)", openFG)
	}
	subFG, _, _ := th.PanelRowIndicatorSelectionSubtree.Decompose()
	if subFG != tcell.PaletteColor(3) {
		t.Fatalf("panel.row.indicator.selection_subtree fg = %v, want yellow (index 3)", subFG)
	}
	mountFG, _, _ := th.PanelRowFolderMount.Decompose()
	if mountFG != tcell.PaletteColor(12) {
		t.Fatalf("panel.row.folder.mount fg = %v, want bright_blue (index 12)", mountFG)
	}
}

func TestDefaultLoadsEmbeddedTheme(t *testing.T) {
	styles := Default()
	if styles.Name != "default" {
		t.Fatalf("Default().Name = %q, want default", styles.Name)
	}

	foreground, background, attrs := styles.PanelRowSelected.Decompose()
	if foreground != tcell.PaletteColor(3) {
		t.Fatalf("selected foreground = %v, want ANSI yellow (index 3)", foreground)
	}
	if background != tcell.ColorDefault {
		t.Fatalf("selected background = %v, want terminal default background (theme uses default)", background)
	}
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("selected attrs do not include bold")
	}
}

func TestParsePaletteANSIIndex(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar": `{ fg = "marker", bg = "black" }`,
	})
	dataStr := string(data)
	dataStr = strings.Replace(dataStr, "yellow = \"#ffff00\"\n", "yellow = \"#ffff00\"\nmarker = 88\n", 1)

	styles, err := parse([]byte(dataStr))
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	fg, _, _ := styles.MenuBar.Decompose()
	if fg != tcell.PaletteColor(88) {
		t.Fatalf("palette foreground = %v, want ANSI index 88", fg)
	}
}

func TestParseResolvesPaletteHexAndAttributes(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar": `{ fg = "white", bg = "#0a0b0c", underline = true, reverse = true }`,
	})

	styles, err := parse(data)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	foreground, background, attrs := styles.MenuBar.Decompose()
	if foreground != tcell.NewRGBColor(1, 2, 3) {
		t.Fatalf("foreground = %v, want palette white", foreground)
	}
	if background != tcell.NewRGBColor(10, 11, 12) {
		t.Fatalf("background = %v, want direct hex color", background)
	}
	if attrs&tcell.AttrUnderline == 0 {
		t.Fatal("attrs do not include underline")
	}
	if attrs&tcell.AttrReverse == 0 {
		t.Fatal("attrs do not include reverse")
	}
}

func TestParseRejectsMissingRequiredStyle(t *testing.T) {
	data := testTheme(t, "custom", map[string]bool{"footer.label": true}, nil)

	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `missing required style "footer.label"`) {
		t.Fatalf("parse() error = %v, want missing footer.label", err)
	}
}

func TestParseRejectsMissingFooterLabelShift(t *testing.T) {
	data := testTheme(t, "custom", map[string]bool{"footer.label.shift": true}, nil)

	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `missing required style "footer.label.shift"`) {
		t.Fatalf("parse() error = %v, want missing footer.label.shift", err)
	}
}

func TestParseAllowsDialogTitleBoldOnly(t *testing.T) {
	data := testTheme(t, "boldtitle", nil, map[string]string{
		"dialog.title": `{ bold = true }`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, _, attrs := th.DialogTitle.Decompose()
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("dialog.title: want bold from minimal style entry")
	}
}

func TestParseLoadsPanelCursorIconFG(t *testing.T) {
	for _, key := range []string{
		"panel.active.row.cursor",
		"panel.active.row.cursor.selected",
		"panel.inactive.row.cursor",
		"panel.inactive.row.cursor.selected",
		"panel.carousel.inactive.row.cursor",
		"panel.carousel.inactive.row.cursor.selected",
		"panel.blocked.row.cursor",
		"panel.blocked.row.cursor.selected",
	} {
		o := map[string]string{}
		for _, k := range requiredStyleKeys {
			if k == key {
				o[k] = `{ fg = "white", bg = "black", icon = "yellow" }`
			} else if _, isOption := dialogSurfaceForegroundStyleKeys[k]; isOption {
				o[k] = `{ fg = "white" }`
			} else {
				o[k] = `{ fg = "white", bg = "black" }`
			}
		}
		data := testTheme(t, "iconrow", nil, o)
		th, err := parse(data)
		if err != nil {
			t.Fatalf("parse for %s: %v", key, err)
		}
		got, ok := th.PanelFileIconFG[key]
		if !ok || got != tcell.NewRGBColor(255, 255, 0) {
			t.Fatalf("%s: PanelFileIconFG = %v ok=%v want yellow", key, got, ok)
		}
	}
}

func TestParseRejectsIconOnNonCursorStyle(t *testing.T) {
	data := testTheme(t, "badicon", nil, map[string]string{
		"menu.bar": `{ fg = "white", bg = "black", icon = "yellow" }`,
	})
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `field "icon" is only allowed on panel cursor row styles`) {
		t.Fatalf("parse() error = %v, want reject icon on menu", err)
	}
}

func TestParseRejectsBGOnDialogOption(t *testing.T) {
	data := testTheme(t, "badoptionbg", nil, map[string]string{
		"dialog.option.active": `{ fg = "yellow", bg = "bright_black", bold = true }`,
	})
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `field "bg" is not allowed (background comes from dialog.surface)`) {
		t.Fatalf("parse() error = %v, want reject bg on dialog.option", err)
	}
}

func TestParseRejectsBGOnDialogIndicatorSelectionSize(t *testing.T) {
	data := testTheme(t, "badindicatorbg", nil, map[string]string{
		"dialog.indicator.selection_size": `{ fg = "yellow", bg = "bright_black" }`,
	})
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `field "bg" is not allowed (background comes from dialog.surface)`) {
		t.Fatalf("parse() error = %v, want reject bg on dialog.indicator.selection_size", err)
	}
}

func TestDialogIndicatorSelectionSizeStyleUsesSurfaceBackground(t *testing.T) {
	data := testTheme(t, "findselectionsize", nil, map[string]string{
		"dialog.surface":                  `{ fg = "white", bg = "yellow" }`,
		"dialog.indicator.selection_size": `{ fg = "yellow", bold = true }`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, surfaceBG, _ := th.DialogSurface.Decompose()
	got := th.DialogIndicatorSelectionSizeStyle()
	fg, bg, attrs := got.Decompose()
	if bg != surfaceBG {
		t.Fatalf("DialogIndicatorSelectionSizeStyle bg = %v, want surface bg %v", bg, surfaceBG)
	}
	wantFG, _, _ := th.DialogIndicatorSelectionSize.Decompose()
	if fg != wantFG {
		t.Fatalf("DialogIndicatorSelectionSizeStyle fg = %v, want %v", fg, wantFG)
	}
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("expected bold from dialog.indicator.selection_size")
	}
}

func TestDialogOptionRowStyleUsesSurfaceBackground(t *testing.T) {
	data := testTheme(t, "optionrow", nil, map[string]string{
		"dialog.surface":       `{ fg = "white", bg = "yellow" }`,
		"dialog.option.active": `{ fg = "yellow", bold = true }`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, surfaceBG, _ := th.DialogSurface.Decompose()
	got := th.DialogOptionRowStyle(true, false)
	fg, bg, attrs := got.Decompose()
	if bg != surfaceBG {
		t.Fatalf("DialogOptionRowStyle bg = %v, want surface bg %v", bg, surfaceBG)
	}
	wantFG, _, _ := th.DialogOptionActive.Decompose()
	if fg != wantFG {
		t.Fatalf("DialogOptionRowStyle fg = %v, want %v from option.active", fg, wantFG)
	}
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("DialogOptionRowStyle: want bold from option.active")
	}
}

func TestDialogOptionRowStyleActiveSelected(t *testing.T) {
	data := testTheme(t, "optionrowactivesel", nil, map[string]string{
		"dialog.surface":                `{ fg = "white", bg = "yellow" }`,
		"dialog.option.active.selected": `{ fg = "yellow", bold = true }`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, surfaceBG, _ := th.DialogSurface.Decompose()
	got := th.DialogOptionRowStyle(true, true)
	fg, bg, attrs := got.Decompose()
	if bg != surfaceBG {
		t.Fatalf("DialogOptionRowStyle bg = %v, want surface bg %v", bg, surfaceBG)
	}
	wantFG, _, _ := th.DialogOptionActiveSelected.Decompose()
	if fg != wantFG {
		t.Fatalf("DialogOptionRowStyle fg = %v, want %v from option.active.selected", fg, wantFG)
	}
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("DialogOptionRowStyle: want bold from option.active.selected")
	}
}

func TestParseRejectsUnknownStyle(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"fuzzy.unknown.token": `{ fg = "white", bg = "black" }`,
	})

	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `unknown style "fuzzy.unknown.token"`) {
		t.Fatalf("parse() error = %v, want unknown style", err)
	}
}

func TestParseRejectsStylesSection(t *testing.T) {
	data := []byte(`name = "legacy"

[palette]
black = "#040506"

[styles]
menu.bar = { fg = "white", bg = "black" }
`)
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), "[styles] is not supported") {
		t.Fatalf("parse() error = %v, want reject [styles]", err)
	}
}

func TestParseDialogSectionFlatKeys(t *testing.T) {
	data := testTheme(t, "dialogflat", nil, map[string]string{
		"dialog.input.active": `{ fg = "white", bg = "default", bold = true }`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fg, _, attrs := th.DialogInputActive.Decompose()
	if fg != tcell.NewRGBColor(0x01, 0x02, 0x03) {
		t.Fatalf("DialogInputActive fg = %v, want palette white", fg)
	}
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("DialogInputActive: want bold")
	}
}

func TestParsePaletteDefaultTerminalColor(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar": `{ fg = "termfg", bg = "termbg" }`,
	})
	dataStr := strings.Replace(string(data), "yellow = \"#ffff00\"\n", "yellow = \"#ffff00\"\ntermfg = \"default\"\ntermbg = \"default\"\n", 1)

	styles, err := parse([]byte(dataStr))
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	fg, bg, _ := styles.MenuBar.Decompose()
	if fg != tcell.ColorDefault || bg != tcell.ColorDefault {
		t.Fatalf("menu fg=%v bg=%v, want ColorDefault for both", fg, bg)
	}
}

func TestParseResolvesDefaultColorKeyword(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar": `{ fg = "default", bg = "default" }`,
	})

	styles, err := parse(data)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	foreground, background, _ := styles.MenuBar.Decompose()
	if foreground != tcell.ColorDefault {
		t.Fatalf("foreground = %v, want ColorDefault", foreground)
	}
	if background != tcell.ColorDefault {
		t.Fatalf("background = %v, want ColorDefault", background)
	}
}

func TestParseResolvesPartialDefaultColorKeyword(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar": `{ fg = "default", bg = "black" }`,
	})

	styles, err := parse(data)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	foreground, background, _ := styles.MenuBar.Decompose()
	if foreground != tcell.ColorDefault {
		t.Fatalf("foreground = %v, want ColorDefault", foreground)
	}
	if background != tcell.NewRGBColor(4, 5, 6) {
		t.Fatalf("background = %v, want palette black #040506", background)
	}
}

func TestParseRejectsInvalidColor(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar": `{ fg = "missing", bg = "black" }`,
	})

	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `unknown color "missing"`) {
		t.Fatalf("parse() error = %v, want unknown color", err)
	}
}

func TestThemeChoicesAppendsDiskOnlyThemes(t *testing.T) {
	dir := t.TempDir()
	custom := testTheme(t, "acustom", nil, nil)
	if err := os.WriteFile(filepath.Join(dir, "acustom.toml"), custom, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	choices, err := ThemeChoices(dir)
	if err != nil {
		t.Fatalf("ThemeChoices: %v", err)
	}
	if len(choices) != len(builtInThemeOrder)+1 {
		t.Fatalf("len = %d, want %d", len(choices), len(builtInThemeOrder)+1)
	}
	last := choices[len(choices)-1]
	if last.Name != "acustom" {
		t.Fatalf("last theme name = %q, want acustom", last.Name)
	}
}

func TestThemeChoicesDiskOverridesBuiltInName(t *testing.T) {
	dir := t.TempDir()
	data := testTheme(t, "default", nil, map[string]string{
		"menu.bar": `{ fg = "white", bg = "#111111" }`,
	})
	if err := os.WriteFile(filepath.Join(dir, "local-default.toml"), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	choices, err := ThemeChoices(dir)
	if err != nil {
		t.Fatalf("ThemeChoices: %v", err)
	}
	var seen int
	var bg tcell.Color
	for _, ch := range choices {
		if ch.Name == "default" {
			seen++
			_, bg, _ = ch.Theme.MenuBar.Decompose()
		}
	}
	if seen != 1 {
		t.Fatalf("default entries = %d, want 1", seen)
	}
	if bg != tcell.NewRGBColor(17, 17, 17) {
		t.Fatalf("overridden default bg = %v, want #111111", bg)
	}
}

func TestLoadDirRejectsDuplicateThemeNames(t *testing.T) {
	dir := t.TempDir()
	data := testTheme(t, "duplicate", nil, nil)
	if err := os.WriteFile(filepath.Join(dir, "one.toml"), data, 0o644); err != nil {
		t.Fatalf("WriteFile(one.toml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "two.toml"), data, 0o644); err != nil {
		t.Fatalf("WriteFile(two.toml) error = %v", err)
	}

	_, err := LoadDir(dir)
	if err == nil || !strings.Contains(err.Error(), `duplicate theme name "duplicate"`) {
		t.Fatalf("LoadDir() error = %v, want duplicate theme name", err)
	}
}

func TestResolveFallsBackToDefaultForUnavailableTheme(t *testing.T) {
	styles, err := Resolve("missing", "")
	if err == nil {
		t.Fatal("Resolve() error = nil, want unavailable theme error")
	}
	if styles.Name != "default" {
		t.Fatalf("Resolve() fallback name = %q, want default", styles.Name)
	}
}

func TestResolvePrefersDiskThemeOverBuiltInWithSameName(t *testing.T) {
	dir := t.TempDir()
	data := testTheme(t, "default", nil, map[string]string{
		"menu.bar": `{ fg = "white", bg = "#112233" }`,
	})
	if err := os.WriteFile(filepath.Join(dir, "custom.toml"), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := Resolve("default", dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	_, bg, _ := got.MenuBar.Decompose()
	if want := tcell.NewRGBColor(0x11, 0x22, 0x33); bg != want {
		t.Fatalf("disk theme bg = %v, want %v (#112233 from user themes dir)", bg, want)
	}
}

func TestResolveSkipsBrokenSiblingTomlFiles(t *testing.T) {
	dir := t.TempDir()
	good := testTheme(t, "default", nil, map[string]string{
		"menu.bar": `{ fg = "white", bg = "#111111" }`,
	})
	if err := os.WriteFile(filepath.Join(dir, "good.toml"), good, 0o644); err != nil {
		t.Fatalf("WriteFile good.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.toml"), []byte("[[[\nnot valid toml\n"), 0o644); err != nil {
		t.Fatalf("WriteFile broken.toml: %v", err)
	}
	got, err := Resolve("default", dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	_, bg, _ := got.MenuBar.Decompose()
	if want := tcell.NewRGBColor(0x11, 0x11, 0x11); bg != want {
		t.Fatalf("bg = %v, want disk override %v", bg, want)
	}
}

func testTheme(t *testing.T, name string, skip map[string]bool, overrides map[string]string) []byte {
	return TestThemeBytesNamed(t, name, skip, overrides)
}
