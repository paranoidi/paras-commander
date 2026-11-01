package findcrit

import (
	"io/fs"
	"regexp"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/search"
)

func TestParseAge(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		in   string
		want time.Time
	}{
		{"", time.Time{}},
		{"30m", now.Add(-30 * time.Minute)},
		{"2h", now.Add(-2 * time.Hour)},
		{"3d", now.Add(-72 * time.Hour)},
		{"2w", now.Add(-14 * 24 * time.Hour)},
		{"6mo", now.Add(-180 * 24 * time.Hour)},
		{"1y", now.Add(-365 * 24 * time.Hour)},
		{"1w3d", now.Add(-10 * 24 * time.Hour)},
		{"1.5d", now.Add(-36 * time.Hour)},
		{" 3D ", now.Add(-72 * time.Hour)},
		{"2026-10-01", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		{"2026-10-01 14:30", time.Date(2026, 10, 1, 14, 30, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		got, err := ParseAge(tt.in, now)
		if err != nil {
			t.Fatalf("ParseAge(%q) error: %v", tt.in, err)
		}
		want := int64(0)
		if !tt.want.IsZero() {
			want = tt.want.UnixNano()
		}
		if got != want {
			t.Errorf("ParseAge(%q) = %v, want %v", tt.in, time.Unix(0, got).UTC(), tt.want)
		}
	}
	for _, bad := range []string{"3", "d", "3x", "yesterday", "2026-13-01", "3d 2h"} {
		if _, err := ParseAge(bad, now); err == nil {
			t.Errorf("ParseAge(%q): want error", bad)
		}
	}
}

func TestParseSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in     string
		want   int64
		wantOK bool
	}{
		{"", 0, false},
		{"0", 0, true},
		{"512", 512, true},
		{"10k", 10 << 10, true},
		{"10K", 10 << 10, true},
		{"4M", 4 << 20, true},
		{"4mb", 4 << 20, true},
		{"4MiB", 4 << 20, true},
		{"1.5G", 3 << 29, true},
		{"2 T", 2 << 40, true},
	}
	for _, tt := range tests {
		got, ok, err := ParseSize(tt.in)
		if err != nil || got != tt.want || ok != tt.wantOK {
			t.Errorf("ParseSize(%q) = %d, %v, %v; want %d, %v", tt.in, got, ok, err, tt.want, tt.wantOK)
		}
	}
	for _, bad := range []string{"k", "10x", "-5", "1e3", "ten"} {
		if _, _, err := ParseSize(bad); err == nil {
			t.Errorf("ParseSize(%q): want error", bad)
		}
	}
}

func TestCriteriaMatch(t *testing.T) {
	t.Parallel()
	day := int64(24 * time.Hour)
	now := int64(100) * day
	file := func(name string, size int64, mod int64, mode fs.FileMode, uid uint32) Subject {
		return Subject{Name: name, Size: size, Meta: &Meta{ModTime: mod, ChangeTime: mod, AccessTime: mod, Mode: mode, UID: uid}}
	}
	recent := file("lantern.go", 2048, now-day, 0o644, 1000)
	old := file("meadow.txt", 0, now-30*day, 0o644, 0)
	script := file("harbor.sh", 300, now-2*day, 0o755, 1000)
	dir := Subject{Name: "orchard", IsDir: true, Meta: &Meta{ModTime: now, Mode: fs.ModeDir | 0o755, EmptyDir: true}}
	link := Subject{Name: "willow", Symlink: true, Broken: true, Meta: &Meta{ModTime: now, Mode: fs.ModeSymlink | 0o777}}
	unstatted := Subject{Name: "pebble.go"}

	tests := []struct {
		name string
		c    Criteria
		in   Subject
		want bool
	}{
		{"zero passes", Criteria{}, unstatted, true},
		{"newer keeps recent", Criteria{Modified: TimeBound{Newer: now - 7*day}}, recent, true},
		{"newer drops old", Criteria{Modified: TimeBound{Newer: now - 7*day}}, old, false},
		{"older keeps old", Criteria{Modified: TimeBound{Older: now - 7*day}}, old, true},
		{"older drops recent", Criteria{Modified: TimeBound{Older: now - 7*day}}, recent, false},
		{"window", Criteria{Changed: TimeBound{Newer: now - 3*day, Older: now - day/2}}, script, true},
		{"no meta fails stat criterion", Criteria{Modified: TimeBound{Newer: 1}}, unstatted, false},
		{"min size", Criteria{MinSize: 1024}, recent, true},
		{"min size drops small", Criteria{MinSize: 1024}, script, false},
		{"max size", Criteria{MaxSize: 512, HasMaxSize: true}, script, true},
		{"max size zero keeps empty", Criteria{HasMaxSize: true}, old, true},
		{"size drops dirs", Criteria{HasMaxSize: true, MaxSize: 1 << 30}, dir, false},
		{"empty file", Criteria{Empty: true}, old, true},
		{"empty drops non-empty", Criteria{Empty: true}, recent, false},
		{"empty dir", Criteria{Empty: true}, dir, true},
		{"empty drops symlink", Criteria{Empty: true}, link, false},
		{"executables", Criteria{Executables: true}, script, true},
		{"executables drop plain", Criteria{Executables: true}, recent, false},
		{"symlinks", Criteria{Symlinks: true}, link, true},
		{"broken symlinks", Criteria{BrokenSymlinks: true}, link, true},
		{"kinds are alternatives", Criteria{Symlinks: true, Executables: true}, script, true},
		{"owner", Criteria{Owner: 1000, HasOwner: true}, recent, true},
		{"owner drops other", Criteria{Owner: 1000, HasOwner: true}, old, false},
		{"not mine", Criteria{NotMine: true, MyUID: 1000}, old, true},
		{"not mine drops mine", Criteria{NotMine: true, MyUID: 1000}, recent, false},
		{"types", Criteria{Types: regexp.MustCompile(`\.go$`)}, recent, true},
		{"types drop other", Criteria{Types: regexp.MustCompile(`\.go$`)}, old, false},
		{"types drop dirs", Criteria{Types: regexp.MustCompile(`.`)}, dir, false},
	}
	for _, tt := range tests {
		if got := tt.c.Match(tt.in); got != tt.want {
			t.Errorf("%s: Match = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestCriteriaNeedsStat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		c    Criteria
		want StatLevel
	}{
		{Criteria{}, StatNone},
		{Criteria{Symlinks: true, BrokenSymlinks: true, Types: regexp.MustCompile("x")}, StatNone},
		{Criteria{Accessed: TimeBound{Older: 5}}, StatMeta},
		{Criteria{MinSize: 1}, StatMeta},
		{Criteria{Executables: true}, StatMeta},
		{Criteria{NotMine: true}, StatMeta},
		{Criteria{Empty: true}, StatMetaEmptyDirs},
	}
	for i, tt := range tests {
		if got := tt.c.NeedsStat(); got != tt.want {
			t.Errorf("case %d: NeedsStat = %d, want %d", i, got, tt.want)
		}
		if got := tt.c.Active(); got != (i > 0) {
			t.Errorf("case %d: Active = %v", i, got)
		}
	}
}

func TestCompileGlob(t *testing.T) {
	t.Parallel()
	tests := []struct {
		query, rel string
		want       bool
	}{
		{"*.go", "amber/lantern.go", true},
		{"*.go", "amber/lantern.go.bak", false},
		{"*.GO", "amber/lantern.go", true}, // case-insensitive
		{"lant", "amber/lantern.go", true}, // a plain word finds names containing it
		{"amber", "amber/lantern.go", false},
		{"amber/*.go", "amber/lantern.go", true},
		{"*/lantern.go", "amber/birch/lantern.go", true}, // '*' crosses directories with a '/'
		{"birch/*", "amber/birch/lantern.go", false},
		{"l?ntern.*", "lantern.md", true},
		{"[a-c]*", "birch", true},
	}
	for _, tt := range tests {
		m, err := Compile(tt.query, ModeGlob, true)
		if err != nil {
			t.Fatalf("Compile(%q): %v", tt.query, err)
		}
		if got := m.Match(tt.rel).Matched; got != tt.want {
			t.Errorf("glob %q on %q = %v, want %v", tt.query, tt.rel, got, tt.want)
		}
	}
	if _, err := Compile("[", ModeGlob, true); err == nil {
		t.Error("Compile(\"[\"): want error")
	}
	m, _ := Compile("*.go", ModeGlob, true)
	if got := m.Match("amber/lantern.go").Ranges; len(got) != 1 || got[0] != (search.Range{Start: 6, End: 16}) {
		t.Errorf("glob ranges = %v, want the basename", got)
	}
}

func TestCompileRegex(t *testing.T) {
	t.Parallel()
	m, err := Compile(`_test\.go$`, ModeRegex, true)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("amber/lantern_test.go").Matched || m.Match("amber/lantern.go").Matched {
		t.Error("regex match wrong")
	}
	if m.Ranked() {
		t.Error("regex results are in walk order")
	}
	m, _ = Compile("an", ModeRegex, false)
	got := m.Match("äamber/lantern").Ranges
	want := []search.Range{{Start: 8, End: 10}} // bytes 9..11: "ä" is two bytes
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("regex ranges = %v, want %v (rune offsets)", got, want)
	}
	if m.Match("LANTERN").Matched {
		t.Error("case-sensitive regex matched other case")
	}
	if _, err := Compile("(", ModeRegex, true); err == nil {
		t.Error("Compile(\"(\"): want error")
	}
}

func TestCompileEmptyAndFuzzy(t *testing.T) {
	t.Parallel()
	for _, mode := range []Mode{ModeFuzzy, ModeGlob, ModeRegex} {
		m, err := Compile("  ", mode, true)
		if err != nil || !m.Empty() || !m.Match("anything").Matched {
			t.Errorf("mode %d: empty query should match everything", mode)
		}
	}
	m, _ := Compile("lntrn", ModeFuzzy, true)
	if !m.Ranked() || !m.Match("amber/lantern.go").Matched {
		t.Error("fuzzy query should rank and match")
	}
	if ModeRegex.Next() != ModeFuzzy || ModeFuzzy.Next() != ModeGlob {
		t.Error("Next does not cycle")
	}
}

func TestExcludes(t *testing.T) {
	t.Parallel()
	ex, err := ParseExcludes("node_modules, *.o  build/out")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		rel  string
		want bool
	}{
		{"node_modules", true},
		{"amber/node_modules", true},
		{"amber/lantern.o", true},
		{"amber/lantern.go", false},
		{"build/out", true},
		{"amber/build/out", false},
	}
	for _, tt := range tests {
		if got := ex.Match(tt.rel); got != tt.want {
			t.Errorf("Match(%q) = %v, want %v", tt.rel, got, tt.want)
		}
	}
	if _, err := ParseExcludes("[bad"); err == nil {
		t.Error("ParseExcludes([bad): want error")
	}
	if ex, _ := ParseExcludes(" , "); ex != nil || ex.Match("x") {
		t.Error("blank excludes should be nil and match nothing")
	}
}
