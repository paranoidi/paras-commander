package archive

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildArgv(t *testing.T) {
	tc := Toolchain{
		Tar:        "/bin/tar",
		Unzip:      "/bin/unzip",
		SevenZ:     "/bin/7z",
		Unrar:      "/bin/unrar",
		Gzip:       "/bin/gzip",
		Bzip2:      "/bin/bzip2",
		Xz:         "/bin/xz",
		Uncompress: "/bin/uncompress",
	}
	archive := "/src/foo.tar.gz"
	dest := "/dst/out"

	argv, err := BuildArgv(FormatTarGz, archive, dest, tc)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/bin/tar", "-C", dest, "-xvzf", archive}
	if len(argv) != len(want) {
		t.Fatalf("argv = %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}

	argv, err = BuildArgv(FormatZip, archive, dest, tc)
	if err != nil {
		t.Fatal(err)
	}
	if argv[0] != "/bin/unzip" || argv[1] != "-o" {
		t.Fatalf("zip argv = %v", argv)
	}

	argv, err = BuildArgv(FormatSevenZ, archive, dest, tc)
	if err != nil {
		t.Fatal(err)
	}
	foundOut := false
	for _, a := range argv {
		if strings.HasPrefix(a, "-o") {
			foundOut = true
			break
		}
	}
	if !foundOut {
		t.Fatalf("7z argv = %v, want -o dest flag", argv)
	}

	argv, err = BuildArgv(FormatRar, archive, dest, tc)
	if err != nil {
		t.Fatal(err)
	}
	last := argv[len(argv)-1]
	if !strings.HasSuffix(last, string(filepath.Separator)) {
		t.Fatalf("unrar dest = %q, want trailing separator", last)
	}

	argv, err = BuildArgv(FormatGz, archive, dest, tc)
	if err != nil {
		t.Fatal(err)
	}
	if argv[0] != "/bin/gzip" || argv[1] != "-dc" {
		t.Fatalf("gzip argv = %v", argv)
	}
}

func TestBuildArgvMissingTool(t *testing.T) {
	tc := Toolchain{}
	_, err := BuildArgv(FormatZip, "/a.zip", "/d", tc)
	if err == nil {
		t.Fatal("expected error when unzip missing")
	}
}

func TestFormatAvailable(t *testing.T) {
	tc := Toolchain{Tar: "/bin/tar"}
	if !FormatTarGz.Available(tc) {
		t.Fatal("tar.gz should be available")
	}
	if FormatZip.Available(tc) {
		t.Fatal("zip should not be available without unzip")
	}
}
