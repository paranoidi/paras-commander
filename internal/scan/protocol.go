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
// Pay attention: the UI must not read the coordinator index directly — only mirror
// these payloads into FindDialogState on the main thread (see llm-docs/navigation.md).
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

	// BatchAdded carries newly indexed entries for incremental UI mirror updates.
	BatchAdded []Entry
	// ReplacedEntries is the full corpus after IndexReplaced (narrow/strip); incremental
	// indexing uses BatchAdded only and IndexFinished carries CountUpdate without a bulk replace.
	ReplacedEntries []Entry

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
