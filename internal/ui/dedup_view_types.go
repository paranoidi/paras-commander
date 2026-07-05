package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
)

// DedupEntry is one row in the cached, flat dedup results list (group order, then rel
// within a group). AbsKey is the mark-map key computed once at sync time.
type DedupEntry struct {
	File       comparepkg.DedupFile
	AbsKey     string
	GroupFirst bool
	Size       int64
	Copies     int
}

// DedupViewState tracks cursor, scrolling, marks, and the delete prompt for the
// find-duplicates screen.
type DedupViewState struct {
	Selected           int
	ListScroll         int
	Marked             map[string]bool // absolute paths marked for deletion
	MarkedCount        int
	MarkedReclaimBytes int64
	SortByWasted       bool // false = order by path (default); true = most space wasted first
	IgnoreEmpty        bool // hide zero-byte duplicate groups (default true, set on Open)
	IgnoredEmptyCount  int  // files hidden by IgnoreEmpty, for the title
}

// DedupEntriesFromSnapshot builds the flat display list from a done-phase snapshot,
// applying the view's sort order and ignore-empty filter. Returns nil when the
// results list is not shown (walking, hashing, etc.). ignoredEmpty is the number
// of files dropped by the ignore-empty filter.
func DedupEntriesFromSnapshot(snap comparepkg.DedupSnapshot, sortByWasted, ignoreEmpty bool) (rows []DedupEntry, ignoredEmpty int) {
	if snap.Phase != comparepkg.DedupDone {
		return nil, 0
	}
	groups := make([]comparepkg.DedupGroup, 0, len(snap.Groups))
	for _, g := range snap.Groups {
		if ignoreEmpty && g.Size == 0 {
			ignoredEmpty += len(g.Files)
			continue
		}
		groups = append(groups, g)
	}
	if sortByWasted {
		slices.SortFunc(groups, comparepkg.DedupGroupBySize)
	} else {
		slices.SortFunc(groups, func(a, b comparepkg.DedupGroup) int {
			return cmp.Compare(a.Files[0].Rel, b.Files[0].Rel)
		})
	}
	for _, g := range groups {
		copies := len(g.Files)
		for i, f := range g.Files {
			rows = append(rows, DedupEntry{
				File:       f,
				AbsKey:     f.Abs.String(),
				GroupFirst: i == 0,
				Size:       g.Size,
				Copies:     copies,
			})
		}
	}
	return rows, ignoredEmpty
}

// EnsureSelectionVisible clamps the selected row and scroll offset.
func (s *DedupViewState) EnsureSelectionVisible(total int, visibleRows int) {
	if total == 0 {
		s.Selected = 0
		s.ListScroll = 0
		return
	}
	if s.Selected >= total {
		s.Selected = total - 1
	}
	if s.Selected < 0 {
		s.Selected = 0
	}
	if visibleRows <= 0 {
		return
	}
	if s.Selected < s.ListScroll {
		s.ListScroll = s.Selected
	}
	if s.Selected >= s.ListScroll+visibleRows {
		s.ListScroll = s.Selected - visibleRows + 1
	}
	maxScroll := max(0, total-visibleRows)
	if s.ListScroll > maxScroll {
		s.ListScroll = maxScroll
	}
	if s.ListScroll < 0 {
		s.ListScroll = 0
	}
}

// DedupGroupFullyMarked reports whether every file in entry idx's duplicate group
// is marked for deletion.
func DedupGroupFullyMarked(list []DedupEntry, marked map[string]bool, idx int) bool {
	if idx < 0 || idx >= len(list) {
		return false
	}
	start, end := dedupGroupBounds(list, idx)
	for i := start; i < end; i++ {
		if !marked[list[i].AbsKey] {
			return false
		}
	}
	return end > start
}

// DedupRedundantUnder returns the AbsKeys of duplicate copies under the selected
// row's directory that can be deleted while leaving exactly one surviving copy of
// each content group. When a copy already survives outside the directory, every
// under-directory copy is redundant; otherwise the first under-directory copy is
// kept. Result is empty when nothing under the directory is safe to drop.
func DedupRedundantUnder(list []DedupEntry, selected int) []string {
	if selected < 0 || selected >= len(list) {
		return nil
	}
	dir := list[selected].File.Abs.Parent().String() + "/"
	var out []string
	for start := 0; start < len(list); {
		_, end := dedupGroupBounds(list, start)
		var under []string
		outside := 0
		for i := start; i < end; i++ {
			// Trailing slash on both sides keeps "/a/b" from matching "/a/bc".
			if strings.HasPrefix(list[i].AbsKey+"/", dir) {
				under = append(under, list[i].AbsKey)
			} else {
				outside++
			}
		}
		if outside == 0 && len(under) > 0 {
			under = under[1:] // keep the first copy alive
		}
		out = append(out, under...)
		start = end
	}
	return out
}

func dedupGroupBounds(list []DedupEntry, idx int) (start, end int) {
	start = idx
	for start > 0 && !list[start].GroupFirst {
		start--
	}
	end = start + 1
	for end < len(list) && !list[end].GroupFirst {
		end++
	}
	return start, end
}

// MarkedSummary returns the end-label text for marked files, or "" when none.
func (s DedupViewState) MarkedSummary() string {
	if s.MarkedCount == 0 {
		return ""
	}
	return fmt.Sprintf("%d marked · %s", s.MarkedCount, formatJobBytes(s.MarkedReclaimBytes))
}
