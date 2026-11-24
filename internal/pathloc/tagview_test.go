package pathloc

import (
	"reflect"
	"testing"
)

func TestTagViewParseCanonical(t *testing.T) {
	tests := []struct {
		raw  string
		want string
		q    TagQuery
	}{
		{"fileindex(clover):///orchard", "fileindex(clover):///orchard", TagQuery{Tags: []string{"clover"}}},
		{"fileindex(thistle&clover&clover):///orchard/", "fileindex(clover&thistle):///orchard", TagQuery{Tags: []string{"clover", "thistle"}}},
		{"fileindex(thistle|clover):///", "fileindex(clover|thistle):///", TagQuery{Any: true, Tags: []string{"clover", "thistle"}}},
		{"fileindex(clover|clover):///", "fileindex(clover):///", TagQuery{Tags: []string{"clover"}}},
		{"fileindex@(clover):///", "fileindex@(clover):///", TagQuery{Flat: true, Tags: []string{"clover"}}},
		{"fileindex@(b|a):///", "fileindex@(a|b):///", TagQuery{Flat: true, Any: true, Tags: []string{"a", "b"}}},
		{"fileindex@(clover):////", "fileindex@(clover):///", TagQuery{Flat: true, Tags: []string{"clover"}}},
		{"fileindex(clover)://", "fileindex(clover):///", TagQuery{Tags: []string{"clover"}}},
		{"fileindex@(clover)://", "fileindex@(clover):///", TagQuery{Flat: true, Tags: []string{"clover"}}},
		{"fileindex(a%20b%2Fc%3Ad%28e%29%26f%7Cg%25h):///x", "fileindex(a%20b%2Fc%3Ad%28e%29%26f%7Cg%25h):///x",
			TagQuery{Tags: []string{"a b/c:d(e)&f|g%h"}}},
		{"fileindex(a b(c)):///x", "fileindex(a%20b%28c%29):///x", TagQuery{Tags: []string{"a b(c)"}}},
		{"fileindex(%41%c3%a4):///x", "fileindex(Aä):///x", TagQuery{Tags: []string{"Aä"}}},
		{"fileindex(über&天気):///x", "fileindex(über&天気):///x", TagQuery{Tags: []string{"über", "天気"}}},
		{"fileindex(a%7Cb&c):///x", "fileindex(a%7Cb&c):///x", TagQuery{Tags: []string{"a|b", "c"}}},
		{"fileindex(clover):///orchard/../lantern", "fileindex(clover):///lantern", TagQuery{Tags: []string{"clover"}}},
	}
	for _, tc := range tests {
		p, err := Parse(tc.raw)
		if err != nil {
			t.Errorf("Parse(%q): %v", tc.raw, err)
			continue
		}
		if p.String() != tc.want {
			t.Errorf("Parse(%q) = %q, want %q", tc.raw, p, tc.want)
		}
		if p.Scheme() != SchemeFileIndex || p.IsLocal() {
			t.Errorf("%q scheme = %s", tc.raw, p.Scheme())
		}
		q, ok := p.TagQuery()
		if !ok || !reflect.DeepEqual(q, tc.q) {
			t.Errorf("TagQuery(%q) = %+v, %v; want %+v", tc.raw, q, ok, tc.q)
		}
		again, err := Parse(p.String())
		if err != nil || !again.Equal(p) {
			t.Errorf("reparse of %q = %q, %v", p, again, err)
		}
		built, err := TagView(tc.q, hostOf(t, p))
		if err != nil || !built.Equal(p) {
			t.Errorf("TagView(%+v) = %q, %v; want %q", tc.q, built, err, p)
		}
	}
}

func hostOf(t *testing.T, p Path) string {
	t.Helper()
	_, host, ok := CutIndexPrefix(p.String())
	if !ok {
		t.Fatalf("no index prefix in %q", p)
	}
	return host
}

func TestTagViewParseErrors(t *testing.T) {
	for _, raw := range []string{
		"fileindex@://",
		"fileindex@:///x",
		"fileindex()://",
		"fileindex():///x",
		"fileindex@():///x",
		"fileindex(&):///x",
		"fileindex(clover&):///x",
		"fileindex(|clover):///x",
		"fileindex(clover&thistle|lantern):///x",
		"fileindex(clover%zz):///x",
		"fileindex(clover)://orchard",
		"fileindex@(clover)://orchard",
		"fileindex://orchard",
		"fileindex://",
	} {
		if p, err := Parse(raw); err == nil {
			t.Errorf("Parse(%q) = %q, want an error", raw, p)
		}
	}
}

func TestTagViewNotAnIndexPath(t *testing.T) {
	for _, raw := range []string{"fileindex(clover)", "fileindex", "fileindex(clover):/x", "fileindexed://x", "fileindex@(clover)", "fileindex@", "(clover)://"} {
		if _, _, ok := CutIndexPrefix(raw); ok {
			t.Errorf("CutIndexPrefix(%q) recognised a local name", raw)
		}
		if HasURIPrefix(raw) {
			t.Errorf("HasURIPrefix(%q) = true", raw)
		}
	}
	p := MustParse("fileindex(clover)")
	if p.Scheme() != SchemeFile {
		t.Fatalf("a local file name parsed as %s: %q", p.Scheme(), p)
	}
	if _, ok := p.TagQuery(); ok {
		t.Fatal("a local file has no tag query")
	}
}

func TestTagViewBuilderErrors(t *testing.T) {
	for _, q := range []TagQuery{{}, {Tags: []string{""}}, {Tags: []string{"clover", ""}}} {
		if p, err := TagView(q, "/"); err == nil {
			t.Errorf("TagView(%+v) = %q, want an error", q, p)
		}
	}
	if p, err := TagView(TagQuery{Tags: []string{"clover"}}, "orchard"); err == nil {
		t.Errorf("a relative host = %q, want an error", p)
	}
	in := TagQuery{Flat: true, Any: true, Tags: []string{"b", "a"}}
	if _, err := TagView(in, "/"); err != nil || in.Tags[0] != "b" {
		t.Errorf("TagView must not reorder the caller's tags: %v, %v", in.Tags, err)
	}
}

func TestPlainIndexHasNoTagQuery(t *testing.T) {
	for _, raw := range []string{"fileindex:///", "fileindex:///orchard", "/orchard", "sftp://harbor/x"} {
		p := MustParse(raw)
		if _, ok := p.TagQuery(); ok || p.IsTagList() {
			t.Errorf("%q is no tag view", raw)
		}
	}
}

func TestCutIndexPrefix(t *testing.T) {
	tests := []struct {
		raw, prefix, host string
		ok                bool
	}{
		{"fileindex:///orchard/lantern", "fileindex://", "/orchard/lantern", true},
		{"fileindex://", "fileindex://", "", true},
		{"fileindex(clover&thistle):///orchard", "fileindex(clover&thistle)://", "/orchard", true},
		{"fileindex(clover)://", "fileindex(clover)://", "/", true},
		{"fileindex@(clover):///", "fileindex@(clover)://", "/", true},
		{"fileindex@(clover):///orchard//lantern/meadow", "fileindex@(clover)://", "/orchard/lantern/meadow", true},
		{"fileindex@(clover):///orchard", "fileindex@(clover)://", "/orchard", true},
		{"fileindex(clover):///orchard//lantern", "fileindex(clover)://", "/orchard//lantern", true},
		{"fileindex@://", "fileindex@://", "/", true},
		{"/orchard", "", "", false},
		{"fileindex(x)", "", "", false},
		{"sftp://harbor/x", "", "", false},
		{"", "", "", false},
	}
	for _, tc := range tests {
		prefix, host, ok := CutIndexPrefix(tc.raw)
		if prefix != tc.prefix || host != tc.host || ok != tc.ok {
			t.Errorf("CutIndexPrefix(%q) = %q, %q, %v; want %q, %q, %v", tc.raw, prefix, host, ok, tc.prefix, tc.host, tc.ok)
		}
	}
	if n := testing.AllocsPerRun(100, func() { CutIndexPrefix("/home/wren/orchard/lantern"); CutIndexPrefix("fileindex:///orchard") }); n != 0 {
		t.Errorf("CutIndexPrefix allocates %v times on the plain forms", n)
	}
}

func TestFlatPathNormalises(t *testing.T) {
	tests := []struct{ raw, want string }{
		{"fileindex@(c):///orchard//lantern/meadow", "fileindex@(c):///orchard//lantern/meadow"},
		{"fileindex@(c):///orchard/./x/..//lantern//../meadow/", "fileindex@(c):///orchard//meadow"},
		{"fileindex@(c):///orchard//", "fileindex@(c):///orchard"},
		{"fileindex@(c):///orchard///", "fileindex@(c):///orchard"},
		{"fileindex@(c):////orchard", "fileindex@(c):///orchard"},
		{"fileindex@(c):////orchard/lantern", "fileindex@(c):///orchard/lantern"},
		{"fileindex@(c):///orchard//lantern//meadow", "fileindex@(c):///orchard//lantern/meadow"},
		{"fileindex@(c):///orchard//../..//x", "fileindex@(c):///orchard//x"},
		{"fileindex@(c):///orchard/thistle/", "fileindex@(c):///orchard/thistle"},
		{"fileindex(c):///orchard//lantern", "fileindex(c):///orchard/lantern"},
	}
	for _, tc := range tests {
		p, err := Parse(tc.raw)
		if err != nil || p.String() != tc.want {
			t.Errorf("Parse(%q) = %q, %v; want %q", tc.raw, p, err, tc.want)
		}
	}
}

func TestFlatListRoot(t *testing.T) {
	root := MustParse("fileindex@(clover):///")
	if !root.IsTagList() {
		t.Fatal("the flat root is the list")
	}
	for _, raw := range []string{"fileindex@(clover):///orchard", "fileindex@(clover):///orchard//lantern", "fileindex(clover):///", "fileindex:///"} {
		if MustParse(raw).IsTagList() {
			t.Errorf("%q is not the list", raw)
		}
	}
}

func TestTagViewPathMethods(t *testing.T) {
	const (
		plain = "fileindex://"
		str   = "fileindex(clover&thistle)://"
		flat  = "fileindex@(clover&thistle)://"
	)
	type call struct {
		raw    string
		parent string
		up     string
		base   string
		host   string
	}
	tests := []call{
		{plain + "/orchard/lantern", plain + "/orchard", plain + "/orchard", "lantern", "/orchard/lantern"},
		{plain + "/orchard", plain + "/", plain + "/", "orchard", "/orchard"},
		{plain + "/", plain + "/", plain + "/", "/", "/"},
		{str + "/orchard/lantern", str + "/orchard", str + "/orchard", "lantern", "/orchard/lantern"},
		{str + "/", str + "/", str + "/", "/", "/"},
		{flat + "/", flat + "/", flat + "/", "/", "/"},
		{flat + "/orchard", flat + "/", flat + "/", "orchard", "/orchard"},
		{flat + "/orchard/lantern", flat + "/orchard", flat + "/", "lantern", "/orchard/lantern"},
		{flat + "/orchard//meadow", flat + "/orchard", flat + "/orchard", "meadow", "/orchard/meadow"},
		{flat + "/orchard//meadow/pebble", flat + "/orchard//meadow", flat + "/orchard//meadow", "pebble", "/orchard/meadow/pebble"},
		{flat + "/orchard//meadow/pebble/reed", flat + "/orchard//meadow/pebble", flat + "/orchard//meadow/pebble", "reed", "/orchard/meadow/pebble/reed"},
	}
	for _, tc := range tests {
		p := MustParse(tc.raw)
		if got := p.Parent().String(); got != tc.parent {
			t.Errorf("Parent(%q) = %q, want %q", tc.raw, got, tc.parent)
		}
		if got := p.Up().String(); got != tc.up {
			t.Errorf("Up(%q) = %q, want %q", tc.raw, got, tc.up)
		}
		if got := p.Base(); got != tc.base {
			t.Errorf("Base(%q) = %q, want %q", tc.raw, got, tc.base)
		}
		if got, ok := p.HostPath(); !ok || got != tc.host {
			t.Errorf("HostPath(%q) = %q, %v; want %q", tc.raw, got, ok, tc.host)
		}
		if got, err := IndexHostPath(p); err != nil || got != tc.host {
			t.Errorf("IndexHostPath(%q) = %q, %v; want %q", tc.raw, got, err, tc.host)
		}
		if got := p.Local().String(); got != tc.host {
			t.Errorf("Local(%q) = %q, want %q", tc.raw, got, tc.host)
		}
	}
	if got := MustParse("/orchard/lantern").Up().String(); got != "/orchard" {
		t.Errorf("Up of a file path = %q", got)
	}
}

func TestTagViewJoin(t *testing.T) {
	tests := []struct{ raw, name, want string }{
		{"fileindex:///orchard", "lantern", "fileindex:///orchard/lantern"},
		{"fileindex(clover):///orchard", "lantern", "fileindex(clover):///orchard/lantern"},
		{"fileindex(clover):///", "lantern", "fileindex(clover):///lantern"},
		{"fileindex@(clover):///", "lantern", "fileindex@(clover):///lantern"},
		{"fileindex@(clover):///orchard", "lantern", "fileindex@(clover):///orchard//lantern"},
		{"fileindex@(clover):///orchard//lantern", "meadow", "fileindex@(clover):///orchard//lantern/meadow"},
		{"fileindex@(clover):///orchard//lantern/", "meadow/", "fileindex@(clover):///orchard//lantern/meadow"},
	}
	for _, tc := range tests {
		got, err := MustParse(tc.raw).Join(tc.name)
		if err != nil || got.String() != tc.want {
			t.Errorf("Join(%q, %q) = %q, %v; want %q", tc.raw, tc.name, got, err, tc.want)
		}
	}
	if _, err := MustParse("fileindex@(clover):///orchard").Join(".."); err == nil {
		t.Error("Join of .. must fail")
	}
	// Parent undoes Join in the host path, so a rename target stays where it belongs.
	row := MustParse("fileindex@(clover):///orchard//lantern")
	child, _ := row.Join("meadow")
	if !child.Parent().Equal(row) {
		t.Errorf("Parent(Join) = %q, want %q", child.Parent(), row)
	}
}

func TestTagViewHasPrefix(t *testing.T) {
	tests := []struct {
		p, prefix string
		want      bool
	}{
		{"fileindex(clover):///orchard/lantern", "fileindex(clover):///orchard", true},
		{"fileindex(clover):///orchard", "fileindex(clover):///orchard", true},
		{"fileindex(clover):///orchard/lantern", "fileindex(clover):///orch", false},
		{"fileindex(clover):///orchard/lantern", "fileindex:///orchard", false},
		{"fileindex:///orchard/lantern", "fileindex(clover):///orchard", false},
		{"fileindex(clover):///orchard", "fileindex(thistle):///orchard", false},
		{"fileindex(clover):///orchard", "fileindex@(clover):///orchard", false},
		{"fileindex@(clover):///orchard//lantern", "fileindex@(clover):///", true},
		{"fileindex@(clover):///orchard", "fileindex@(clover):///", true},
		{"fileindex@(clover):///", "fileindex@(clover):///", true},
		{"fileindex@(clover):///orchard", "fileindex@(thistle):///", false},
		{"fileindex@(clover):///orchard", "fileindex(clover):///", false},
		{"fileindex@(clover):///orchard//lantern", "fileindex@(clover):///orchard", true},
		{"fileindex@(clover):///orchard//lantern/meadow", "fileindex@(clover):///orchard//lantern", true},
		{"fileindex@(clover):///orchard/lantern", "fileindex@(clover):///orchard//lantern", true},
		{"fileindex@(clover):///orchard//lantern", "fileindex@(clover):///orchard/lan", false},
		{"fileindex@(clover):///", "fileindex@(clover):///orchard", false},
		{"/orchard/lantern", "fileindex(clover):///orchard", false},
	}
	for _, tc := range tests {
		if got := MustParse(tc.p).HasPrefix(MustParse(tc.prefix)); got != tc.want {
			t.Errorf("HasPrefix(%q, %q) = %v, want %v", tc.p, tc.prefix, got, tc.want)
		}
	}
	if TreesOverlap(MustParse("fileindex(clover):///orchard"), MustParse("fileindex:///orchard")) {
		t.Error("a tag view shares no tree with the plain index")
	}
}

func TestSplitDirBaseTagViews(t *testing.T) {
	tests := []struct{ raw, dir, name string }{
		{"fileindex:///orchard/lantern", "fileindex:///orchard", "lantern"},
		{"fileindex(clover):///orchard/lantern", "fileindex(clover):///orchard", "lantern"},
		{"fileindex@(clover):///orchard//lantern", "fileindex@(clover):///orchard", "lantern"},
		{"fileindex@(clover):///orchard//lantern/meadow", "fileindex@(clover):///orchard//lantern", "meadow"},
		{"fileindex@(clover):///orchard", "fileindex@(clover):///", "orchard"},
		{"fileindex@(clover):///", "fileindex@(clover):///", "/"},
	}
	for _, tc := range tests {
		if dir, name := SplitDirBase(tc.raw); dir != tc.dir || name != tc.name {
			t.Errorf("SplitDirBase(%q) = %q, %q; want %q, %q", tc.raw, dir, name, tc.dir, tc.name)
		}
	}
}

func TestCommonParentInTagView(t *testing.T) {
	a := MustParse("fileindex(clover):///orchard/lantern")
	b := MustParse("fileindex(clover):///orchard/meadow")
	if anc, ok := CommonAncestor(a, b); !ok || anc.String() != "fileindex(clover):///orchard" {
		t.Errorf("CommonAncestor = %q, %v", anc, ok)
	}
	if _, ok := CommonAncestor(a, MustParse("fileindex:///orchard/meadow")); ok {
		t.Error("a tag view and the plain index share no ancestor")
	}
}

func TestRawCleanAndDiskPath(t *testing.T) {
	for _, tc := range []struct{ raw, clean, disk string }{
		{"/orchard//moss/", "/orchard/moss", "/orchard/moss"},
		{"moss/../fern", "fern", "fern"},
		{"", ".", "."},
		{"fileindex:///orchard/moss/", "fileindex:///orchard/moss", "/orchard/moss"},
		{"fileindex(b&a):///orchard", "fileindex(a&b):///orchard", "/orchard"},
		{"fileindex@(a):///orchard//moss", "fileindex@(a):///orchard//moss", "/orchard/moss"},
		{"sftp://host/orchard", MustParse("sftp://host/orchard").String(), MustParse("sftp://host/orchard").String()},
		{"fileindex(a&b|c):///orchard", "fileindex(a&b|c):///orchard", "/orchard"},
	} {
		if got := Clean(tc.raw); got != tc.clean {
			t.Errorf("Clean(%q) = %q, want %q", tc.raw, got, tc.clean)
		}
		if got := DiskPath(tc.raw); got != tc.disk {
			t.Errorf("DiskPath(%q) = %q, want %q", tc.raw, got, tc.disk)
		}
	}
}
