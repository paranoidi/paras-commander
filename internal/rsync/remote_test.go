package rsync

import (
	"errors"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestResolveLocal(t *testing.T) {
	got, err := Resolve(pathloc.PathsForTest("/srv/harbor/orchard"), pathloc.PathsForTest("/srv/meadow")[0])
	if err != nil {
		t.Fatal(err)
	}
	if got.SSH != "" || got.Sources[0] != "/srv/harbor/orchard" || got.Dest != "/srv/meadow" {
		t.Fatalf("targets = %+v", got)
	}
}

func TestResolveRemoteDestination(t *testing.T) {
	dest := pathloc.MustParse("sftp://ann@willow.example:2222/srv/meadow")
	got, err := Resolve(pathloc.PathsForTest("/srv/harbor/orchard"), dest)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dest != "ann@willow.example:/srv/meadow" || got.SSH != "ssh -o BatchMode=yes -p 2222" {
		t.Fatalf("targets = %+v", got)
	}
}

func TestResolveRemoteSourcesHomeRelative(t *testing.T) {
	src := []pathloc.Path{pathloc.MustParse("sftp://willow.example/~/orchard"), pathloc.MustParse("sftp://willow.example/~/lantern")}
	got, err := Resolve(src, pathloc.PathsForTest("/srv/meadow")[0])
	if err != nil {
		t.Fatal(err)
	}
	if got.Sources[0] != "willow.example:~/orchard" || got.Sources[1] != "willow.example:~/lantern" || got.SSH != "ssh -o BatchMode=yes" {
		t.Fatalf("targets = %+v", got)
	}
}

func TestResolveRejectsTwoRemoteSides(t *testing.T) {
	src := pathloc.MustParse("sftp://willow.example/srv/orchard")
	for _, dest := range []string{"sftp://willow.example/srv/meadow", "sftp://harbor.example/srv/meadow"} {
		if _, err := Resolve([]pathloc.Path{src}, pathloc.MustParse(dest)); !errors.Is(err, errBothRemote) {
			t.Fatalf("dest %s: err = %v, want errBothRemote", dest, err)
		}
	}
}

func TestResolveRejectsFileIndexLocations(t *testing.T) {
	idx, err := pathloc.FileIndex("/srv/orchard")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve([]pathloc.Path{idx}, pathloc.PathsForTest("/srv/meadow")[0]); err == nil {
		t.Fatal("a fileindex:// source must be rejected")
	}
}
