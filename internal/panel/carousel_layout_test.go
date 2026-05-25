package panel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCarouselCenterHasSubdirectories(t *testing.T) {
	mixed := t.TempDir()
	sub := filepath.Join(mixed, "maple")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mixed, "cedar.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(mixed)
	if err != nil {
		t.Fatal(err)
	}
	if !state.CarouselCenterHasSubdirectories() {
		t.Fatal("listing with subdirectory should report true")
	}

	flat := t.TempDir()
	for _, name := range []string{"one.txt", "two.txt"} {
		if err := os.WriteFile(filepath.Join(flat, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	filesOnly, err := New(flat)
	if err != nil {
		t.Fatal(err)
	}
	if filesOnly.CarouselCenterHasSubdirectories() {
		t.Fatal("file-only listing should report false")
	}
}
