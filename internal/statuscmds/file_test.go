package statuscmds

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
)

func TestDecodeClampsAndDefaults(t *testing.T) {
	f, err := Decode([]byte(`
[[command]]
name = "harbor"
command = "echo lantern"
interval_ms = 10
max_width = 500

[[command]]
name = "meadow"
label = "Meadow clock"
command = "echo orchard"
`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	h := f.Commands[0]
	if h.Label != "harbor" || h.IntervalMS != config.DefaultStatusCommandIntervalMS || h.MaxWidth != config.StatusCommandMaxWidthMax {
		t.Fatalf("harbor = %+v, want label=name, default interval, max width clamp", h)
	}
	m := f.Commands[1]
	if m.Label != "Meadow clock" || m.MaxWidth != config.DefaultStatusCommandMaxWidth {
		t.Fatalf("meadow = %+v", m)
	}
}

func TestDecodeRejectsDuplicateAndMissingFields(t *testing.T) {
	for _, src := range []string{
		"[[command]]\ncommand = \"echo x\"\n",
		"[[command]]\nname = \"pebble\"\n",
		"[[command]]\nname = \"pebble\"\ncommand = \"a\"\n[[command]]\nname = \"pebble\"\ncommand = \"b\"\n",
	} {
		if _, err := Decode([]byte(src)); err == nil {
			t.Fatalf("Decode(%q) = nil error", src)
		}
	}
}

func TestResolveOverridesBuiltinAndFiltersShow(t *testing.T) {
	f := &File{
		Show: []string{"tundra", "memory", "loadavg", "memory"},
		Commands: []Def{
			{Name: "loadavg", Label: "Load", Command: "echo quiver", IntervalMS: 1000, MaxWidth: 9},
			{Name: "tundra", Label: "tundra", Command: "echo glacier", IntervalMS: 1000, MaxWidth: 9},
		},
	}
	s := Resolve(f)
	if s.Defs[0].Name != "loadavg" || s.Defs[0].Command != "echo quiver" {
		t.Fatalf("Defs[0] = %+v, want overridden loadavg in place", s.Defs[0])
	}
	if last := s.Defs[len(s.Defs)-1]; last.Name != "tundra" {
		t.Fatalf("last def = %q, want user-only tundra appended", last.Name)
	}
	if want := []string{"tundra", "memory", "loadavg"}; !reflect.DeepEqual(s.Show, want) {
		t.Fatalf("Show = %v, want %v", s.Show, want)
	}
	if got := s.Shown(); len(got) != 3 || got[0].Command != "echo glacier" {
		t.Fatalf("Shown = %+v", got)
	}
	if got := Resolve(&File{Show: []string{"nonesuch"}}).Show; len(got) != 0 {
		t.Fatalf("unknown show name kept: %v", got)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "status.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Show) != 0 || len(s.Defs) != len(Builtins()) {
		t.Fatalf("Load missing = %+v", s)
	}
}

func TestStubDecodesEmpty(t *testing.T) {
	f, err := Decode([]byte(StatusStubTOML))
	if err != nil {
		t.Fatalf("Decode stub: %v", err)
	}
	if len(f.Show) != 0 || len(f.Commands) != 0 {
		t.Fatalf("stub = %+v, want empty", f)
	}
}

func TestSaveShowCreatesStubAndKeepsCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status.toml")
	if err := SaveShow(path, []string{"memory"}); err != nil {
		t.Fatalf("SaveShow: %v", err)
	}
	b, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(b), StatusStubTOML+"show = [\"memory\"]\n") {
		t.Fatalf("created file:\n%s", b)
	}

	user := "show = [\n  \"loadavg\",\n  \"walnut\",\n]\n\n[[command]]\nname = \"walnut\"\ncommand = \"echo show = [\\\"x\\\"]\"\n"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveShow(path, []string{"walnut", "battery"}); err != nil {
		t.Fatalf("SaveShow: %v", err)
	}
	b, _ = os.ReadFile(path)
	want := "show = [\"walnut\", \"battery\"]\n\n[[command]]\nname = \"walnut\"\ncommand = \"echo show = [\\\"x\\\"]\"\n"
	if string(b) != want {
		t.Fatalf("rewritten:\n%s\nwant:\n%s", b, want)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(s.Show, []string{"walnut", "battery"}) {
		t.Fatalf("Show = %v", s.Show)
	}

	if err := SaveShow(path, nil); err != nil {
		t.Fatalf("SaveShow none: %v", err)
	}
	if s, _ := Load(path); len(s.Show) != 0 {
		t.Fatalf("Show after clearing = %v", s.Show)
	}
}
