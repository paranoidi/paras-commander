package fileindex

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/findcrit"
)

// describe prints groups as "a|b c" with ! for negated terms and the pattern shown raw.
func describe(g termGroups) string {
	var groups []string
	for _, alts := range g {
		var terms []string
		for _, t := range alts {
			s := t.pattern
			if t.not {
				s = "!" + s
			}
			terms = append(terms, s)
		}
		groups = append(groups, strings.Join(terms, "|"))
	}
	return strings.Join(groups, " ")
}

func TestParseTerms(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"beach 2019", "%beach% %2019%"},
		{"BEACH", "%beach%"},
		{"beach | lake", "%beach%|%lake%"},
		{"a | b c", "%a%|%b% %c%"},
		{"a | b | c d", "%a%|%b%|%c% %d%"},
		{"!2019", "!%2019%"},
		{"^img", "img%"},
		{".jpg$", "%.jpg"},
		{"^img.jpg$", "img.jpg"},
		{"*.jpg", "%.jpg"},
		{"report-??", "report-__"},
		{"^*.jpg", "%.jpg"},
		{`"summer beach"`, "%summer beach%"},
		{`summer\ beach`, "%summer beach%"},
		{`"*.jpg"`, "%*.jpg%"},
		{`"a | b"`, "%a | b%"},
		{`"!x"`, "%!x%"},
		{`"unclosed text`, "%unclosed text%"},
		{"'beach", "%beach%"},
		{"100%", `%100\%%`},
		{"snake_case", `%snake\_case%`},
		{`back\slash`, `%back\\slash%`},
		{"! ^ $ ' |", ""},
		{"| a |", "%a%"},
		{"a | | b", "%a%|%b%"},
		{`""`, ""},
		{"", ""},
		{"   ", ""},
		{"Ünïcode", "%ünïcode%"},
	} {
		if got := describe(parseTerms(c.in)); got != c.want {
			t.Errorf("parseTerms(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPatternTerms(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"holiday", "%holiday%"},
		{"two words", "%two words%"},
		{"*.JPG", "%.jpg"},
		{"a|b !c", "%a|b !c%"},
		{"50%_off", `%50\%\_off%`},
		{"", ""},
	} {
		if got := describe(patternTerms(c.in)); got != c.want {
			t.Errorf("patternTerms(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTrigramExpression(t *testing.T) {
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"beach", `("bea" AND "eac" AND "ach")`, true},
		{"ab", "", false},
		{"beach | ab", "", false},
		{"!beach", "", false},
		{"a*b", "", false},
		{"ab beach", `("bea" AND "eac" AND "ach")`, true},
		{"abc | xyz1", `("abc") OR ("xyz" AND "yz1")`, true},
		{"abc*def", `("abc" AND "def")`, true},
		{"ab !x beach", `("bea" AND "eac" AND "ach")`, true},
		{"ab | beach !x", "", false},
		{"abcdef ghijklm", `("ghi" AND "hij" AND "ijk" AND "jkl" AND "klm")`, true},
	} {
		got, ok := parseTerms(c.in).trigram()
		if got != c.want || ok != c.ok {
			t.Errorf("trigram(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
	// Runs count characters, not bytes: a two-letter Greek term is too short.
	if got, ok := patternTerms(`say"hi"`).trigram(); !ok || got != `("say" AND "ay""" AND "y""h" AND """hi" AND "hi""")` {
		t.Errorf("a quote in a token must be doubled: %q, %v", got, ok)
	}
	if _, ok := parseTerms("αβ").trigram(); ok {
		t.Error("two runes must not use the trigram index")
	}
	if got, ok := parseTerms("αβγ").trigram(); !ok || got != `("αβγ")` {
		t.Errorf("trigram(αβγ) = %q, %v", got, ok)
	}
}

// findFixture scans a tree with a range of names and returns the index and the tree root.
func findFixture(t *testing.T) (*Index, string) {
	t.Helper()
	x := testIndex(t, initIndex(t))
	root := tree(t)
	for name, body := range map[string]string{
		"orchard/Sunset-Hill.jpg":          "a",
		"orchard/sunset-lake.png":          "b",
		"orchard/sunset-hill-2019.txt":     "c",
		"orchard/ÉCLAIR.txt":               "d",
		"orchard/50%_done.txt":             "e",
		"orchard/ab.txt":                   "f",
		"orchard/quartz/deep/pebble.txt":   "g",
		".hollow/ghost.txt":                "h",
		"orchard/.cellar/sunset-secret.md": "i",
		"sunset-dir/note.md":               "j",
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	return x, root
}

func findRels(t *testing.T, x *Index, root string, q FindQuery, limit int) ([]string, bool) {
	t.Helper()
	got, trunc, err := x.Find(context.Background(), root, q, limit)
	if err != nil {
		t.Fatalf("Find(%+v): %v", q, err)
	}
	var out []string
	for _, f := range got {
		rel, err := filepath.Rel(root, f.Path)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel = f.Path
		}
		if f.IsDir {
			rel += "/"
		}
		out = append(out, rel)
	}
	return out, trunc
}

func TestFindModes(t *testing.T) {
	x, root := findFixture(t)
	check := func(q FindQuery, want ...string) {
		t.Helper()
		got, _ := findRels(t, x, root, q, 0)
		if !slices.Equal(got, want) {
			t.Errorf("Find(%+v) = %q, want %q", q, got, want)
		}
	}
	check(FindQuery{Text: "HILL"}, "orchard/sunset-hill-2019.txt", "orchard/Sunset-Hill.jpg")
	check(FindQuery{Text: "sunset hill"}, "orchard/sunset-hill-2019.txt", "orchard/Sunset-Hill.jpg")
	check(FindQuery{Text: "sunset hill !2019"}, "orchard/Sunset-Hill.jpg")
	check(FindQuery{Text: "lake | 2019"}, "orchard/sunset-hill-2019.txt", "orchard/sunset-lake.png")
	check(FindQuery{Text: "hill | lake !2019"}, "orchard/Sunset-Hill.jpg", "orchard/sunset-lake.png")
	check(FindQuery{Text: "^sunset-l"}, "orchard/sunset-lake.png")
	check(FindQuery{Text: ".png$"}, "orchard/sunset-lake.png")
	check(FindQuery{Text: "^ab.txt$"}, "orchard/ab.txt")
	check(FindQuery{Text: "ab"}, "orchard/ab.txt")                       // too short for the trigram index
	check(FindQuery{Text: "éclair"}, "orchard/ÉCLAIR.txt")               // non-ASCII case folds
	check(FindQuery{Text: "ÉCLAIR"}, "orchard/ÉCLAIR.txt")               //
	check(FindQuery{Text: "50%_done"}, "orchard/50%_done.txt")           // % and _ are literal
	check(FindQuery{Text: "50_done"})                                    //
	check(FindQuery{Text: "*-hill.*"}, "orchard/Sunset-Hill.jpg")        // a wildcard term is the whole name
	check(FindQuery{Text: "sunset-h?ll.jpg"}, "orchard/Sunset-Hill.jpg") //
	check(FindQuery{Text: `"sunset-hill 2019"`})
	check(FindQuery{Text: "sunset", OnlyDirs: true}, "sunset-dir/")
	check(FindQuery{Text: "sunset-d", OnlyFiles: true})
	check(FindQuery{Text: "!sunset !orchard !lantern !hollow !deep !quartz !harbor !meadow !thistle !note !ghost !ab !done !clair !pebble"})
	check(FindQuery{Text: "", Mode: findcrit.IndexTerms})
	// Pattern: spaces are literal, no operators.
	check(FindQuery{Text: "sunset-hill 2019", Mode: findcrit.IndexPattern})
	check(FindQuery{Text: "hill-2019", Mode: findcrit.IndexPattern}, "orchard/sunset-hill-2019.txt")
	check(FindQuery{Text: "*.png", Mode: findcrit.IndexPattern}, "orchard/sunset-lake.png")
	// Boolean: files only.
	check(FindQuery{Text: "sunset hill", Mode: findcrit.IndexBoolean}, "orchard/sunset-hill-2019.txt", "orchard/Sunset-Hill.jpg")
	check(FindQuery{Text: "sunset NOT hill", Mode: findcrit.IndexBoolean}, "orchard/sunset-lake.png")
	check(FindQuery{Text: "dir", Mode: findcrit.IndexBoolean})
}

func TestFindBooleanSyntaxError(t *testing.T) {
	x, root := findFixture(t)
	_, _, err := x.Find(context.Background(), root, FindQuery{Text: `"unbalanced`, Mode: findcrit.IndexBoolean}, 0)
	if err == nil {
		t.Fatal("an FTS syntax error must be returned")
	}
	if _, _, err = x.Find(context.Background(), root, FindQuery{Text: `"unbalanced`}, 0); err != nil {
		t.Fatalf("a Terms query has no syntax errors: %v", err)
	}
}

func TestFindCancelled(t *testing.T) {
	x, root := findFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := x.Find(ctx, root, FindQuery{Text: "sunset"}, 0); err != context.Canceled {
		t.Fatalf("cancelled Find = %v, want context.Canceled", err)
	}
}

func TestFindMergesDirsAndCaps(t *testing.T) {
	x, root := findFixture(t)
	got, trunc := findRels(t, x, root, FindQuery{Text: "sunset"}, 0)
	// Sorted by lowered name: the directory sits among the files, not after them.
	want := []string{"orchard/sunset-hill-2019.txt", "orchard/Sunset-Hill.jpg", "orchard/sunset-lake.png", "sunset-dir/"}
	slices.SortFunc(want, func(a, b string) int {
		return strings.Compare(strings.ToLower(filepath.Base(a)), strings.ToLower(filepath.Base(b)))
	})
	if !slices.Equal(got, want) || trunc {
		t.Fatalf("sunset = %q (trunc %v), want %q", got, trunc, want)
	}
	// A cap keeps the first names of both kinds: the directory is not starved by the files.
	got, trunc = findRels(t, x, root, FindQuery{Text: "sunset"}, 4)
	if len(got) != 4 || trunc {
		t.Fatalf("limit 4 = %q trunc=%v", got, trunc)
	}
	got, trunc = findRels(t, x, root, FindQuery{Text: "t"}, 3)
	if len(got) != 3 || !trunc {
		t.Fatalf("limit 3 = %q trunc=%v, want 3 and truncated", got, trunc)
	}
	all, _ := findRels(t, x, root, FindQuery{Text: "t"}, 0)
	if !slices.Equal(got, all[:3]) {
		t.Fatalf("capped %q is not the head of %q", got, all)
	}
}

func TestFindScopesDepthAndHidden(t *testing.T) {
	x, root := findFixture(t)
	count := func(r string, q FindQuery) int {
		t.Helper()
		got, _ := findRels(t, x, r, q, 0)
		return len(got)
	}
	// The whole index sees the tree from "/", whatever directory the panel is in.
	whole := FindQuery{Text: "*.txt"}
	if n := count("/", whole); n != 7 { // the hidden ghost.txt is left out
		t.Fatalf("whole index = %d, want 7", n)
	}
	all := count("/", FindQuery{Text: "*.txt", IncludeHidden: true})
	if all != 8 {
		t.Fatalf("whole index with hidden = %d, want 8", all)
	}
	if n := count(filepath.Join(root, "orchard"), FindQuery{Text: "*.txt"}); n != 5 {
		t.Fatalf("current directory = %d, want 5", n)
	}
	if n := count(filepath.Join(root, "orchard"), FindQuery{Text: "*.txt", MaxDepth: 1}); n != 4 {
		t.Fatalf("depth 1 = %d, want 4", n)
	}
	// A hidden directory root: its own name is not judged, what is below it is.
	cellar := filepath.Join(root, "orchard", ".cellar")
	if n := count(cellar, FindQuery{Text: "sunset"}); n != 1 {
		t.Fatalf("inside a hidden root = %d, want 1", n)
	}
	// An unknown directory searches nothing.
	if n := count(filepath.Join(root, "absent"), FindQuery{Text: "sunset"}); n != 0 {
		t.Fatalf("unknown root = %d, want 0", n)
	}
	// Roots: nested roots do not repeat a result, and the root's own name is never a result.
	orchard := filepath.Join(root, "orchard")
	nested := FindQuery{Text: "quartz", Roots: []string{orchard, filepath.Join(orchard, "quartz")}}
	if got, _ := findRels(t, x, root, nested, 0); !slices.Equal(got, []string{"orchard/quartz/"}) {
		t.Fatalf("nested roots = %q", got)
	}
	nested.Text = "deep"
	if got, _ := findRels(t, x, root, nested, 0); !slices.Equal(got, []string{"orchard/quartz/deep/"}) {
		t.Fatalf("nested roots, deep = %q", got)
	}
	rootsOnly := FindQuery{Text: "orchard", Roots: []string{orchard}}
	if got, _ := findRels(t, x, root, rootsOnly, 0); len(got) != 0 {
		t.Fatalf("a scope root matched itself: %q", got)
	}
	unknown := FindQuery{Text: "sunset", Roots: []string{filepath.Join(root, "absent")}}
	if got, _ := findRels(t, x, root, unknown, 0); len(got) != 0 {
		t.Fatalf("unknown scope root = %q", got)
	}
	// An unknown root next to a known one still searches the known one.
	mixed := FindQuery{Text: "lake", Roots: []string{filepath.Join(root, "absent"), orchard}}
	if got, _ := findRels(t, x, root, mixed, 0); !slices.Equal(got, []string{"orchard/sunset-lake.png"}) {
		t.Fatalf("mixed roots = %q", got)
	}
}

// TestFindScopeRootIsWhole: "/" has a directory row of its own, but must not be treated as a scope
// (that would build the tree of the whole index for every query).
func TestFindScopeRootIsWhole(t *testing.T) {
	x, root := findFixture(t)
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := d.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if whole, ids, err := findScope(tx, "/", nil); !whole || len(ids) != 0 || err != nil {
		t.Fatalf(`scope "/" = whole %v ids %v err %v, want whole`, whole, ids, err)
	}
	if whole, ids, err := findScope(tx, root, nil); whole || len(ids) != 1 || err != nil {
		t.Fatalf("scope %q = whole %v ids %v err %v, want one id", root, whole, ids, err)
	}
}
