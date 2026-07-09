package compare

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// RelDir returns the slash-separated parent directory of a relative file path.
// Root-level files (no directory component) return "".
func RelDir(rel string) string {
	dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel)))
	if dir == "." {
		return ""
	}
	return dir
}

// RelBase returns the filename component of a slash-separated relative path.
func RelBase(rel string) string {
	return filepath.ToSlash(filepath.Base(filepath.FromSlash(rel)))
}

// JoinRel joins root with a slash-relative path.
func JoinRel(root pathloc.Path, rel string) (pathloc.Path, error) {
	if root.IsRemote() {
		return pathloc.Path{}, ErrRemoteUnsupported()
	}
	host, err := root.FilePath()
	if err != nil {
		return pathloc.Path{}, err
	}
	return pathloc.File(filepath.Join(host, filepath.FromSlash(rel)))
}
