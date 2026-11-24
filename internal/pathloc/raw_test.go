package pathloc

import (
	"os"
	"path/filepath"
	"testing"
)

// hostInputs are host path strings, written well and badly, for the checks that a raw function
// is path/filepath on a host path.
var hostInputs = []string{
	"", ".", "..", "/", "//", "/meadow", "/meadow/", "/meadow//orchard", "/meadow/./orchard/../lantern",
	"/meadow/orchard/..", "/../meadow", "meadow", "meadow/orchard", "meadow/../..", "../meadow",
	"./meadow/", "/meadow/fileindex", "/meadow/sftp:", "fileindex(clover)", "fileindex@x", "sftp:/meadow",
}

// uriForms is one location of each form: SFTP, plain index, structure and flat tag view (the list,
// a row, the rest inside a row).
var uriForms = []string{
	"sftp://willow@harbor:2222/meadow/orchard",
	"sftp://harbor/~/lantern",
	"sftp://harbor/",
	"fileindex:///meadow/orchard",
	"fileindex:///",
	"fileindex(clover&thistle):///meadow/orchard",
	"fileindex(clover&thistle):///",
	"fileindex@(clover|thistle):///",
	"fileindex@(clover|thistle):///meadow",
	"fileindex@(clover|thistle):///meadow//orchard/lantern",
}

func TestRawHostEqualsFilepath(t *testing.T) {
	t.Parallel()
	for _, in := range hostInputs {
		if got, want := Clean(in), filepath.Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
		if got, want := Dir(in), filepath.Dir(in); got != want {
			t.Errorf("Dir(%q) = %q, want %q", in, got, want)
		}
		for _, elem := range [][]string{nil, {""}, {"orchard"}, {"..", "orchard"}, {"a/b", "./c/"}, {"/abs"}} {
			if got, want := Join(in, elem...), filepath.Join(append([]string{in}, elem...)...); got != want {
				t.Errorf("Join(%q, %q) = %q, want %q", in, elem, got, want)
			}
		}
		for _, other := range hostInputs {
			got, gerr := Rel(in, other)
			want, werr := filepath.Rel(in, other)
			if got != want || (gerr == nil) != (werr == nil) {
				t.Errorf("Rel(%q, %q) = %q, %v; want %q, %v", in, other, got, gerr, want, werr)
			}
		}
	}
}

func TestRawClean(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"/meadow//orchard/":                                  "/meadow/orchard",
		"sftp://willow@harbor:2222//meadow/./orchard/":       "sftp://willow@harbor:2222/meadow/orchard",
		"sftp://harbor":                                      "sftp://harbor/",
		"fileindex:///meadow//orchard/":                      "fileindex:///meadow/orchard",
		"fileindex:///meadow/../clover":                      "fileindex:///clover",
		"fileindex:///":                                      "fileindex:///",
		"fileindex:///meadow/orchard":                        "fileindex:///meadow/orchard",
		"fileindex://meadow":                                 "fileindex://meadow", // not absolute: as given
		"fileindex(thistle&clover):///meadow/":               "fileindex(clover&thistle):///meadow",
		"fileindex(clover):///":                              "fileindex(clover):///",
		"fileindex(clover)://":                               "fileindex(clover):///",
		"fileindex@(clover):///meadow//orchard/./lantern/":   "fileindex@(clover):///meadow//orchard/lantern",
		"fileindex@(clover):///meadow//":                     "fileindex@(clover):///meadow",
		"fileindex@(clover):///meadow/orchard":               "fileindex@(clover):///meadow/orchard",
		"fileindex(clover&thistle|willow):///meadow/orchard": "fileindex(clover&thistle|willow):///meadow/orchard", // unparseable: as given
		"fileindex@:///meadow":                               "fileindex@:///meadow",
	} {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRawCleanDoesNotAllocateWhenClean(t *testing.T) {
	for _, in := range []string{"/meadow/orchard/lantern.txt", "fileindex:///meadow/orchard/lantern.txt"} {
		if n := testing.AllocsPerRun(100, func() { _ = Clean(in) }); n != 0 {
			t.Errorf("Clean(%q) allocates %v times", in, n)
		}
		if n := testing.AllocsPerRun(100, func() { _ = Canonical(in) }); n != 0 {
			t.Errorf("Canonical(%q) allocates %v times", in, n)
		}
	}
}

func TestRawCanonical(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{
		"":                                       "",
		"   ":                                    "",
		"/meadow//orchard/":                      "/meadow/orchard",
		"/":                                      "/",
		"orchard/../lantern":                     filepath.Join(cwd, "lantern"),
		"sftp://":                                "",
		"sftp://willow@harbor:2222/meadow/..":    "sftp://willow@harbor:2222/",
		"fileindex:///meadow//orchard/":          "fileindex:///meadow/orchard",
		"fileindex://meadow":                     "",
		"fileindex(thistle&clover):///meadow/":   "fileindex(clover&thistle):///meadow",
		"fileindex(a&b|c):///meadow":             "",
		"fileindex@(clover):///meadow//orchard/": "fileindex@(clover):///meadow//orchard",
		"fileindex@(clover):///meadow//":         "fileindex@(clover):///meadow",
		"fileindex(clover)":                      filepath.Join(cwd, "fileindex(clover)"), // a local file name
	} {
		if got := Canonical(in); got != want {
			t.Errorf("Canonical(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range append(append([]string{}, hostInputs...), uriForms...) {
		if want := parsedString(in); Canonical(in) != want {
			t.Errorf("Canonical(%q) = %q, Parse gives %q", in, Canonical(in), want)
		}
	}
}

// parsedString is the canonical string Parse gives, "" when it fails.
func parsedString(raw string) string {
	p, err := Parse(raw)
	if err != nil {
		return ""
	}
	return p.String()
}

func TestRawHostOf(t *testing.T) {
	t.Parallel()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{
		"/meadow//orchard/":                      "/meadow/orchard",
		"orchard":                                filepath.Join(cwd, "orchard"),
		"fileindex:///meadow//orchard/":          "/meadow/orchard",
		"fileindex:///":                          "/",
		"fileindex(clover):///meadow/orchard":    "/meadow/orchard",
		"fileindex(clover):///":                  "/",
		"fileindex@(clover):///meadow//orchard/": "/meadow/orchard",
		"fileindex@(clover):///":                 "/",
		"sftp://harbor/meadow":                   "",
		"fileindex://meadow":                     "",
		"":                                       "",
	} {
		got, ok := HostOf(in)
		if got != want || ok != (want != "") {
			t.Errorf("HostOf(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestRawDir(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"sftp://willow@harbor:2222/meadow/orchard": "sftp://willow@harbor:2222/meadow",
		"sftp://harbor/meadow":                     "sftp://harbor/",
		"sftp://harbor/":                           "sftp://harbor/",
		"sftp://harbor/~/lantern":                  "sftp://harbor/~",
		"fileindex:///meadow/orchard/":             "fileindex:///meadow",
		"fileindex:///meadow":                      "fileindex:///",
		"fileindex:///":                            "fileindex:///",
		"fileindex(clover&thistle):///meadow/a":    "fileindex(clover&thistle):///meadow",
		"fileindex(clover):///":                    "fileindex(clover):///",
		"fileindex@(clover):///":                   "fileindex@(clover):///",
		"fileindex@(clover):///meadow":             "fileindex@(clover):///",
		"fileindex@(clover):///meadow//orchard":    "fileindex@(clover):///meadow",
		"fileindex@(clover):///meadow//a/b":        "fileindex@(clover):///meadow//a",
		"fileindex://meadow":                       "fileindex://meadow", // unparseable: as given
	} {
		if got := Dir(in); got != want {
			t.Errorf("Dir(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRawBase(t *testing.T) {
	t.Parallel()
	for _, in := range hostInputs {
		if got, want := Base(in), filepath.Base(in); got != want {
			t.Errorf("Base(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"sftp://willow@harbor:2222/meadow/orchard/": "orchard",
		"sftp://harbor/":                        "/",
		"fileindex:///meadow/orchard":           "orchard",
		"fileindex:///":                         "/",
		"fileindex(clover&thistle):///":         "/",
		"fileindex@(clover):///meadow//orchard": "orchard",
		"fileindex://meadow":                    "fileindex://meadow", // unparseable: as given
	} {
		if got := Base(in); got != want {
			t.Errorf("Base(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRawJoin(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		elem []string
		want string
	}{
		{"sftp://harbor/meadow", []string{"orchard"}, "sftp://harbor/meadow/orchard"},
		{"sftp://harbor/meadow", []string{"orchard/lantern", "..", "./clover"}, "sftp://harbor/meadow/orchard/clover"},
		{"sftp://harbor/~", []string{"lantern"}, "sftp://harbor/~/lantern"},
		{"sftp://harbor/", []string{"..", "meadow"}, "sftp://harbor/meadow"},
		{"fileindex:///", []string{"meadow", "orchard"}, "fileindex:///meadow/orchard"},
		{"fileindex:///meadow/", []string{"../clover/"}, "fileindex:///clover"},
		{"fileindex:///meadow", nil, "fileindex:///meadow"},
		{"fileindex:///meadow", []string{"", "."}, "fileindex:///meadow"},
		{"fileindex(clover):///meadow", []string{"orchard"}, "fileindex(clover):///meadow/orchard"},
		{"fileindex(clover):///", []string{"meadow"}, "fileindex(clover):///meadow"},
		{"fileindex@(clover):///", []string{"meadow"}, "fileindex@(clover):///meadow"},
		{"fileindex@(clover):///meadow", []string{"orchard"}, "fileindex@(clover):///meadow//orchard"},
		{"fileindex@(clover):///meadow//orchard", []string{"lantern/moss"}, "fileindex@(clover):///meadow//orchard/lantern/moss"},
		{"fileindex@(clover):///meadow//orchard/lantern", []string{".."}, "fileindex@(clover):///meadow//orchard"},
		{"fileindex://meadow", []string{"orchard"}, "fileindex://meadow"}, // unparseable: as given
	} {
		if got := Join(tc.raw, tc.elem...); got != tc.want {
			t.Errorf("Join(%q, %q) = %q, want %q", tc.raw, tc.elem, got, tc.want)
		}
	}
	// Join then Dir is the way back, in every form.
	for _, in := range uriForms {
		if got := Dir(Join(in, "lantern")); got != Canonical(in) && Canonical(in) != "" {
			t.Errorf("Dir(Join(%q, lantern)) = %q, want %q", in, got, Canonical(in))
		}
	}
}

func TestRawRel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		base, target, want string
		ok                 bool
	}{
		{"sftp://willow@harbor:2222/meadow", "sftp://willow@harbor:2222/meadow/orchard/lantern", "orchard/lantern", true},
		{"sftp://harbor/meadow/orchard", "sftp://harbor/meadow/clover", "../clover", true},
		{"sftp://harbor/meadow", "sftp://other/meadow/clover", "", false},
		{"sftp://harbor/meadow", "/meadow/clover", "", false},
		{"fileindex:///meadow", "fileindex:///meadow/orchard", "orchard", true},
		{"fileindex:///meadow/orchard", "fileindex:///meadow", "..", true},
		{"fileindex:///meadow", "/meadow/orchard", "", false},
		{"/meadow", "fileindex:///meadow/orchard", "", false},
		{"fileindex(clover):///meadow", "fileindex(clover):///meadow/orchard", "orchard", true},
		{"fileindex(clover):///meadow", "fileindex(thistle):///meadow/orchard", "", false},
		{"fileindex(clover):///meadow", "fileindex:///meadow/orchard", "", false},
		{"fileindex@(clover):///meadow", "fileindex@(clover):///meadow//orchard/lantern", "orchard/lantern", true},
		{"fileindex@(clover):///meadow//orchard", "fileindex@(clover):///meadow//orchard/lantern", "lantern", true},
		{"fileindex@(clover):///", "fileindex@(clover):///meadow", "meadow", true},
		{"sftp://harbor/meadow", "fileindex:///meadow", "", false},
	} {
		got, err := Rel(tc.base, tc.target)
		if got != tc.want || (err == nil) != tc.ok {
			t.Errorf("Rel(%q, %q) = %q, %v; want %q, ok %v", tc.base, tc.target, got, err, tc.want, tc.ok)
		}
	}
}

// BenchmarkRawClean and BenchmarkRawCanonical are the per-row cost of the two hot functions (the
// size column and the selection keys call them for every row on every repaint).
func BenchmarkRawClean(b *testing.B)     { benchRaw(b, Clean) }
func BenchmarkRawCanonical(b *testing.B) { benchRaw(b, Canonical) }

var benchSink string

func benchRaw(b *testing.B, fn func(string) string) {
	for name, in := range map[string]string{
		"host":  "/home/meadow/orchard/lantern/harbor.txt",
		"index": "fileindex:///home/meadow/orchard/lantern/harbor.txt",
		"flat":  "fileindex@(clover):///meadow/orchard//lantern/harbor.txt",
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchSink = fn(in)
			}
		})
	}
}
