package findcrit

// IndexMode is how Find in index reads its query text.
type IndexMode int

const (
	// IndexTerms is the default: space-separated terms on basenames (see fileindex.Index.Find).
	IndexTerms IndexMode = iota
	// IndexPattern is one case-insensitive glob on basenames, a substring when it has no wildcard.
	IndexPattern
	// IndexBoolean is a raw FTS5 word query on file names (files only).
	IndexBoolean
	indexModeCount
)

// IndexModeLabels are the match mode radio labels, indexed by IndexMode.
var IndexModeLabels = [indexModeCount]string{"Terms", "Pattern", "Boolean"}

// Next is the mode after m (wrapping), for the cycle action.
func (m IndexMode) Next() IndexMode { return (m + 1) % indexModeCount }

// Tag is the short name shown in the query row and the criteria summary ("" for terms).
func (m IndexMode) Tag() string {
	switch m {
	case IndexPattern:
		return "pattern"
	case IndexBoolean:
		return "boolean"
	}
	return ""
}
