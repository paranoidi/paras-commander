package columnprofile

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestBuiltinsMatchColumnSets(t *testing.T) {
	for _, want := range []struct {
		name string
		set  panel.ColumnSet
	}{
		{config.ColumnProfileModified, panel.ColumnsModified},
		{config.ColumnProfilePermissions, panel.ColumnsPermissions},
		{config.ColumnProfileBrief, panel.ColumnsBrief},
		{config.ColumnProfileIndex, panel.ColumnsBrief},
	} {
		p, ok := Find(nil, want.name)
		if !ok || !p.Builtin {
			t.Fatalf("%s: found %v builtin %v", want.name, ok, p.Builtin)
		}
		if set, unknown := p.Set(); set != want.set || len(unknown) != 0 {
			t.Fatalf("%s: set %v, unknown %v", want.name, set.Names(), unknown)
		}
	}
}

func TestCycleOrderSkipsIndexAndFollowsFile(t *testing.T) {
	saved := []Profile{{Name: "orchard"}, {Name: "harbor"}}
	want := []string{config.ColumnProfileModified, config.ColumnProfilePermissions, config.ColumnProfileBrief, "orchard", "harbor"}
	cycle := Cycle(saved)
	if len(cycle) != len(want) {
		t.Fatalf("cycle = %+v", cycle)
	}
	for i, name := range want {
		if cycle[i].Name != name {
			t.Fatalf("cycle[%d] = %q, want %q", i, cycle[i].Name, name)
		}
	}
	if got := Next(saved, "harbor").Name; got != config.ColumnProfileModified {
		t.Fatalf("after the last = %q", got)
	}
	if got := Next(saved, "").Name; got != config.ColumnProfileModified {
		t.Fatalf("after none = %q", got)
	}
	if got := Next(saved, config.ColumnProfileIndex).Name; got != config.ColumnProfileModified {
		t.Fatalf("after a skipped profile = %q", got)
	}
}

func TestAllDropsSavedProfileNamedLikeBuiltin(t *testing.T) {
	got, ok := Find([]Profile{{Name: config.ColumnProfileBrief, Columns: []string{"owner"}}}, config.ColumnProfileBrief)
	if !ok || !got.Builtin {
		t.Fatalf("got %+v, want the built-in", got)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", config.DefaultColumnProfilesFileName)
	if saved, err := Load(path); err != nil || saved != nil {
		t.Fatalf("missing file = %+v, %v", saved, err)
	}
	if err := Upsert(path, Profile{Name: "orchard", Description: "first", Columns: []string{"owner"}, Meta: []string{"meadow"}}); err != nil {
		t.Fatal(err)
	}
	if err := Upsert(path, Profile{Name: "harbor"}); err != nil {
		t.Fatal(err)
	}
	if err := Upsert(path, Profile{Name: "orchard", Description: "second", Columns: []string{"group"}}); err != nil {
		t.Fatal(err)
	}
	saved, err := Load(path)
	if err != nil || len(saved) != 2 || saved[0].Description != "second" || saved[0].Columns[0] != "group" {
		t.Fatalf("saved = %+v, err = %v", saved, err)
	}
	if err := Remove(path, "orchard"); err != nil {
		t.Fatal(err)
	}
	saved, err = Load(path)
	if err != nil || len(saved) != 1 || saved[0].Name != "harbor" {
		t.Fatalf("after remove = %+v, %v", saved, err)
	}
}

func TestBuiltinNamesAreRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.DefaultColumnProfilesFileName)
	if err := Upsert(path, Profile{Name: config.ColumnProfileBrief}); err == nil {
		t.Fatal("Upsert of a built-in name must fail")
	}
	if err := Remove(path, config.ColumnProfileBrief); err == nil {
		t.Fatal("Remove of a built-in name must fail")
	}
}

func TestResolveFile(t *testing.T) {
	if got := ResolveFile("  /srv/lantern.toml ", "/cfg"); got != "/srv/lantern.toml" {
		t.Fatalf("explicit = %q", got)
	}
	if got := ResolveFile("", "/cfg"); got != filepath.Join("/cfg", config.DefaultColumnProfilesFileName) {
		t.Fatalf("default = %q", got)
	}
	if got := ResolveFile("", ""); got != "" {
		t.Fatalf("unknown = %q", got)
	}
	if err := Save("", nil); err == nil {
		t.Fatal("Save with no location must fail")
	}
}
