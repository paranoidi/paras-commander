package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/themes"
)

func TestDefaultMatchesEmbeddedTheme(t *testing.T) {
	data, err := themes.Files.ReadFile("default.toml")
	if err != nil {
		t.Fatalf("ReadFile(default.toml): %v", err)
	}
	embedded, err := parse(data)
	if err != nil {
		t.Fatalf("parse(embedded): %v", err)
	}
	got := Default()

	if got.Name != embedded.Name {
		t.Fatalf("Name = %q, want %q", got.Name, embedded.Name)
	}

	assertSymbolRuneEqual(t, "SymbolFilelistSelectionSubtree", got.SymbolFilelistSelectionSubtree(), embedded.SymbolFilelistSelectionSubtree())
	assertSymbolRuneEqual(t, "SymbolFilelistNew", got.SymbolFilelistNew(), embedded.SymbolFilelistNew())
	assertSymbolRuneEqual(t, "SymbolFilelistNoPermission", got.SymbolFilelistNoPermission(), embedded.SymbolFilelistNoPermission())
	assertSymbolStrEqual(t, "FolderIconDefault", got.FolderIconGlyph(FolderIconDefault), embedded.FolderIconGlyph(FolderIconDefault))
	assertSymbolStrEqual(t, "FolderIconOpen", got.FolderIconGlyph(FolderIconOpen), embedded.FolderIconGlyph(FolderIconOpen))
	assertSymbolStrEqual(t, "FolderIconScanning", got.FolderIconGlyph(FolderIconScanning), embedded.FolderIconGlyph(FolderIconScanning))
	assertSymbolStrEqual(t, "FolderIconMount", got.FolderIconGlyph(FolderIconMount), embedded.FolderIconGlyph(FolderIconMount))
	assertSymbolStrEqual(t, "FolderIconExcluded", got.FolderIconGlyph(FolderIconExcluded), embedded.FolderIconGlyph(FolderIconExcluded))
	assertSymbolRuneEqual(t, "SymbolScrollbarThumb", got.SymbolScrollbarThumb(), embedded.SymbolScrollbarThumb())

	for _, status := range []string{
		"scanning", "queued", "running", "paused", "canceled", "failed", "decision", "completed",
	} {
		label := "SymbolJobsList(" + status + ")"
		assertSymbolStrEqual(t, label, got.SymbolJobsList(status), embedded.SymbolJobsList(status))
	}
	assertSymbolStrEqual(t, "SymbolWorking", got.SymbolWorking(), embedded.SymbolWorking())

	assertStyleEqual(t, "PanelRowMarkNew", got.PanelRowMarkNew, embedded.PanelRowMarkNew)
	assertStyleEqual(t, "PanelRowMarkNewPrevious", got.PanelRowMarkNewPrevious, embedded.PanelRowMarkNewPrevious)
	assertStyleEqual(t, "PanelIconFolderOpen", got.PanelIconFolderOpen, embedded.PanelIconFolderOpen)
	assertStyleEqual(t, "PanelRowMarkSelectionSubtree", got.PanelRowMarkSelectionSubtree, embedded.PanelRowMarkSelectionSubtree)
	assertStyleEqual(t, "PanelRowMarkNoPermission", got.PanelRowMarkNoPermission, embedded.PanelRowMarkNoPermission)
	assertStyleEqual(t, "PanelIconFolderMount", got.PanelIconFolderMount, embedded.PanelIconFolderMount)
	assertStyleEqual(t, "PanelRowSelected", got.PanelRowSelected, embedded.PanelRowSelected)
}

func assertSymbolRuneEqual(t *testing.T, label string, got, want rune) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", label, string(got), string(want))
	}
}

func assertSymbolStrEqual(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", label, got, want)
	}
}

func assertStyleEqual(t *testing.T, label string, got, want tcell.Style) {
	t.Helper()
	gotFG, gotBG, gotAttrs := got.Decompose()
	wantFG, wantBG, wantAttrs := want.Decompose()
	if gotFG != wantFG || gotBG != wantBG || gotAttrs != wantAttrs {
		t.Fatalf("%s = fg %v bg %v attrs %v, want fg %v bg %v attrs %v",
			label, gotFG, gotBG, gotAttrs, wantFG, wantBG, wantAttrs)
	}
}

func TestParsePaletteANSIIndex(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar.active": `{ fg = "marker", bg = "black" }`,
	})
	dataStr := string(data)
	dataStr = strings.Replace(dataStr, "yellow = \"#ffff00\"\n", "yellow = \"#ffff00\"\nmarker = 88\n", 1)

	styles, err := parse([]byte(dataStr))
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	fg, _, _ := styles.MenuBarActive.Decompose()
	if fg != tcell.PaletteColor(88) {
		t.Fatalf("palette foreground = %v, want ANSI index 88", fg)
	}
}

func TestParseResolvesPaletteHexAndAttributes(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar.active": `{ fg = "white", bg = "#0a0b0c", underline = true, reverse = true }`,
	})

	styles, err := parse(data)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	foreground, background, attrs := styles.MenuBarActive.Decompose()
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
		"menu.bar.active": `{ fg = "white", bg = "black", icon = "yellow" }`,
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
	if err == nil || !strings.Contains(err.Error(), `field "bg" is not allowed (background is merged at render time)`) {
		t.Fatalf("parse() error = %v, want reject bg on dialog.option", err)
	}
}

func TestParseRejectsBGOnDialogProgressLabel(t *testing.T) {
	data := testTheme(t, "badprogresslabelbg", nil, map[string]string{
		"dialog.progress.label.on_track": `{ fg = "white", bg = "black", bold = true }`,
	})
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `field "bg" is not allowed (background is merged at render time)`) {
		t.Fatalf("parse() error = %v, want reject bg on dialog.progress.label.on_track", err)
	}
}

func TestDialogProgressLabelOnBarUsesTrackBackground(t *testing.T) {
	data := testTheme(t, "dedupprogress", nil, map[string]string{
		"dialog.progress.track":          `{ fg = "black", bg = "yellow" }`,
		"dialog.progress.fill":           `{ fg = "black", bg = "white" }`,
		"dialog.progress.label.on_track": `{ fg = "white", bold = true }`,
		"dialog.progress.label.on_fill":  `{ fg = "black", bold = true }`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, wantTrackBG, _ := th.DialogProgressTrack.Decompose()
	_, wantFillBG, _ := th.DialogProgressFill.Decompose()

	trackLabel := th.DialogProgressLabelOnBar(false)
	_, trackBG, _ := trackLabel.Decompose()
	if trackBG != wantTrackBG {
		t.Fatalf("DialogProgressLabelOnBar(track) bg = %v, want track bg %v", trackBG, wantTrackBG)
	}

	fillLabel := th.DialogProgressLabelOnBar(true)
	_, fillBG, _ := fillLabel.Decompose()
	if fillBG != wantFillBG {
		t.Fatalf("DialogProgressLabelOnBar(fill) bg = %v, want fill bg %v", fillBG, wantFillBG)
	}
}

func TestParseRejectsBGOnDialogStatusSelectionSize(t *testing.T) {
	data := testTheme(t, "badstatusbg", nil, map[string]string{
		"dialog.status.selection_size": `{ fg = "yellow", bg = "bright_black" }`,
	})
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `field "bg" is not allowed (background is merged at render time)`) {
		t.Fatalf("parse() error = %v, want reject bg on dialog.status.selection_size", err)
	}
}

func TestDialogStatusSelectionSizeStyleUsesSurfaceBackground(t *testing.T) {
	data := testTheme(t, "findselectionsize", nil, map[string]string{
		"dialog.surface":               `{ fg = "white", bg = "yellow" }`,
		"dialog.status.selection_size": `{ fg = "yellow", bold = true }`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, surfaceBG, _ := th.DialogSurface.Decompose()
	got := th.DialogStatusSelectionSizeStyle()
	fg, bg, attrs := got.Decompose()
	if bg != surfaceBG {
		t.Fatalf("DialogStatusSelectionSizeStyle bg = %v, want surface bg %v", bg, surfaceBG)
	}
	wantFG, _, _ := th.DialogStatusSelectionSize.Decompose()
	if fg != wantFG {
		t.Fatalf("DialogStatusSelectionSizeStyle fg = %v, want %v", fg, wantFG)
	}
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("expected bold from dialog.status.selection_size")
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

func TestParseQuickActionBorderDefaultsToRounded(t *testing.T) {
	data := testTheme(t, "qaborderdefault", nil, nil)
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if th.DialogQuickActionBorderStyle != QuickActionBorderRounded {
		t.Fatalf("DialogQuickActionBorderStyle = %q, want %q", th.DialogQuickActionBorderStyle, QuickActionBorderRounded)
	}
	if th.QuickActionBorderGlyphs() != (primitive.RoundedBorder) {
		t.Fatalf("QuickActionBorderGlyphs() = %v, want RoundedBorder", th.QuickActionBorderGlyphs())
	}
}

func TestParseQuickActionBorderSharp(t *testing.T) {
	data := testTheme(t, "qabordersharp", nil, map[string]string{
		"dialog.quickaction.border": `"sharp"`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if th.DialogQuickActionBorderStyle != QuickActionBorderSharp {
		t.Fatalf("DialogQuickActionBorderStyle = %q, want %q", th.DialogQuickActionBorderStyle, QuickActionBorderSharp)
	}
	if th.QuickActionBorderGlyphs() != (primitive.SharpBorder) {
		t.Fatalf("QuickActionBorderGlyphs() = %v, want SharpBorder", th.QuickActionBorderGlyphs())
	}
}

func TestParseQuickActionBorderRejectsInvalidValue(t *testing.T) {
	data := testTheme(t, "qaborderbad", nil, map[string]string{
		"dialog.quickaction.border": `"curvy"`,
	})
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `dialog.quickaction.border must be`) {
		t.Fatalf("parse() error = %v, want invalid border style error", err)
	}
}

func TestParsePaletteDefaultTerminalColor(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar.active": `{ fg = "termfg", bg = "termbg" }`,
	})
	dataStr := strings.Replace(string(data), "yellow = \"#ffff00\"\n", "yellow = \"#ffff00\"\ntermfg = \"default\"\ntermbg = \"default\"\n", 1)

	styles, err := parse([]byte(dataStr))
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	fg, bg, _ := styles.MenuBarActive.Decompose()
	if fg != tcell.ColorDefault || bg != tcell.ColorDefault {
		t.Fatalf("menu fg=%v bg=%v, want ColorDefault for both", fg, bg)
	}
}

func TestParseResolvesDefaultColorKeyword(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar.active": `{ fg = "default", bg = "default" }`,
	})

	styles, err := parse(data)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	foreground, background, _ := styles.MenuBarActive.Decompose()
	if foreground != tcell.ColorDefault {
		t.Fatalf("foreground = %v, want ColorDefault", foreground)
	}
	if background != tcell.ColorDefault {
		t.Fatalf("background = %v, want ColorDefault", background)
	}
}

func TestParseResolvesPartialDefaultColorKeyword(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar.active": `{ fg = "default", bg = "black" }`,
	})

	styles, err := parse(data)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	foreground, background, _ := styles.MenuBarActive.Decompose()
	if foreground != tcell.ColorDefault {
		t.Fatalf("foreground = %v, want ColorDefault", foreground)
	}
	if background != tcell.NewRGBColor(4, 5, 6) {
		t.Fatalf("background = %v, want palette black #040506", background)
	}
}

func TestParseRejectsInvalidColor(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar.active": `{ fg = "missing", bg = "black" }`,
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
		"menu.bar.active": `{ fg = "white", bg = "#111111" }`,
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
			_, bg, _ = ch.Theme.MenuBarActive.Decompose()
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
		"menu.bar.active": `{ fg = "white", bg = "#112233" }`,
	})
	if err := os.WriteFile(filepath.Join(dir, "custom.toml"), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := Resolve("default", dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	_, bg, _ := got.MenuBarActive.Decompose()
	if want := tcell.NewRGBColor(0x11, 0x22, 0x33); bg != want {
		t.Fatalf("disk theme bg = %v, want %v (#112233 from user themes dir)", bg, want)
	}
}

func TestResolveSkipsBrokenSiblingTomlFiles(t *testing.T) {
	dir := t.TempDir()
	good := testTheme(t, "default", nil, map[string]string{
		"menu.bar.active": `{ fg = "white", bg = "#111111" }`,
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
	_, bg, _ := got.MenuBarActive.Decompose()
	if want := tcell.NewRGBColor(0x11, 0x11, 0x11); bg != want {
		t.Fatalf("bg = %v, want disk override %v", bg, want)
	}
}

func testTheme(t *testing.T, name string, skip map[string]bool, overrides map[string]string) []byte {
	return TestThemeBytesNamed(t, name, skip, overrides)
}
