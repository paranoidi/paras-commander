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
		got, err := SameContent(context.Background(), orchard, c.other, c.chunk, nil, nil)
		if err != nil || got != c.want {
			t.Errorf("%s: got %v, %v; want %v", c.name, got, err, c.want)
		}
	}
}

func TestSameContentStopsAtFirstDifferingChunk(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) pathloc.Path {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return pathloc.FileMust(p)
	}
	beacon := write("beacon.bin", "aaaabbbbcccc")
	cases := []struct {
		name     string
		data     string
		wantSame bool
		wantRead int64
	}{
		{"first chunk differs", "Xaaabbbbcccc", false, 8},
		{"second chunk differs", "aaaaXbbbcccc", false, 16},
		{"identical reads both whole", "aaaabbbbcccc", true, 24},
		{"size differs reads nothing", "aaaa", false, 0},
	}
	for _, c := range cases {
		other := write("quarry.bin", c.data)
		var read int64
		same, err := SameContent(context.Background(), beacon, other, 4, make([]byte, 2), func(n int64) { read = n })
		if err != nil || same != c.wantSame || read != c.wantRead {
			t.Errorf("%s: same=%v read=%d err=%v; want same=%v read=%d", c.name, same, read, err, c.wantSame, c.wantRead)
		}
	}
}
