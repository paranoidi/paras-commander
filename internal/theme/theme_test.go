package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/gdamore/tcell/v2"
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

	assertIconRuneEqual(t, "IconFilelistSelectionSubtree", got.IconFilelistSelectionSubtree(), embedded.IconFilelistSelectionSubtree())
	assertIconRuneEqual(t, "IconFilelistNew", got.IconFilelistNew(), embedded.IconFilelistNew())
	assertIconRuneEqual(t, "IconFilelistNoPermission", got.IconFilelistNoPermission(), embedded.IconFilelistNoPermission())
	assertIconStrEqual(t, "FolderIconDefault", got.FolderIcon(FolderIconDefault), embedded.FolderIcon(FolderIconDefault))
	assertIconStrEqual(t, "FolderIconOpen", got.FolderIcon(FolderIconOpen), embedded.FolderIcon(FolderIconOpen))
	assertIconStrEqual(t, "FolderIconScanning", got.FolderIcon(FolderIconScanning), embedded.FolderIcon(FolderIconScanning))
	assertIconStrEqual(t, "FolderIconMount", got.FolderIcon(FolderIconMount), embedded.FolderIcon(FolderIconMount))
	assertIconStrEqual(t, "FolderIconExcluded", got.FolderIcon(FolderIconExcluded), embedded.FolderIcon(FolderIconExcluded))
	assertIconRuneEqual(t, "IconScrollbarThumb", got.IconScrollbarThumb(), embedded.IconScrollbarThumb())

	for _, status := range []string{
		"scanning", "queued", "running", "paused", "canceled", "failed", "decision", "completed",
	} {
		label := "IconJobsList(" + status + ")"
		assertIconStrEqual(t, label, got.IconJobsList(status), embedded.IconJobsList(status))
	}
	assertIconStrEqual(t, "IconWorking", got.IconWorking(), embedded.IconWorking())
	assertIconStrEqual(t, "IconSearchIcon", got.IconSearchIcon(), embedded.IconSearchIcon())

	assertStyleEqual(t, "PanelRowMarkNew", got.PanelRowMarkNew, embedded.PanelRowMarkNew)
	assertStyleEqual(t, "PanelRowMarkNewPrevious", got.PanelRowMarkNewPrevious, embedded.PanelRowMarkNewPrevious)
	assertStyleEqual(t, "PanelIconFolderOpen", got.PanelIconFolderOpen, embedded.PanelIconFolderOpen)
	assertStyleEqual(t, "PanelRowMarkSelectionSubtree", got.PanelRowMarkSelectionSubtree, embedded.PanelRowMarkSelectionSubtree)
	assertStyleEqual(t, "PanelRowMarkNoPermission", got.PanelRowMarkNoPermission, embedded.PanelRowMarkNoPermission)
	assertStyleEqual(t, "PanelIconFolderMount", got.PanelIconFolderMount, embedded.PanelIconFolderMount)
	assertStyleEqual(t, "PanelRowSelected", got.PanelRowSelected, embedded.PanelRowSelected)
	assertStyleEqual(t, "DialogSearchIcon", got.DialogSearchIcon, embedded.DialogSearchIcon)
}

func assertIconRuneEqual(t *testing.T, label string, got, want rune) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", label, string(got), string(want))
	}
}

func assertIconStrEqual(t *testing.T, label, got, want string) {
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
		"palette.red":     "88",
		"roles.accent":    `"red"`,
		"menu.bar.active": `{ fg = "accent", bg = "bg" }`,
	})

	styles, err := parse(data)
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
		"roles.surface":   `"#0a0b0c"`,
		"menu.bar.active": `{ fg = "fg", bg = "surface", underline = true, reverse = true }`,
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
		t.Fatalf("background = %v, want hex role value", background)
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
				o[k] = `{ fg = "fg", bg = "bg", icon = "warning" }`
			} else if _, isOption := dialogSurfaceForegroundStyleKeys[k]; isOption {
				o[k] = `{ fg = "fg" }`
			} else {
				o[k] = `{ fg = "fg", bg = "bg" }`
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
		"menu.bar.active": `{ fg = "fg", bg = "bg", icon = "warning" }`,
	})
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `field "icon" is only allowed on panel cursor row and disk-usage bar styles`) {
		t.Fatalf("parse() error = %v, want reject icon on menu", err)
	}
}

func TestDiskUsageBarIconKey(t *testing.T) {
	cases := []struct {
		active, cursor, selected bool
		key                      string
	}{
		{false, false, false, "panel.usage.normal"},
		{true, false, false, "panel.usage.normal"},
		{true, false, true, "panel.usage.selected"},
		{true, true, false, "panel.active.usage.cursor"},
		{true, true, true, "panel.active.usage.cursor.selected"},
		{false, true, false, "panel.inactive.usage.cursor"},
		{false, true, true, "panel.inactive.usage.cursor.selected"},
	}
	for _, tc := range cases {
		o := map[string]string{}
		for _, k := range requiredStyleKeys {
			if _, isOption := dialogSurfaceForegroundStyleKeys[k]; isOption {
				o[k] = `{ fg = "fg" }`
			} else {
				o[k] = `{ fg = "fg", bg = "bg" }`
			}
		}
		plain, err := parse(testTheme(t, "baricon", nil, o))
		if err != nil {
			t.Fatalf("parse without icon: %v", err)
		}
		if got := plain.DiskUsageBarIconKey(tc.active, tc.cursor, tc.selected); got != "" {
			t.Fatalf("%s without icon: key = %q, want empty", tc.key, got)
		}
		o[tc.key] = `{ fg = "fg", bg = "bg", icon = "warning" }`
		th, err := parse(testTheme(t, "baricon", nil, o))
		if err != nil {
			t.Fatalf("parse for %s: %v", tc.key, err)
		}
		if got := th.DiskUsageBarIconKey(tc.active, tc.cursor, tc.selected); got != tc.key {
			t.Fatalf("key = %q, want %q", got, tc.key)
		}
		if got := th.PanelFileIconFG[tc.key]; got != tcell.NewRGBColor(255, 255, 0) {
			t.Fatalf("%s: PanelFileIconFG = %v, want yellow", tc.key, got)
		}
	}
}

func TestDefaultThemeBarIconOnlyOnNonCursorBar(t *testing.T) {
	th := Default()
	black := th.PanelFileIconFG["panel.active.row.cursor"]
	for _, selected := range []bool{false, true} {
		if key := th.DiskUsageBarIconKey(true, false, selected); key == "" {
			t.Errorf("selected=%v: non-cursor bar must set icon", selected)
		} else if got := th.PanelFileIconFG[key]; got != black {
			t.Errorf("%s icon = %v, want %v", key, got, black)
		}
		for _, active := range []bool{false, true} {
			if key := th.DiskUsageBarIconKey(active, true, selected); key != "" {
				t.Errorf("cursor bar active=%v selected=%v: key = %q, want none", active, selected, key)
			}
		}
	}
}

func TestParseRejectsBGOnDialogOption(t *testing.T) {
	data := testTheme(t, "badoptionbg", nil, map[string]string{
		"dialog.option.active": `{ fg = "warning", bg = "surface", bold = true }`,
	})
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `field "bg" is not allowed (background is merged at render time)`) {
		t.Fatalf("parse() error = %v, want reject bg on dialog.option", err)
	}
}

func TestParseRejectsBGOnDialogProgressLabel(t *testing.T) {
	data := testTheme(t, "badprogresslabelbg", nil, map[string]string{
		"dialog.progress.label.on_track": `{ fg = "fg", bg = "bg", bold = true }`,
	})
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `field "bg" is not allowed (background is merged at render time)`) {
		t.Fatalf("parse() error = %v, want reject bg on dialog.progress.label.on_track", err)
	}
}

func TestDialogProgressLabelOnBarUsesTrackBackground(t *testing.T) {
	data := testTheme(t, "dedupprogress", nil, map[string]string{
		"dialog.progress.track":          `{ fg = "bg", bg = "warning" }`,
		"dialog.progress.fill":           `{ fg = "bg", bg = "fg" }`,
		"dialog.progress.label.on_track": `{ fg = "fg", bold = true }`,
		"dialog.progress.label.on_fill":  `{ fg = "bg", bold = true }`,
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
		"dialog.status.selection_size": `{ fg = "warning", bg = "surface" }`,
	})
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `field "bg" is not allowed (background is merged at render time)`) {
		t.Fatalf("parse() error = %v, want reject bg on dialog.status.selection_size", err)
	}
}

func TestDialogStatusSelectionSizeStyleUsesSurfaceBackground(t *testing.T) {
	data := testTheme(t, "findselectionsize", nil, map[string]string{
		"dialog.surface":               `{ fg = "fg", bg = "warning" }`,
		"dialog.status.selection_size": `{ fg = "warning", bold = true }`,
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
		"dialog.surface":       `{ fg = "fg", bg = "warning" }`,
		"dialog.option.active": `{ fg = "warning", bold = true }`,
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
		"dialog.surface":                `{ fg = "fg", bg = "warning" }`,
		"dialog.option.active.selected": `{ fg = "warning", bold = true }`,
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
		"fuzzy.unknown.token": `{ fg = "fg", bg = "bg" }`,
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
menu.bar = { fg = "fg", bg = "bg" }
`)
	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), "[styles] is not supported") {
		t.Fatalf("parse() error = %v, want reject [styles]", err)
	}
}

func TestParseDialogSectionFlatKeys(t *testing.T) {
	data := testTheme(t, "dialogflat", nil, map[string]string{
		"dialog.input.active": `{ fg = "fg", bg = "bg", bold = true }`,
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

func TestParseLeaderMenuStyles(t *testing.T) {
	data := testTheme(t, "modemenu", nil, map[string]string{
		"leader_menu.key": `{ fg = "fg", bold = true }`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if th.LeaderMenuKey == (tcell.Style{}) {
		t.Fatal("LeaderMenuKey should be set")
	}
	if th.LeaderMenuGroup == (tcell.Style{}) {
		t.Fatal("LeaderMenuGroup should be set")
	}
	if th.IconLeaderMenuArrow() == 0 {
		t.Fatal("IconLeaderMenuArrow should have a default")
	}
}

func TestParseRoleDefaultTerminalColor(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"roles.fg":        `"default"`,
		"roles.bg":        `"default"`,
		"menu.bar.active": `{ fg = "fg", bg = "bg" }`,
	})

	styles, err := parse(data)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	fg, bg, _ := styles.MenuBarActive.Decompose()
	if fg != tcell.ColorDefault || bg != tcell.ColorDefault {
		t.Fatalf("menu fg=%v bg=%v, want ColorDefault for both", fg, bg)
	}
}

func TestParseResolvesPartialDefaultColorKeyword(t *testing.T) {
	data := testTheme(t, "custom", nil, map[string]string{
		"roles.fg":        `"default"`,
		"menu.bar.active": `{ fg = "fg", bg = "bg" }`,
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
		"menu.bar.active": `{ fg = "missing", bg = "bg" }`,
	})

	_, err := parse(data)
	if err == nil || !strings.Contains(err.Error(), `fg "missing" is not a role`) {
		t.Fatalf("parse() error = %v, want not a role", err)
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
		"roles.surface":   `"#111111"`,
		"menu.bar.active": `{ fg = "fg", bg = "surface" }`,
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
		"roles.surface":   `"#112233"`,
		"menu.bar.active": `{ fg = "fg", bg = "surface" }`,
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
		"roles.surface":   `"#111111"`,
		"menu.bar.active": `{ fg = "fg", bg = "surface" }`,
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

func TestPanelBottomIndicatorUsesDedicatedGitignoreAndDotfilesStyles(t *testing.T) {
	t.Parallel()
	status := tcell.StyleDefault.Foreground(tcell.ColorRed)
	frame := tcell.StyleDefault.Foreground(tcell.ColorBlue)
	th := Theme{
		PanelStatusGitignore:      status,
		PanelStatusDotfilesHidden: status,
		PanelActiveFrame:          frame,
		PanelInactiveFrame:        frame,
	}
	if got := th.PanelBottomIndicator(PanelBottomIndicatorKeyGitignore, true, false); got != status {
		t.Fatalf("gitignore = %v, want dedicated status %v (not frame %v)", got, status, frame)
	}
	if got := th.PanelBottomIndicator(PanelBottomIndicatorKeyDotfilesHidden, true, false); got != status {
		t.Fatalf("dotfiles_hidden = %v, want dedicated status %v (not frame %v)", got, status, frame)
	}
}

func testTheme(t *testing.T, name string, skip map[string]bool, overrides map[string]string) []byte {
	return TestThemeBytesNamed(t, name, skip, overrides)
}

func TestDialogTextSelectStyleFallsBackToSurface(t *testing.T) {
	data := testTheme(t, "textselectfallback", nil, map[string]string{
		"dialog.surface": `{ fg = "fg", bg = "warning" }`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, surfaceBG, _ := th.DialogSurface.Decompose()
	fg, bg, attrs := th.DialogTextSelectStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)).Decompose()
	if bg != surfaceBG || fg != tcell.ColorRed || attrs&tcell.AttrBold == 0 {
		t.Fatalf("fallback style fg=%v bg=%v attrs=%v", fg, bg, attrs)
	}

	data = testTheme(t, "textselectset", nil, map[string]string{
		"dialog.surface":     `{ fg = "fg", bg = "warning" }`,
		"dialog.text_select": `{ bg = "bg" }`,
	})
	th, err = parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, want, _ := th.DialogTextSelect.Decompose()
	if _, bg, _ = th.DialogTextSelectStyle(tcell.StyleDefault.Foreground(tcell.ColorRed)).Decompose(); bg != want || bg == surfaceBG {
		t.Fatalf("bg = %v, want %v", bg, want)
	}
}

func TestEmptyTextAndStyle(t *testing.T) {
	var bare Theme
	if got := bare.EmptyText(); got != "[empty]" {
		t.Fatalf("fallback EmptyText = %q", got)
	}
	if fg, _, _ := bare.PanelEmptyStyle().Decompose(); fg != tcell.PaletteColor(8) {
		t.Fatalf("fallback fg = %v", fg)
	}
	set := Theme{Icons: map[string]string{IconKeyEmpty: "nothing"}, PanelEmpty: tcell.StyleDefault.Foreground(tcell.ColorRed)}
	if got := set.EmptyText(); got != "nothing" {
		t.Fatalf("EmptyText = %q", got)
	}
	if fg, _, _ := set.PanelEmptyStyle().Decompose(); fg != tcell.ColorRed {
		t.Fatalf("fg = %v", fg)
	}
	def := Default()
	if def.EmptyText() != "[empty]" {
		t.Fatalf("default EmptyText = %q", def.EmptyText())
	}
	if fg, _, _ := def.PanelEmptyStyle().Decompose(); fg != tcell.PaletteColor(8) {
		t.Fatalf("default fg = %v", fg)
	}
}

func TestResolveColorUsesPalette(t *testing.T) {
	th := Default()
	if c, err := th.ResolveColor("bright_black"); err != nil || c != tcell.PaletteColor(8) {
		t.Fatalf("bright_black = %v, %v", c, err)
	}
	if c, err := th.ResolveColor("#ff8800"); err != nil || c != tcell.GetColor("#ff8800") {
		t.Fatalf("#ff8800 = %v, %v", c, err)
	}
	if _, err := th.ResolveColor("nonesuch"); err == nil {
		t.Fatal("unknown color resolved")
	}
}

func TestPreviewSearchIconStyle(t *testing.T) {
	row := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlue)
	if got := (Theme{}).PreviewSearchIconStyle(row); got != row {
		t.Errorf("PreviewSearchIconStyle without the key = %v, want the row style", got)
	}
	th := Theme{PreviewSearchIcon: tcell.StyleDefault.Foreground(tcell.ColorTeal).Background(tcell.ColorRed)}
	fg, bg, _ := th.PreviewSearchIconStyle(row).Decompose()
	if fg != tcell.ColorTeal || bg != tcell.ColorBlue {
		t.Errorf("PreviewSearchIconStyle = (%v, %v), want teal on the row's blue (key bg ignored)", fg, bg)
	}
	styles, err := parse(testTheme(t, "custom", nil, nil))
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	if styles.PreviewSearchIcon != (tcell.Style{}) {
		t.Errorf("PreviewSearchIcon = %v, want zero for a theme without the key", styles.PreviewSearchIcon)
	}
	if Default().PreviewSearchIcon == (tcell.Style{}) {
		t.Error("default theme should define preview.search_icon")
	}
}

func TestTooltipStylesFallBackWhenThemeOmitsThem(t *testing.T) {
	styles, err := parse(testTheme(t, "custom", nil, nil))
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	if styles.TooltipText != styles.DialogCompletionItem {
		t.Errorf("TooltipText = %v, want the dialog.completion.item fallback", styles.TooltipText)
	}
	wantFG, _, _ := styles.DialogAccent.Decompose()
	_, wantBG, _ := styles.DialogCompletionItem.Decompose()
	if fg, bg, _ := styles.TooltipInfoStyle().Decompose(); fg != wantFG || bg != wantBG {
		t.Errorf("TooltipInfoStyle = (%v, %v), want the accent foreground on the tooltip box", fg, bg)
	}
}

func TestPanelMetaAlertStylesFallBack(t *testing.T) {
	data := testTheme(t, "metaalertfallback", map[string]bool{"panel.meta.warn": true, "panel.meta.critical": true}, nil)
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fg, _, _ := th.PanelMetaWarn.Decompose(); fg != tcell.NewRGBColor(0xff, 0xff, 0) {
		t.Fatalf("warn fg = %v, want the warning role (test palette yellow)", fg)
	}
	if fg, _, attrs := th.PanelMetaCritical.Decompose(); fg != tcell.PaletteColor(1) || attrs&tcell.AttrBold == 0 {
		t.Fatalf("critical fg = %v attrs = %v, want bold danger role (ANSI red)", fg, attrs)
	}
}

func TestBrokenLinkIconFallsBackToDangerRole(t *testing.T) {
	data, err := themes.Files.ReadFile("default.toml")
	if err != nil {
		t.Fatalf("ReadFile(default.toml): %v", err)
	}
	withKey, err := parse(data)
	if err != nil {
		t.Fatalf("parse(default): %v", err)
	}
	const line = "broken_link = { fg = \"danger\" }"
	if !strings.Contains(string(data), line) {
		t.Fatalf("default.toml has no %q", line)
	}
	without, err := parse([]byte(strings.Replace(string(data), line, "", 1)))
	if err != nil {
		t.Fatalf("parse(without key): %v", err)
	}
	if without.PanelIconBrokenLink != withKey.PanelIconBrokenLink {
		t.Fatalf("fallback = %v, want the default theme's %v", without.PanelIconBrokenLink, withKey.PanelIconBrokenLink)
	}
}

func TestParseRejectsUnknownRoleKey(t *testing.T) {
	_, err := parse(testTheme(t, "custom", nil, map[string]string{"roles.nonesuch": `"red"`}))
	if err == nil || !strings.Contains(err.Error(), "roles.nonesuch: unknown role") {
		t.Fatalf("parse() error = %v, want unknown role", err)
	}
}

func TestParseRejectsUnknownPaletteKey(t *testing.T) {
	_, err := parse(testTheme(t, "custom", nil, map[string]string{"palette.fg": "7"}))
	if err == nil || !strings.Contains(err.Error(), "palette.fg: unknown color name") {
		t.Fatalf("parse() error = %v, want unknown palette key", err)
	}
}

func TestParseRejectsRoleValueNotAPaletteName(t *testing.T) {
	_, err := parse(testTheme(t, "custom", nil, map[string]string{"roles.accent": `"fg"`}))
	if err == nil || !strings.Contains(err.Error(), "roles.accent") {
		t.Fatalf("parse() error = %v, want role value error", err)
	}
}

func TestParseRejectsHueHexAndDefaultInStyles(t *testing.T) {
	for _, value := range []string{"yellow", "#112233", "default", "bright_black"} {
		for _, field := range []string{"fg", "bg"} {
			data := testTheme(t, "custom", nil, map[string]string{
				"menu.bar.active": fmt.Sprintf(`{ %s = %q }`, field, value),
			})
			_, err := parse(data)
			want := fmt.Sprintf(`style "menu.bar.active": %s %q is not a role; use one of`, field, value)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%s = %q: parse() error = %v, want %q", field, value, err, want)
			}
		}
	}
	data := testTheme(t, "custom", nil, map[string]string{
		"menu.bar.active": `{ fg = "fg", bg = "bg", icon = "red" }`,
	})
	if _, err := parse(data); err == nil {
		t.Error("icon naming a hue parsed")
	}
	data = testTheme(t, "custom", nil, map[string]string{
		"menu.progress.done.gfx.0": `{ fg = "yellow" }`,
	})
	if _, err := parse(data); err == nil || !strings.Contains(err.Error(), `fg "yellow" is not a role`) {
		t.Errorf("gfx hue: parse() error = %v", err)
	}
}

func TestRolesFallBackToDefaultPaletteNames(t *testing.T) {
	th, err := parse(testTheme(t, "custom", nil, map[string]string{
		"palette.bright_blue": `"#123456"`,
		"roles.surface":       `"#abcdef"`,
	}))
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	if got := th.roles["highlight_bg"]; got != tcell.NewRGBColor(0x12, 0x34, 0x56) {
		t.Errorf("highlight_bg = %v, want its default palette name bright_blue (#123456)", got)
	}
	if got := th.roles["surface"]; got != tcell.NewRGBColor(0xab, 0xcd, 0xef) {
		t.Errorf("surface = %v, want the explicit hex", got)
	}
	if got := th.roles["error"]; got != tcell.PaletteColor(9) {
		t.Errorf("error = %v, want ANSI 9 (bright_red)", got)
	}
	if len(th.roles) != len(roleDefs) {
		t.Errorf("%d roles, want %d", len(th.roles), len(roleDefs))
	}
}

// Optional-style fallbacks resolve through roles, so recoloring a role recolors them.
func TestOptionalStyleFallbackFollowsRoles(t *testing.T) {
	th, err := parse(testTheme(t, "custom", nil, map[string]string{
		"roles.danger": `"#102030"`,
		"roles.muted":  `"#405060"`,
	}))
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	if fg, _, _ := th.PanelMetaCritical.Decompose(); fg != tcell.NewRGBColor(0x10, 0x20, 0x30) {
		t.Errorf("PanelMetaCritical fg = %v, want the danger role", fg)
	}
	if fg, _, _ := th.PanelIconBrokenLink.Decompose(); fg != tcell.NewRGBColor(0x10, 0x20, 0x30) {
		t.Errorf("PanelIconBrokenLink fg = %v, want the danger role", fg)
	}
	if fg, _, _ := th.MenuStatusSeparator.Decompose(); fg != tcell.NewRGBColor(0x40, 0x50, 0x60) {
		t.Errorf("MenuStatusSeparator fg = %v, want the muted role", fg)
	}
}

func TestResolveColorRolesThenPaletteThenHex(t *testing.T) {
	th, err := parse(testTheme(t, "custom", nil, map[string]string{
		"palette.red":  `"#010101"`,
		"roles.danger": `"#020202"`,
		"roles.accent": `"red"`,
	}))
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	for value, want := range map[string]tcell.Color{
		"danger":  tcell.NewRGBColor(2, 2, 2), // role
		"accent":  tcell.NewRGBColor(1, 1, 1), // role naming a palette color
		"red":     tcell.NewRGBColor(1, 1, 1), // palette name
		"#ff8800": tcell.GetColor("#ff8800"),  // hex
		"default": tcell.ColorDefault,
	} {
		if got, err := th.ResolveColor(value); err != nil || got != want {
			t.Errorf("ResolveColor(%q) = %v, %v; want %v", value, got, err, want)
		}
	}
	if _, err := th.ResolveColor("nonesuch"); err == nil {
		t.Error("unknown color resolved")
	}
}

// themes/default.toml lists every role explicitly, equal to the built-in default table.
func TestDefaultThemeRolesEqualGoTable(t *testing.T) {
	data, err := themes.Files.ReadFile("default.toml")
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Palette map[string]any    `toml:"palette"`
		Roles   map[string]string `toml:"roles"`
	}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		t.Fatal(err)
	}
	if want := RoleDefaults(); !reflect.DeepEqual(raw.Roles, want) {
		t.Errorf("default.toml [roles] = %v\nGo table      = %v", raw.Roles, want)
	}
	if len(raw.Palette) != len(ansiNames) {
		t.Errorf("default.toml [palette] has %d entries, want %d", len(raw.Palette), len(ansiNames))
	}
}

// Every embedded theme loads: default.toml in CI, plus the generated themes in local builds.
func TestEveryEmbeddedThemeLoads(t *testing.T) {
	entries, err := themes.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := themes.Files.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parse(data); err != nil {
			t.Errorf("%s: %v", e.Name(), err)
		}
	}
}
