package localfs

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

// Rename performs a same-directory rename.
func Rename(oldPath, newPath string) error {
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("rename %q to %q: %w", oldPath, newPath, err)
	}
	return nil
}

// Mkdir creates a single directory (no parent creation).
func Mkdir(path string, perm os.FileMode) error {
	if err := os.Mkdir(path, perm); err != nil {
		return fmt.Errorf("mkdir %q: %w", path, err)
	}
	return nil
}

// Remove removes a single file or empty directory.
func Remove(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove %q: %w", path, err)
	}
	return nil
}

// RemoveAll removes a file or directory tree recursively.
func RemoveAll(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove all %q: %w", path, err)
	}
	return nil
}

// Chmod changes the permissions of a file or directory.
func Chmod(path string, mode os.FileMode) error {
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("chmod %q: %w", path, err)
	}
	return nil
}

// Chown changes the owner and group of a file or directory.
// uid and gid may be -1 to leave unchanged.
func Chown(path string, uid, gid int) error {
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("chown %q: %w", path, err)
	}
	return nil
}

// Symlink creates a symbolic link.
func Symlink(target, linkPath string) error {
	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("symlink %q -> %q: %w", linkPath, target, err)
	}
	return nil
}

// Link creates a hard link.
func Link(source, newPath string) error {
	if err := os.Link(source, newPath); err != nil {
		return fmt.Errorf("link %q -> %q: %w", newPath, source, err)
	}
	return nil
}

// LookupUser resolves a username string to a uid.
// Returns -1 if the name is empty (unchanged) or on lookup failure.
func LookupUser(name string) (int, error) {
	if name == "" {
		return -1, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		// Try numeric uid.
		uid, parseErr := strconv.Atoi(name)
		if parseErr != nil {
			return -1, fmt.Errorf("lookup user %q: %w", name, err)
		}
		return uid, nil
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return -1, fmt.Errorf("parse uid for user %q: %w", name, err)
	}
	return uid, nil
}

// LookupGroup resolves a group name string to a gid.
// Returns -1 if the name is empty (unchanged) or on lookup failure.
func LookupGroup(name string) (int, error) {
	if name == "" {
		return -1, nil
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		// Try numeric gid.
		gid, parseErr := strconv.Atoi(name)
		if parseErr != nil {
			return -1, fmt.Errorf("lookup group %q: %w", name, err)
		}
		return gid, nil
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return -1, fmt.Errorf("parse gid for group %q: %w", name, err)
	}
	return gid, nil
}

// ResolveRelative resolves a path relative to baseDir.
// If path is already absolute, it is returned as-is (cleaned).
func ResolveRelative(path, baseDir string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}
