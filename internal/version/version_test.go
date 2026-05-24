package version

import (
	"strings"
	"testing"
)

func TestLineFormat(t *testing.T) {
	line := Line()
	if !strings.HasPrefix(line, "pc ") {
		t.Fatalf("Line() = %q, want pc prefix", line)
	}
	ver := strings.TrimPrefix(line, "pc ")
	if ver == "" {
		t.Fatal("version suffix is empty")
	}
}

func TestStringNonEmpty(t *testing.T) {
	if String() == "" {
		t.Fatal("String() is empty")
	}
}
