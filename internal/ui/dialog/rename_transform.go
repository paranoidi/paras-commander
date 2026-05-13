package dialog

import "strings"

// ApplyRenameSanitize returns s with selected replacements: dots to spaces first,
// then underscores to spaces (only for enabled flags).
func ApplyRenameSanitize(s string, dotsToSpace, underscoresToSpace bool) string {
	out := s
	if dotsToSpace {
		out = strings.ReplaceAll(out, ".", " ")
	}
	if underscoresToSpace {
		out = strings.ReplaceAll(out, "_", " ")
	}
	return out
}

// RenameSlugifySep is the replacement character for spaces in slugify.
type RenameSlugifySep byte

const (
	// RenameSlugifyDot replaces each ASCII space with '.'.
	RenameSlugifyDot RenameSlugifySep = '.'
	// RenameSlugifyUnderscore replaces each ASCII space with '_'.
	RenameSlugifyUnderscore RenameSlugifySep = '_'
)

// ApplyRenameSlugify replaces each ASCII space with sep (1:1).
func ApplyRenameSlugify(s string, sep RenameSlugifySep) string {
	if sep != RenameSlugifyDot && sep != RenameSlugifyUnderscore {
		sep = RenameSlugifyDot
	}
	return strings.Map(func(r rune) rune {
		if r == ' ' {
			return rune(sep)
		}
		return r
	}, s)
}
