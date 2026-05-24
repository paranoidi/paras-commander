package sftp

import (
	"fmt"
	"path"
	"strings"

	pkgsftp "github.com/pkg/sftp"
)

// resolveRemotePath expands SFTP home markers (~, ~/…) to the server path.
// Explicit / is left unchanged (filesystem root).
func resolveRemotePath(client *pkgsftp.Client, remote string) (string, error) {
	if !isTildeRemote(remote) {
		return remote, nil
	}
	home, err := client.RealPath(".")
	if err != nil {
		return "", fmt.Errorf("resolve remote home: %w", err)
	}
	switch {
	case remote == "~":
		return home, nil
	case strings.HasPrefix(remote, "~/"):
		return path.Join(home, strings.TrimPrefix(remote, "~/")), nil
	default:
		return remote, nil
	}
}

func isTildeRemote(remote string) bool {
	return remote == "~" || strings.HasPrefix(remote, "~/")
}

// expandRemoteTilde maps tilde-marked paths to absolute paths using a known home directory.
// Used by tests; production uses resolveRemotePath with a live SFTP client.
func expandRemoteTilde(home, remote string) (string, error) {
	if home == "" {
		return "", fmt.Errorf("resolve remote home: empty home")
	}
	if !isTildeRemote(remote) {
		return remote, nil
	}
	switch {
	case remote == "~":
		return home, nil
	case strings.HasPrefix(remote, "~/"):
		return path.Join(home, strings.TrimPrefix(remote, "~/")), nil
	default:
		return remote, nil
	}
}
