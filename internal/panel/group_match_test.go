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
