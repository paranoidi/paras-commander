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
		buf   []byte
		want  bool
	}{
		{"identical small blocks", meadow, make([]byte, 6), true},
		{"identical default buffer", meadow, nil, true},
		{"differs in last block", lastDiff, make([]byte, 6), false},
		{"differs default buffer", lastDiff, nil, false},
		{"shorter file differs", shorter, make([]byte, 6), false},
	}
	for _, c := range cases {
		got, err := SameContent(context.Background(), orchard, c.other, c.buf, nil)
		if err != nil || got != c.want {
			t.Errorf("%s: got %v, %v; want %v", c.name, got, err, c.want)
		}
	}
}

func TestSameContentStopsAtFirstDifferingBlock(t *testing.T) {
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
		{"first block differs", "Xaaabbbbcccc", false, 8},
		{"second block differs", "aaaaXbbbcccc", false, 16},
		{"identical reads both whole", "aaaabbbbcccc", true, 24},
		{"shorter file stops when it ends", "aaaa", false, 12},
	}
	for _, c := range cases {
		other := write("quarry.bin", c.data)
		var read int64
		same, err := SameContent(context.Background(), beacon, other, make([]byte, 8), func(n int64) { read = n })
		if err != nil || same != c.wantSame || read != c.wantRead {
			t.Errorf("%s: same=%v read=%d err=%v; want same=%v read=%d", c.name, same, read, err, c.wantSame, c.wantRead)
		}
	}
}
