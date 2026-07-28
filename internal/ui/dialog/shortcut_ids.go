package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/search"
)

// ShortcutIDs returns n unique lowercase letter IDs of uniform length (at least 2).
// Order is aa, ab, … az, ba, … then longer strings when 26^len is insufficient.
func ShortcutIDs(n int) []string {
	if n <= 0 {
		return nil
	}
	width := 2
	for pow26(width) < n {
		width++
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = encodeBase26(i, width)
	}
	return out
}

func pow26(width int) int {
	p := 1
	for i := 0; i < width; i++ {
		p *= 26
	}
	return p
}

func encodeBase26(i, width int) string {
	b := make([]byte, width)
	for j := width - 1; j >= 0; j-- {
		b[j] = byte('a' + i%26)
		i /= 26
	}
	return string(b)
}

// AssignPathPickerShortcutIDs sets PathPickerItem.ID for every item in list order.
func AssignPathPickerShortcutIDs(items []PathPickerItem) {
	ids := ShortcutIDs(len(items))
	for i := range items {
		items[i].ID = ids[i]
	}
}

// PreferExactShortcutID collapses ranked results to the single entry whose ID equals
// query when the whole query is an exact ID match. Otherwise returns ranked unchanged.
func PreferExactShortcutID(
	ids []string,
	query string,
	ranked []int,
	matchRanges [][]search.Range,
	caseInsensitive bool,
) (outRanked []int, outRanges [][]search.Range) {
	q := strings.TrimSpace(query)
	if q == "" || len(ids) == 0 {
		return ranked, matchRanges
	}
	for i, id := range ids {
		ok := id == q
		if caseInsensitive {
			ok = strings.EqualFold(id, q)
		}
		if !ok {
			continue
		}
		outRanges = make([][]search.Range, len(matchRanges))
		if len(outRanges) > i {
			outRanges[i] = []search.Range{{Start: 0, End: utf8.RuneCountInString(id)}}
		}
		return []int{i}, outRanges
	}
	return ranked, matchRanges
}
