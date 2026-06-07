package gitignore

import (
	"os"
	"path/filepath"
	"strings"
)

// ValidWorkTreeRoot returns the work tree root when dir is inside a Git repository
// with usable metadata (.git/HEAD or a linked gitdir), or "" otherwise.
// Results are cached so descendants of a known work tree skip repeated parent walks.
func ValidWorkTreeRoot(dir string) string {
	return sharedWorkTreeResolver.validWorkTreeRoot(dir)
}

// WorkTreeRoot walks parents from dir until a .git file or directory exists.
// Returns the directory containing .git, or "" if not inside a Git work tree.
// Results are cached so descendants of a known work tree skip repeated parent walks.
func WorkTreeRoot(dir string) string {
	return sharedWorkTreeResolver.workTreeRoot(dir)
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

func gitMetadataValid(workRoot string) bool {
	return gitDirHasHEAD(resolveGitDir(filepath.Join(workRoot, ".git"), workRoot))
}

func resolveGitDir(gitEntry, workRoot string) string {
	st, err := os.Stat(gitEntry)
	if err != nil {
		return ""
	}
	if st.IsDir() {
		return gitEntry
	}
	data, err := os.ReadFile(gitEntry)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	const prefix = "gitdir: "
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	ref := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if ref == "" {
		return ""
	}
	if !filepath.IsAbs(ref) {
		ref = filepath.Join(workRoot, ref)
	}
	return filepath.Clean(ref)
}

func gitDirHasHEAD(gitDir string) bool {
	if gitDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(gitDir, "HEAD"))
	return err == nil
}
