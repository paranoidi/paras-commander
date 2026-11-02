package panel

import (
	"strings"
	"testing"
)

func TestNewGroupMatcherRegexInvalid(t *testing.T) {
	_, err := NewGroupMatcher("[", GroupPatternRegex, false)
	if err == nil {
		t.Fatal("expected error for invalid regexp")
	}
	if !strings.Contains(err.Error(), "invalid regexp") {
		t.Fatalf("error = %v, want invalid regexp prefix", err)
	}
}

func TestNewGroupMatcherShellInvalid(t *testing.T) {
	_, err := NewGroupMatcher("[", GroupPatternShell, false)
	if err == nil {
		t.Fatal("expected error for invalid shell pattern")
	}
	if !strings.Contains(err.Error(), "invalid shell pattern") {
		t.Fatalf("error = %v, want invalid shell pattern prefix", err)
	}
}

func TestGroupMatcherRegex(t *testing.T) {
	m, err := NewGroupMatcher(`^main.*\.go$`, GroupPatternRegex, false)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("main.go") {
		t.Fatal("main.go should match")
	}
	if !m.Match("main_test.go") {
		t.Fatal("main_test.go should match")
	}
	if m.Match("utils.go") {
		t.Fatal("utils.go should not match")
	}
}

func TestGroupMatcherShell(t *testing.T) {
	m, err := NewGroupMatcher("*.go", GroupPatternShell, false)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("main.go") {
		t.Fatal("main.go should match")
	}
	if m.Match("README.md") {
		t.Fatal("README.md should not match")
	}
}

func TestGroupMatcherSimple(t *testing.T) {
	m, err := NewGroupMatcher("main", GroupPatternSimple, false)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("main.go") {
		t.Fatal("main.go should match")
	}
	if !m.Match("Main_test.go") {
		t.Fatal("Main_test.go should match case-insensitively")
	}
}

func TestGroupMatcherSimpleCaseSensitive(t *testing.T) {
	m, err := NewGroupMatcher("main", GroupPatternSimple, true)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("main.go") {
		t.Fatal("main.go should match")
	}
	if m.Match("Main_test.go") {
		t.Fatal("Main_test.go should not match case-sensitively")
	}
}

func TestGroupMatcherSimpleAutoShell(t *testing.T) {
	cases := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"*.go", "Main.go", true},       // a wildcard switches to a whole-name shell match
		{"*.go", "main.go.bak", false},  // ...so the pattern is no longer a substring
		{"ma?n*", "main_test.go", true}, // '?' triggers too
		{"ma?n", "main.go", false},
		{"[1080p]", "show [1080p].mkv", true}, // '[' alone stays a substring
		{"[1080p]", "show 1.mkv", false},
	}
	for _, tc := range cases {
		m, err := NewGroupMatcher(tc.pattern, GroupPatternSimple, false)
		if err != nil {
			t.Fatalf("%q: %v", tc.pattern, err)
		}
		if got := m.Match(tc.name); got != tc.want {
			t.Errorf("%q vs %q: got %v want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

func TestGroupMatcherSimpleAutoShellInvalid(t *testing.T) {
	_, err := NewGroupMatcher("*[a", GroupPatternSimple, false)
	if err == nil || !strings.Contains(err.Error(), "invalid shell pattern") {
		t.Fatalf("err = %v, want invalid shell pattern", err)
	}
}

func TestGroupMatcherSimpleAutoShellFullPath(t *testing.T) {
	m, err := NewGroupMatcherFullPath("*harbor*", GroupPatternSimple, false)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("/tmp/harbor/lantern.txt") {
		t.Fatal("auto shell '*' should cross directory separators in full-path mode")
	}
}

func TestGroupMatcherFullPathShellDoesNotCrossSeparatorByDefault(t *testing.T) {
	m, err := NewGroupMatcher("*src*", GroupPatternShell, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Match("project/src/main.go") {
		t.Fatal("basename matcher should not see the directory component")
	}
}

func TestGroupMatcherFullPathShellCrossesSeparator(t *testing.T) {
	m, err := NewGroupMatcherFullPath("*src*", GroupPatternShell, false)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("project/src/main.go") {
		t.Fatal("*src* should match a directory component in full-path mode")
	}
	if m.Match("project/lib/main.go") {
		t.Fatal("no src component should not match")
	}
}

func TestGroupMatcherFullPathRegexMatchesDirComponent(t *testing.T) {
	m, err := NewGroupMatcherFullPath(`/src/`, GroupPatternRegex, false)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("project/src/main.go") {
		t.Fatal("regex full-path should match a directory component")
	}
}
