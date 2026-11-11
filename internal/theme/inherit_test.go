package theme

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// schemaIconKeys is every [icons] key the package reads: the IconKey* constants plus the key
// families built from a status or state name.
func schemaIconKeys(t *testing.T) []string {
	t.Helper()
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`IconKey\w+\s+=\s+"([^"]+)"`)
	var keys []string
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			keys = append(keys, m[1])
		}
	}
	for _, s := range fileIndexStates {
		keys = append(keys, IconFileIndexStateKey(s.key))
	}
	for _, status := range []string{"scanning", "queued", "running", "paused", "canceling", "canceled", "failed", "decision", "completed"} {
		keys = append(keys, "jobs.list."+status)
	}
	for _, status := range []string{"scanning", "queued", "running", "paused", "canceling", "failed", "decision", "completed"} {
		keys = append(keys, "menu.job."+status)
	}
	for _, r := range []string{JobsOperationSuccess, JobsOperationFailed, JobsOperationSkipped, JobsOperationSourceDeleted} {
		keys = append(keys, "jobs.operations."+r)
	}
	return keys
}

// TestDefaultDefinesEverySchemaKey keeps themes/default.toml, the base every theme inherits from,
// complete: every style key, one gradient per trail and every icon key.
func TestDefaultDefinesEverySchemaKey(t *testing.T) {
	base, err := defaultSource()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range styleKeys {
		if _, ok := base.specs[key]; !ok {
			t.Errorf("default.toml lacks style %q", key)
		}
	}
	for _, trail := range gfxTrails {
		if _, ok := base.specs[trail+".0"]; !ok {
			t.Errorf("default.toml lacks gradient %q", trail+".0")
		}
	}
	for _, key := range schemaIconKeys(t) {
		if strings.TrimSpace(base.icons[key]) == "" {
			t.Errorf("default.toml lacks icon %q", key)
		}
	}
	for key := range base.specs {
		if !styleKeySet[key] && gfxTrailOf(key) == "" {
			t.Errorf("default.toml defines style %q outside the schema", key)
		}
	}
}

func styleOf(t *testing.T, th Theme, key string) tcell.Style {
	t.Helper()
	switch key {
	case "footer.key":
		return th.FooterKey
	case "menu.dropdown":
		return th.MenuDropdown
	case "menu.dropdown.accent":
		return th.MenuDropdownAccent
	}
	t.Fatalf("styleOf: unsupported key %q", key)
	return tcell.Style{}
}

func TestParseInheritsEverythingFromDefault(t *testing.T) {
	got, err := parse([]byte("name = \"bare\"\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := Default()
	want.Name = "bare"
	assertThemesEqual(t, got, want)
}

func dumpTheme(th Theme) string { return fmt.Sprintf("%+v", th) }

func assertThemesEqual(t *testing.T, got, want Theme) {
	t.Helper()
	if g, w := dumpTheme(got), dumpTheme(want); g != w {
		t.Fatalf("themes differ:\n got %s\nwant %s", g, w)
	}
}

func TestParseOverlayReplacesStylesWhole(t *testing.T) {
	got, err := parse([]byte(`name = "over"
[footer]
key = { fg = "fg" }
[menu]
dropdown = { fg = "ink" }
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	def := Default()
	if _, _, attrs := styleOf(t, got, "footer.key").Decompose(); attrs&tcell.AttrBold != 0 {
		t.Fatal("footer.key kept the default's bold: a style replaces the inherited one whole")
	}
	if _, bg, _ := styleOf(t, got, "menu.dropdown").Decompose(); bg != tcell.ColorDefault {
		t.Fatalf("menu.dropdown bg = %v, want none (whole replacement)", bg)
	}
	// menu.dropdown is both a leaf and a parent: its children stay inherited.
	assertStyleEqual(t, "menu.dropdown.accent", styleOf(t, got, "menu.dropdown.accent"), styleOf(t, def, "menu.dropdown.accent"))
}

func TestParseOverlayPaletteRolesAndIcons(t *testing.T) {
	got, err := parse([]byte(`name = "over"
[palette]
red = "#112233"
[roles]
accent = "red"
[icons.tree]
branch = "+-"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	def := Default()
	if c, _ := got.ResolveColor("red"); c != tcell.NewRGBColor(0x11, 0x22, 0x33) {
		t.Fatalf("red = %v", c)
	}
	if c, _ := got.ResolveColor("accent"); c != tcell.NewRGBColor(0x11, 0x22, 0x33) {
		t.Fatalf("accent = %v", c)
	}
	if c, _ := got.ResolveColor("blue"); c != mustColor(t, def, "blue") {
		t.Fatalf("blue = %v, want inherited", c)
	}
	if got.IconTreeBranch() != "+-" || got.IconTreeEnd() != def.IconTreeEnd() {
		t.Fatalf("icons: branch %q end %q", got.IconTreeBranch(), got.IconTreeEnd())
	}
}

func mustColor(t *testing.T, th Theme, name string) tcell.Color {
	t.Helper()
	c, err := th.ResolveColor(name)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseOverlayGfxTrailReplacesWhole(t *testing.T) {
	def := Default()
	if len(def.MenuProgressDeleteGfx) < 2 {
		t.Fatalf("default delete trail has %d entries, test needs at least 2", len(def.MenuProgressDeleteGfx))
	}
	got, err := parse([]byte(`name = "over"
[menu.progress]
delete.gfx.0 = { fg = "fg" }
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got.MenuProgressDeleteGfx) != 1 {
		t.Fatalf("delete trail has %d entries, want 1 (a defined trail replaces the inherited one)", len(got.MenuProgressDeleteGfx))
	}
	if len(got.MenuProgressDoneGfx) != len(def.MenuProgressDoneGfx) {
		t.Fatalf("done trail has %d entries, want the inherited %d", len(got.MenuProgressDoneGfx), len(def.MenuProgressDoneGfx))
	}
}

func TestParseOverlayStaysStrict(t *testing.T) {
	for name, body := range map[string]string{
		"unknown style": "[footer]\nnope = { fg = \"fg\" }\n",
		"hue colour":    "[footer]\nkey = { fg = \"yellow\" }\n",
		"unknown field": "extra = 1\n",
		"unknown role":  "[roles]\nnope = \"red\"\n",
	} {
		if _, err := parse([]byte("name = \"bad\"\n" + body)); err == nil {
			t.Errorf("%s: parse accepted the theme", name)
		}
	}
}
