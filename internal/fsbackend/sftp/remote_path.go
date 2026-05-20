package sftp

import (
	"fmt"
	"strings"

	pkgsftp "github.com/pkg/sftp"
)

// resolveRemotePath expands SFTP URI home markers (~, /~) to the server path.
// Explicit / is left unchanged (filesystem root).
func resolveRemotePath(client *pkgsftp.Client, remote string) (string, error) {
	switch {
	case remote == "~", remote == "/~":
		resolved, err := client.RealPath(".")
		if err != nil {
			return "", fmt.Errorf("resolve remote home: %w", err)
		}
		return resolved, nil
	case strings.HasPrefix(remote, "~/"):
		resolved, err := client.RealPath(remote)
		if err != nil {
			return "", fmt.Errorf("resolve remote path %q: %w", remote, err)
		}
		return resolved, nil
	case strings.HasPrefix(remote, "/~/"):
		resolved, err := client.RealPath(strings.TrimPrefix(remote, "/"))
		if err != nil {
			return "", fmt.Errorf("resolve remote path %q: %w", remote, err)
		}
		return resolved, nil
	default:
		return remote, nil
	}
}
