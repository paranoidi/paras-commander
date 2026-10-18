package pathloc

import (
	"path/filepath"
	"testing"
)

func TestTrailingWhitespacePreserved(t *testing.T) {
	t.Parallel()
	base := filepath.Join(t.TempDir(), "meadow ")
	p, err := Parse(base)
	if err != nil || p.String() != base {
		t.Fatalf("Parse = %q, %v; want %q", p.String(), err, base)
	}
	f, err := File(base)
	if err != nil || f.String() != base {
		t.Fatalf("File = %q, %v", f.String(), err)
	}
	j, err := f.Parent().Join("meadow ")
	if err != nil || j.String() != base {
		t.Fatalf("Join = %q, %v", j.String(), err)
	}
	if _, err := Parse("   "); err == nil {
		t.Fatal("blank input must be rejected")
	}
}
