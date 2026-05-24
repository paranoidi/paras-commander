package sftp

import "testing"

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
