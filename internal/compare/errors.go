package compare

import (
	"errors"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// unsupportedRoot reports that compare cannot walk root; the message names its scheme.
func unsupportedRoot(root pathloc.Path) error {
	return errors.New(pathloc.Unavailable("Compare", root))
}

func firstNonLocal(roots ...pathloc.Path) (pathloc.Path, bool) {
	for _, r := range roots {
		if !r.IsLocal() {
			return r, true
		}
	}
	return pathloc.Path{}, false
}
