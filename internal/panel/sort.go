package panel

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// SortState describes the panel-local sort configuration.
type SortState struct {
	Mode                  SortMode
	Reverse               bool
	DirectoriesFirst      bool
	DiskUsageIdleSizeSort bool // After a disk-usage scan finishes, sort by cached sizes largest-first once idle (see config delay).
}

// SortMode describes the sort key for panel entries.
type SortMode int

const (
	SortName SortMode = iota
	SortExtension
	SortSize
	SortMtime
)

// String returns the display label for the sort mode.
func (m SortMode) String() string {
	switch m {
	case SortName:
		return "Name"
	case SortExtension:
		return "Extension"
	case SortSize:
		return "Size"
	case SortMtime:
		return "Modified"
	default:
		return "Name"
	}
}

// ParseSortMode parses a sort mode from its config string.
func ParseSortMode(value string) (SortMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "name":
		return SortName, nil
	case "extension":
		return SortExtension, nil
	case "size":
		return SortSize, nil
	case "mtime":
		return SortMtime, nil
	default:
		return SortName, fmt.Errorf("unknown sort mode %q", value)
	}
}

// IterateSortModes returns sort modes in a predictable cycle.
func IterateSortModes() []SortMode {
	return []SortMode{SortName, SortExtension, SortSize, SortMtime}
}

// ApplySort sorts s.Entries in-place using the current sort state.
func (s *State) ApplySort() {
	if len(s.Entries) == 0 {
		return
	}

	sort.SliceStable(s.Entries, func(i, j int) bool {
		left := s.Entries[i]
		right := s.Entries[j]

		// Directories first
		if s.Sort.DirectoriesFirst && left.Type != right.Type {
			if left.Type == localfs.EntryDirectory {
				return true
			}
			if right.Type == localfs.EntryDirectory {
				return false
			}
		}

		reverse := s.Sort.Reverse

		// Primary sort key
		var cmp int
		if s.primarySortUsesDiskTotals() {
			cmp = compareDiskUsagePrimary(left, right, s.DiskSorter, false)
			if cmp != 0 {
				// Largest cached totals first; unknown sizes stay last (handled inside compareDiskUsagePrimary).
				return cmp < 0
			}
		} else {
			cmp = compareByMode(left, right, s.Sort.Mode)
			if cmp != 0 {
				if reverse {
					return cmp > 0
				}
				return cmp < 0
			}
		}

		// Tie-break: name ascending (always, even when reverse)
		cmp = stringsCompare(strings.ToLower(left.Name), strings.ToLower(right.Name))
		if cmp != 0 {
			return cmp < 0
		}
		cmp = stringsCompare(left.Name, right.Name)
		if cmp != 0 {
			return cmp < 0
		}

		// Final tie-break: absolute path ascending
		return left.Path < right.Path
	})
}

func compareByMode(left, right localfs.Entry, mode SortMode) int {
	switch mode {
	case SortName:
		return stringsCompare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	case SortExtension:
		return stringsCompare(extension(left.Name), extension(right.Name))
	case SortSize:
		return intCompare(left.Size, right.Size)
	case SortMtime:
		if left.ModifiedAt.Before(right.ModifiedAt) {
			return -1
		}
		if left.ModifiedAt.After(right.ModifiedAt) {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func (s *State) primarySortUsesDiskTotals() bool {
	return s.Sort.DiskUsageIdleSizeSort && s.IdleDiskTotalsSort
}

// compareDiskUsagePrimary orders by cached subtree or file aggregates from diskSorter.
// Unknown paths sort after any known path. Larger sizes sort first when reverse is false.
func compareDiskUsagePrimary(left, right localfs.Entry, diskSorter func(string) (int64, bool), reverse bool) int {
	leftKey := filepath.Clean(left.Path)
	rightKey := filepath.Clean(right.Path)

	okL := false
	var lv int64
	if diskSorter != nil {
		if n, ok := diskSorter(leftKey); ok {
			lv = n
			okL = true
		}
	}
	okR := false
	var rv int64
	if diskSorter != nil {
		if n, ok := diskSorter(rightKey); ok {
			rv = n
			okR = true
		}
	}

	if !okL && !okR {
		return 0
	}
	if !okL {
		return 1
	}
	if !okR {
		return -1
	}

	if reverse {
		return intCompare(lv, rv)
	}
	return intCompare(rv, lv)
}

// ListColumnTitles builds panel header labels with ↑/↓ on the active sort column.
func (s State) ListColumnTitles(showIcons bool) (nameTitle, sizeTitle, thirdTitle string) {
	const asc = '↑'
	const desc = '↓'
	nameBase := "Name"
	if showIcons {
		nameBase = " Name"
	}
	f := EffectiveListFormat(s.ListFormat)
	if s.primarySortUsesDiskTotals() {
		switch f {
		case ListFormatBrief:
			return nameBase, fmt.Sprintf("%cSize", desc), ""
		case ListFormatPerm:
			return nameBase, fmt.Sprintf("%cSize", desc), "Permissions"
		default:
			return nameBase, fmt.Sprintf("%cSize", desc), "Modified"
		}
	}
	arrow := asc
	if s.Sort.Reverse {
		arrow = desc
	}
	const lblMod = "Modified"
	const lblPerm = "Permissions"
	if f == ListFormatBrief {
		switch s.Sort.Mode {
		case SortName, SortExtension:
			return fmt.Sprintf("%c%s", arrow, nameBase), "Size", ""
		case SortSize:
			return nameBase, fmt.Sprintf("%cSize", arrow), ""
		case SortMtime:
			return nameBase, fmt.Sprintf("%cSize", arrow), ""
		default:
			return fmt.Sprintf("%c%s", arrow, nameBase), "Size", ""
		}
	}
	if f == ListFormatPerm {
		switch s.Sort.Mode {
		case SortName, SortExtension:
			return fmt.Sprintf("%c%s", arrow, nameBase), "Size", lblPerm
		case SortSize:
			return nameBase, fmt.Sprintf("%cSize", arrow), lblPerm
		case SortMtime:
			return nameBase, "Size", fmt.Sprintf("%c%s", arrow, lblPerm)
		default:
			return fmt.Sprintf("%c%s", arrow, nameBase), "Size", lblPerm
		}
	}
	switch s.Sort.Mode {
	case SortName, SortExtension:
		return fmt.Sprintf("%c%s", arrow, nameBase), "Size", lblMod
	case SortSize:
		return nameBase, fmt.Sprintf("%cSize", arrow), lblMod
	case SortMtime:
		return nameBase, "Size", fmt.Sprintf("%c%s", arrow, lblMod)
	default:
		return fmt.Sprintf("%c%s", arrow, nameBase), "Size", lblMod
	}
}

func extension(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 || idx == len(name)-1 {
		return ""
	}
	return name[idx+1:]
}

func stringsCompare(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func intCompare(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
