package scan

import (
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fswalk"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/search"
)

// StartOpts configures a new find scan session.
type StartOpts struct {
	Gen                  int
	DisplayRoot          string
	Roots                []string
	IncludeHidden        bool
	Gitignore            *gitignore.Cache
	VolumeGate           diskusage.ListingVolumeGate
	SelectionRoots       []string
	SearchOnlySelections bool
	Walk                 fswalk.Params
}

// Event is sent from the scan coordinator to the UI thread.
type Event struct {
	Gen int

	CountUpdate bool
	Count       int
	WalkWorkers int
	IndexActive bool

	IndexFinished bool
	IndexErr      string

	// IndexReplaced is set after a bulk in-place index rewrite (e.g. include-hidden off).
	IndexReplaced bool

	MatchResult bool
	Match       MatchOutput
}

// MatchOutput is a completed rank/filter pass over the coordinator index.
type MatchOutput struct {
	Gen             int
	Ranked          []int
	FullRanked      []int
	MatchRanges     map[int][]search.Range
	DisplayRelLines []string // parallel to Ranked when UI entries not yet synced
	EntriesLen      int
	OnlyDirs        bool
	OnlyFiles       bool
}

// MatchRequest asks the match coordinator to rank the current index.
type MatchRequest struct {
	Gen             int
	Query           string
	OnlyDirs        bool
	OnlyFiles       bool
	CaseInsensitive bool
	MaxResults      int
}
