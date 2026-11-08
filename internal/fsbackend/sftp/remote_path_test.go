package sftp

import (
	"os"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	pkgsftp "github.com/pkg/sftp"
)

func TestResolveRemotePathPassthrough(t *testing.T) {
	t.Parallel()
	tests := []string{"/", "/var/www", "/tmp", ""}
	for _, remote := range tests {
		got, err := resolveRemotePath(nil, remote)
		if err != nil {
			t.Fatalf("resolveRemotePath(%q): %v", remote, err)
		}
		if got != remote {
			t.Fatalf("resolveRemotePath(%q) = %q, want passthrough", remote, got)
		}
	}
}

func TestExpandRemoteTilde(t *testing.T) {
	t.Parallel()
	const home = "/home/pi"
	tests := []struct {
		remote string
		want   string
	}{
		{"~", home},
		{"~/apps", home + "/apps"},
		{"~/apps/bin", home + "/apps/bin"},
		{"/var/log", "/var/log"},
	}
	for _, tc := range tests {
		got, err := expandRemoteTilde(home, tc.remote)
		if err != nil {
			t.Fatalf("expandRemoteTilde(%q): %v", tc.remote, err)
		}
		if got != tc.want {
			t.Fatalf("expandRemoteTilde(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

// sftpStatInfo is a remote listing row carrying SFTP attributes.
type sftpStatInfo struct {
	os.FileInfo
	stat *pkgsftp.FileStat
}

func (i sftpStatInfo) Name() string       { return "lantern.txt" }
func (i sftpStatInfo) Size() int64        { return 6 }
func (i sftpStatInfo) Mode() os.FileMode  { return 0o644 }
func (i sftpStatInfo) ModTime() time.Time { return time.Unix(1700000000, 0) }
func (i sftpStatInfo) IsDir() bool        { return false }
func (i sftpStatInfo) Sys() any           { return i.stat }

func TestEntryFromInfoReadsNumericOwnerAndAccessTime(t *testing.T) {
	info := sftpStatInfo{stat: &pkgsftp.FileStat{UID: 1001, GID: 1002, Atime: 1700000500}}
	e := entryFromInfo(pathloc.Path{}, info)
	if e.Owner != "1001" || e.Group != "1002" || e.AccessedAt.Unix() != 1700000500 {
		t.Fatalf("entry = %+v", e)
	}
	if e.Links != 0 || !e.ChangedAt.IsZero() || !e.CreatedAt.IsZero() {
		t.Fatalf("SFTP v3 has no links, ctime or birth time: %+v", e)
	}
}
