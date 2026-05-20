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
