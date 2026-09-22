package panel

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// SortState describes the panel-local sort configuration.
type SortState struct {
	Mode                  SortMode
	Reverse               bool
	DirectoriesFirst      bool
	DiskUsageIdleSizeSort bool // After a disk-usage scan finishes, sort by cached sizes largest-first once idle (see config delay).
	// MetaColumn is the active meta column's EntryName when Mode is SortMeta.
	MetaColumn string
}

// SortMode describes the sort key for panel entries.
type SortMode int

const (
	SortName SortMode = iota
	SortExtension
	SortSize
	SortMtime
	// SortMeta sorts by an active meta column's value (SortState.MetaColumn). Not settable via
	// config default_sort — meta columns are only known once a panel activates them.
	SortMeta
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
	case SortMeta:
		return "Meta"
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

// SortDialogRadio describes one sort-mode radio in the sort dialog.
type SortDialogRadio struct {
	Mode     SortMode
	Label    string
	Shortcut rune
}

// SortDialogRadios is the canonical radio list for the sort dialog and its key handler.
func SortDialogRadios() []SortDialogRadio {
	return []SortDialogRadio{
		{SortName, "Name", 'n'},
		{SortExtension, "Extension", 'e'},
		{SortSize, "Size", 's'},
		{SortMtime, "Modify time", 'm'},
	}
}

// ApplySort sorts s.Entries in-place using the current sort state, then (in tree mode) resyncs
// TreeRoots/treeRows so the visible tree order matches — see resyncTreeOrder for why that's needed.
func (s *State) ApplySort() {
	SortEntries(s.Entries, s.Sort, s.DiskSorter, s.primarySortUsesDiskTotals(), s.MetaValue)
	s.resyncTreeOrder()
}

// SortEntries sorts entries in place per sortState, tie-breaking by name then path.
// useDiskPrimary switches the primary sort key to diskSorter's cached totals (used for the
// idle disk-usage sort in flat mode); callers that never want this (e.g. tree-mode child
// loads) pass false regardless of the panel's own DiskUsageIdleSizeSort setting. metaValue
// resolves a meta column's raw value for a path (used only when sortState.Mode == SortMeta);
// nil is fine when meta sorting is never used by the caller.
func SortEntries(entries []localfs.Entry, sortState SortState, diskSorter func(string) (int64, bool), useDiskPrimary bool, metaValue func(column string) (values map[string]string, pending string, ok bool)) {
	if len(entries) == 0 {
		return
	}

	var metaKeys map[string]metaSortKey
	metaDesc := false
	if !useDiskPrimary && sortState.Mode == SortMeta {
		var numeric bool
		metaKeys, numeric = buildMetaSortKeys(entries, sortState.MetaColumn, metaValue)
		metaDesc = !sortState.MetaAscending(numeric)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]

		// Directories first
		if sortState.DirectoriesFirst && left.Type != right.Type {
			if left.Type == localfs.EntryDirectory {
				return true
			}
			if right.Type == localfs.EntryDirectory {
				return false
			}
		}

		reverse := sortState.Reverse

		// Primary sort key
		var cmp int
		switch {
		case useDiskPrimary:
			cmp = compareDiskUsagePrimary(left, right, diskSorter, false)
			if cmp != 0 {
				// Largest cached totals first; unknown sizes stay last (handled inside compareDiskUsagePrimary).
				return cmp < 0
			}
		case sortState.Mode == SortMeta:
			cmp = compareMetaKeys(metaKeys[left.Path], metaKeys[right.Path], metaDesc)
			if cmp != 0 {
				return cmp < 0
			}
		default:
			cmp = compareByMode(left, right, sortState.Mode)
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

// ActiveMetaSortColumn returns the meta column entries are actually ordered by, or "" when
// the sort is not by a meta column or the disk-usage size sort has taken over (see SortEntries).
func (s *State) ActiveMetaSortColumn() string {
	if s.Sort.Mode != SortMeta || s.primarySortUsesDiskTotals() {
		return ""
	}
	return s.Sort.MetaColumn
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

// metaSortKey is one entry's precomputed sort key for an active meta column, resolved once per
// sort (see buildMetaSortKeys) instead of re-parsing the raw string on every comparison.
type metaSortKey struct {
	missing bool // no value, or blank/pending after trimming; always sorts last
	numeric bool
	num     float64
	str     string // lowercased, only meaningful when !numeric && !missing
}

// SortArrow returns the header arrow for a sort order: it points toward the larger values,
// so ↓ when values grow down the list (ascending) and ↑ when they shrink.
func SortArrow(ascending bool) rune {
	if ascending {
		return '↓'
	}
	return '↑'
}

// MetaAscending reports whether a meta column sorts ascending: numeric columns sort
// high-first by default, text columns A→Z; Reverse flips either.
func (s SortState) MetaAscending(numeric bool) bool {
	return numeric == s.Reverse
}

// MetaValuesNumeric reports whether a meta column is numeric: at least one value is set and
// every set value (non-blank, not the pending placeholder) parses as a float.
func MetaValuesNumeric(values map[string]string, pending string) bool {
	found := false
	for _, v := range values {
		if pending != "" && v == pending {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return false
		}
		found = true
	}
	return found
}

// buildMetaSortKeys resolves col's values once via metaValue and precomputes each entry's sort
// key (numeric parse, lowercased string, or missing) so compareMetaKeys never touches the raw
// string map or strconv during the sort itself. numeric reports MetaValuesNumeric for the column.
func buildMetaSortKeys(entries []localfs.Entry, col string, metaValue func(column string) (values map[string]string, pending string, ok bool)) (keys map[string]metaSortKey, numeric bool) {
	keys = make(map[string]metaSortKey, len(entries))
	var values map[string]string
	var pending string
	var ok bool
	if metaValue != nil {
		values, pending, ok = metaValue(col)
	}
	for _, e := range entries {
		if !ok {
			keys[e.Path] = metaSortKey{missing: true}
			continue
		}
		v, exists := values[e.Path]
		if !exists || (pending != "" && v == pending) {
			keys[e.Path] = metaSortKey{missing: true}
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			keys[e.Path] = metaSortKey{missing: true}
			continue
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			keys[e.Path] = metaSortKey{numeric: true, num: f}
			continue
		}
		keys[e.Path] = metaSortKey{str: strings.ToLower(v)}
	}
	return keys, MetaValuesNumeric(values, pending)
}

// compareMetaKeys orders two precomputed meta sort keys: numeric compare when both are numeric,
// numbers before text when only one is, otherwise case-insensitive string compare. Missing keys
// sort last regardless of reverse, same rule as compareDiskUsagePrimary's unknown sizes; reverse
// only flips the ordering between two known values.
func compareMetaKeys(l, r metaSortKey, reverse bool) int {
	if l.missing && r.missing {
		return 0
	}
	if l.missing {
		return 1
	}
	if r.missing {
		return -1
	}
	var cmp int
	switch {
	case l.numeric && r.numeric:
		cmp = floatCompare(l.num, r.num)
	case l.numeric:
		cmp = -1
	case r.numeric:
		cmp = 1
	default:
		cmp = stringsCompare(l.str, r.str)
	}
	if reverse {
		return -cmp
	}
	return cmp
}

// listNameColumnTitle returns the name-column header. With icons, a leading space
// aligns "Name" with entry names (icon strip is separate). The sort arrow replaces
// that space so "↑Name" lines up with " filename", not "↑ Name".
func listNameColumnTitle(showIcons bool, arrow rune) string {
	if arrow != 0 {
		return fmt.Sprintf("%cName", arrow)
	}
	if showIcons {
		return " Name"
	}
	return "Name"
}

// ListColumnTitles builds panel header labels with the SortArrow on the active sort column.
func (s State) ListColumnTitles(showIcons bool) (nameTitle, sizeTitle, thirdTitle string) {
	nameBase := listNameColumnTitle(showIcons, 0)
	f := EffectiveListFormat(s.ListFormat)
	if s.primarySortUsesDiskTotals() {
		switch f {
		case ListFormatBrief:
			return nameBase, fmt.Sprintf("%cSize", SortArrow(false)), ""
		case ListFormatPerm:
			return nameBase, fmt.Sprintf("%cSize", SortArrow(false)), "Permissions"
		default:
			return nameBase, fmt.Sprintf("%cSize", SortArrow(false)), "Modified"
		}
	}
	const lblMod = "Modified"
	const lblPerm = "Permissions"
	if s.Sort.Mode == SortMeta {
		// No arrow on the built-in Name/Size/Modified/Permissions columns; the meta column
		// header carries the arrow instead (see panelListHeader).
		switch f {
		case ListFormatBrief:
			return nameBase, "Size", ""
		case ListFormatPerm:
			return nameBase, "Size", lblPerm
		default:
			return nameBase, "Size", lblMod
		}
	}
	arrow := SortArrow(!s.Sort.Reverse)
	if f == ListFormatBrief {
		switch s.Sort.Mode {
		case SortName, SortExtension:
			return listNameColumnTitle(showIcons, arrow), "Size", ""
		case SortSize:
			return nameBase, fmt.Sprintf("%cSize", arrow), ""
		case SortMtime:
			// Brief has no Modified column to attach the arrow to; omit it rather
			// than misattributing it to Size.
			return nameBase, "Size", ""
		default:
			return listNameColumnTitle(showIcons, arrow), "Size", ""
		}
	}
	if f == ListFormatPerm {
		switch s.Sort.Mode {
		case SortName, SortExtension:
			return listNameColumnTitle(showIcons, arrow), "Size", lblPerm
		case SortSize:
			return nameBase, fmt.Sprintf("%cSize", arrow), lblPerm
		case SortMtime:
			// Perm has no Modified column to attach the arrow to; omit it rather
			// than misattributing it to Permissions.
			return nameBase, "Size", lblPerm
		default:
			return listNameColumnTitle(showIcons, arrow), "Size", lblPerm
		}
	}
	switch s.Sort.Mode {
	case SortName, SortExtension:
		return listNameColumnTitle(showIcons, arrow), "Size", lblMod
	case SortSize:
		return nameBase, fmt.Sprintf("%cSize", arrow), lblMod
	case SortMtime:
		return nameBase, "Size", fmt.Sprintf("%c%s", arrow, lblMod)
	default:
		return listNameColumnTitle(showIcons, arrow), "Size", lblMod
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

func floatCompare(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
