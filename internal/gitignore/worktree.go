package gitignore

import (
	"os"
	"path/filepath"
	"strings"
)

// WorkTreeRoot walks parents from dir until a .git file or directory exists.
// Returns the directory containing .git, or "" if not inside a Git work tree.
func WorkTreeRoot(dir string) string {
	dir, err := filepath.Abs(dir)
	if err != nil || dir == "" {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// dirsFromRootTo returns workRoot followed by each path segment down to target (both cleaned).
func dirsFromRootTo(workRoot, target string) []string {
	workRoot = filepath.Clean(workRoot)
	target = filepath.Clean(target)
	if workRoot == "" || target == "" {
		return nil
	}
	if target == workRoot {
		return []string{workRoot}
	}
	rel, err := filepath.Rel(workRoot, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	out := []string{workRoot}
	cur := workRoot
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		out = append(out, cur)
	}
	return out
}
