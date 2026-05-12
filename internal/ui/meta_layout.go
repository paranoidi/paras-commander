package ui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

const (
	// panelListMetaMaxFields caps how many tab/newline-delimited fields a meta command may emit.
	panelListMetaMaxFields = 8
	// panelMetaRawMaxBytes rejects absurdly large stdout before splitting.
	panelMetaRawMaxBytes = 16384
)

// layoutMetaCells formats meta command stdout for the panel column.
// Tab (\t) and line feed (\n) delimit fields (after \r\n/\r normalization); if neither appears
// in the trimmed payload, the whole string is one legacy cell (width capped by panelListMetaMax).
// Column count is the maximum field count across all non-empty rows; shorter rows pad with empty cells.
func layoutMetaCells(metaResults map[string]string) (metaColW int, formatted map[string]string) {
	formatted = make(map[string]string, len(metaResults))
	if len(metaResults) == 0 {
		return panelListMetaMinCells, formatted
	}

	parsed := make(map[string][]string, len(metaResults))
	nCols := 0

	for path, raw := range metaResults {
		if raw == "" {
			formatted[path] = ""
			continue
		}
		fields, tooLong := parseMetaRaw(raw)
		if tooLong {
			formatted[path] = "too long"
			continue
		}
		if len(fields) == 0 {
			formatted[path] = ""
			continue
		}
		parsed[path] = fields
		if len(fields) > nCols {
			nCols = len(fields)
		}
	}

	if nCols == 0 {
		return clampMetaColW(panelListMetaMinCells), formatted
	}

	colW := make([]int, nCols)
	for j := 0; j < nCols; j++ {
		maxw := 0
		for _, row := range parsed {
			cell := ""
			if j < len(row) {
				cell = row[j]
			}
			if w := runewidth.StringWidth(cell); w > maxw {
				maxw = w
			}
		}
		colW[j] = maxw
	}

	gaps := nCols - 1
	if gaps < 0 {
		gaps = 0
	}
	total := gaps
	for _, w := range colW {
		total += w
	}
	for total > panelListMetaMax {
		bi := -1
		best := -1
		for i, w := range colW {
			if w > best {
				best = w
				bi = i
			}
		}
		if bi < 0 || best <= 0 {
			break
		}
		colW[bi]--
		total--
	}

	for path, row := range parsed {
		cells := make([]string, 0, nCols)
		for j := 0; j < nCols; j++ {
			cell := ""
			if j < len(row) {
				cell = row[j]
			}
			cells = append(cells, alignMetaCell(cell, colW[j], metaCellAllDigits(cell)))
		}
		formatted[path] = strings.Join(cells, " ")
	}

	metaColW = panelListMetaMinCells
	for _, s := range formatted {
		if w := runewidth.StringWidth(s); w > metaColW {
			metaColW = w
		}
	}
	return clampMetaColW(metaColW), formatted
}

func clampMetaColW(w int) int {
	if w < panelListMetaMinCells {
		return panelListMetaMinCells
	}
	if w > panelListMetaMax {
		return panelListMetaMax
	}
	return w
}

// parseMetaRaw splits one command's stdout into fields. tooLong is true if the value must not be shown.
func parseMetaRaw(raw string) (fields []string, tooLong bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, false
	}
	if len(trimmed) > panelMetaRawMaxBytes {
		return nil, true
	}

	if !strings.ContainsAny(trimmed, "\t\n") {
		if runewidth.StringWidth(trimmed) > panelListMetaMax {
			return nil, true
		}
		return []string{trimmed}, false
	}

	norm := strings.ReplaceAll(trimmed, "\r\n", "\n")
	norm = strings.ReplaceAll(norm, "\r", "\n")
	norm = strings.ReplaceAll(norm, "\n", "\t")
	out := strings.Split(norm, "\t")
	if len(out) > panelListMetaMaxFields {
		out = out[:panelListMetaMaxFields]
	}
	return out, false
}

func metaCellAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// padMetaLineToWidth pads or truncates the full meta column string to w terminal cells.
func padMetaLineToWidth(s string, w int) string {
	if w <= 0 {
		return s
	}
	if runewidth.StringWidth(s) > w {
		s = runewidth.Truncate(s, w, "")
	}
	return runewidth.FillRight(s, w)
}

func alignMetaCell(s string, colW int, rightDigits bool) string {
	if colW <= 0 {
		if runewidth.StringWidth(s) <= 0 {
			return ""
		}
		return runewidth.Truncate(s, 0, "")
	}
	t := s
	if runewidth.StringWidth(s) > colW {
		t = runewidth.Truncate(s, colW, "")
	}
	if rightDigits && metaCellAllDigits(t) {
		return runewidth.FillLeft(t, colW)
	}
	return runewidth.FillRight(t, colW)
}
