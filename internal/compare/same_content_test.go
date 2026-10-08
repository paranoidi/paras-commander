package compare

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestSameContent(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) pathloc.Path {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return pathloc.FileMust(p)
	}
	orchard := write("orchard.txt", "abcdefghij")
	meadow := write("meadow.txt", "abcdefghij")
	lastDiff := write("harbor.txt", "abcdefghiX")
	shorter := write("lantern.txt", "abcdefghi")
	cases := []struct {
		name  string
		other pathloc.Path
		chunk int64
		want  bool
	}{
		{"identical chunked", meadow, 3, true},
		{"identical whole", meadow, 0, true},
		{"differs in last chunk", lastDiff, 3, false},
		{"differs whole", lastDiff, 0, false},
		{"size differs", shorter, 3, false},
	}
	for _, c := range cases {
		got, err := SameContent(context.Background(), orchard, c.other, c.chunk, nil)
		if err != nil || got != c.want {
			t.Errorf("%s: got %v, %v; want %v", c.name, got, err, c.want)
		}
	}
}
