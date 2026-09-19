package archive

import (
	"reflect"
	"testing"
)

func TestListable(t *testing.T) {
	if !FormatZip.Listable() || !FormatTarGz.Listable() || !FormatSevenZ.Listable() {
		t.Fatal("expected zip/tar.gz/7z listable")
	}
	if FormatGz.Listable() || FormatBz2.Listable() || FormatXz.Listable() || FormatZ.Listable() {
		t.Fatal("single-stream formats must not be listable")
	}
	if !ListableName("garden/tulip.zip") || ListableName("garden/tulip.gz") {
		t.Fatal("ListableName mismatch")
	}
}

func TestListArgv(t *testing.T) {
	tests := []struct {
		f    Format
		tool string
		path string
		want []string
	}{
		{FormatTarGz, "/bin/tar", "/tmp/a.tar.gz", []string{"/bin/tar", "-tf", "/tmp/a.tar.gz"}},
		{FormatZip, "/bin/unzip", "/tmp/a.zip", []string{"/bin/unzip", "-Z1", "/tmp/a.zip"}},
		{FormatJar, "/bin/unzip", "/tmp/a.jar", []string{"/bin/unzip", "-Z1", "/tmp/a.jar"}},
		{FormatRar, "/bin/unrar", "/tmp/a.rar", []string{"/bin/unrar", "lb", "--", "/tmp/a.rar"}},
		{FormatSevenZ, "/bin/7z", "/tmp/a.7z", []string{"/bin/7z", "l", "-ba", "-slt", "/tmp/a.7z"}},
		{FormatGz, "/bin/gzip", "/tmp/a.gz", nil},
	}
	for _, tt := range tests {
		got := ListArgv(tt.f, tt.tool, tt.path)
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%v: got %v want %v", tt.f, got, tt.want)
		}
	}
}

func TestParseListing(t *testing.T) {
	tarOut := "garden/tulip.txt\ngarden/fern/\ngarden/fern/moss.txt\n"
	got := ParseListing(FormatTar, []byte(tarOut))
	want := []string{"garden/tulip.txt", "garden/fern/", "garden/fern/moss.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tar: got %v want %v", got, want)
	}

	seven := "Path = garden/tulip.txt\nSize = 12\n\nPath = garden/fern/\nFolder = +\n"
	got = ParseListing(FormatSevenZ, []byte(seven))
	want = []string{"garden/tulip.txt", "garden/fern/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("7z: got %v want %v", got, want)
	}
}
