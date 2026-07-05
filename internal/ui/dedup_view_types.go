package ui

import (
	"fmt"

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
}

// DedupEntriesFromSnapshot builds the flat display list from a done-phase snapshot.
// Returns nil when the results list is not shown (walking, hashing, etc.).
func DedupEntriesFromSnapshot(snap comparepkg.DedupSnapshot) []DedupEntry {
	if snap.Phase != comparepkg.DedupDone {
		return nil
	}
	var rows []DedupEntry
	for _, g := range snap.Groups {
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
	return rows
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
