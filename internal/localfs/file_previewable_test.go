package localfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckFilePreviewableText(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.txt")
	if err := os.WriteFile(p, []byte("hello 世界\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckFilePreviewable(p); err != nil {
		t.Fatal(err)
	}
}

func TestCheckFilePreviewableLegacyEncoding(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "legacy.txt")
	// "Käyttö" in Windows-1252/Latin-1: 0xE4 = ä, 0xF6 = ö. Not valid UTF-8, no NUL byte.
	if err := os.WriteFile(p, []byte("K\xe4ytt\xf6"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckFilePreviewable(p); err != nil {
		t.Fatalf("err = %v, want nil (legacy-encoded text should be previewable)", err)
	}
}

func TestCheckFilePreviewableBinaryNUL(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "b.bin")
	if err := os.WriteFile(p, []byte("a\x00b"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckFilePreviewable(p)
	if !errors.Is(err, ErrFilePreviewBinary) {
		t.Fatalf("err = %v want ErrFilePreviewBinary", err)
	}
}

func TestCheckFilePreviewableDirectory(t *testing.T) {
	dir := t.TempDir()
	err := CheckFilePreviewable(dir)
	if !errors.Is(err, ErrFilePreviewIsDir) {
		t.Fatalf("err = %v want ErrFilePreviewIsDir", err)
	}
}

func TestCheckFilePreviewableImage(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"meadow.png", "river.JPG", "stone.webp"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte{0x00, 0x01}, 0o644); err != nil {
			t.Fatal(err)
		}
		err := CheckFilePreviewable(p)
		if !errors.Is(err, ErrFilePreviewImage) {
			t.Fatalf("%s: err = %v want ErrFilePreviewImage", name, err)
		}
		if !IsImagePath(p) {
			t.Fatalf("IsImagePath(%q) = false", p)
		}
	}
}

func TestIsImagePathExtensions(t *testing.T) {
	cases := map[string]bool{
		"a.jpg": true, "b.jpeg": true, "c.png": true, "d.gif": true,
		"e.webp": true, "f.bmp": true, "g.tif": true, "h.tiff": true,
		"l.psd": true,
		"i.txt": false, "j.bin": false, "k.go": false,
	}
	for name, want := range cases {
		if got := IsImagePath(name); got != want {
			t.Fatalf("IsImagePath(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsImageMagickPath(t *testing.T) {
	cases := map[string]bool{
		"a.psd": true, "b.PSD": true,
		"c.png": false, "d.txt": false,
	}
	for name, want := range cases {
		if got := IsImageMagickPath(name); got != want {
			t.Fatalf("IsImageMagickPath(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestCheckFilePreviewableMedia(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"clip.mkv", "song.MP3", "movie.mp4"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte{0x00, 0x01}, 0o644); err != nil {
			t.Fatal(err)
		}
		err := CheckFilePreviewable(p)
		if !errors.Is(err, ErrFilePreviewMedia) {
			t.Fatalf("%s: err = %v want ErrFilePreviewMedia", name, err)
		}
		if !IsMediaPath(p) {
			t.Fatalf("IsMediaPath(%q) = false", p)
		}
	}
}

func TestIsMediaPathExtensions(t *testing.T) {
	cases := map[string]bool{
		"a.mkv": true, "b.mp4": true, "c.webm": true, "d.mp3": true, "e.flac": true,
		"f.txt": false, "g.png": false, "h.go": false,
	}
	for name, want := range cases {
		if got := IsMediaPath(name); got != want {
			t.Fatalf("IsMediaPath(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsVideoPathExtensions(t *testing.T) {
	cases := map[string]bool{
		"a.mkv": true, "b.mp4": true, "c.webm": true, "d.mp3": false, "e.flac": false,
		"f.txt": false, "g.png": false,
	}
	for name, want := range cases {
		if got := IsVideoPath(name); got != want {
			t.Fatalf("IsVideoPath(%q) = %v, want %v", name, got, want)
		}
	}
}
