package compare

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

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
