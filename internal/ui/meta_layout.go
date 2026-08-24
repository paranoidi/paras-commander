package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
)

// MetaColumnLayout is a rendered meta column ready for panel list rows.
type MetaColumnLayout struct {
	Title     string
	Width     int
	Formatted map[string]string
}

// LayoutMetaColumns formats each active meta column and returns layouts plus total terminal width
// (including two-cell gaps between columns).
func LayoutMetaColumns(cols []MetaColumnState) (layouts []MetaColumnLayout, totalWidth int) {
	if len(cols) == 0 {
		return nil, 0
	}
	layouts = make([]MetaColumnLayout, len(cols))
	for i, col := range cols {
		w, formatted := layoutMetaCells(col.Results)
		layouts[i] = MetaColumnLayout{
			Title:     col.ColumnTitle,
			Width:     w,
			Formatted: formatted,
		}
		if i > 0 {
			totalWidth += 2
		}
		totalWidth += w
	}
	return layouts, totalWidth
}

// MetaHeaderText returns the padded header segment for meta columns.
func MetaHeaderText(layouts []MetaColumnLayout) string {
	if len(layouts) == 0 {
		return ""
	}
	parts := make([]string, len(layouts))
	for i, lay := range layouts {
		parts[i] = padMetaLineToWidth(lay.Title, lay.Width)
	}
	return strings.Join(parts, "  ")
}

// MetaRowText returns the padded meta segment for one file row.
func MetaRowText(layouts []MetaColumnLayout, path string) string {
	if len(layouts) == 0 {
		return ""
	}
	parts := make([]string, len(layouts))
	for i, lay := range layouts {
		text := ""
		if lay.Formatted != nil {
			text = lay.Formatted[path]
		}
		parts[i] = padMetaLineToWidth(text, lay.Width)
	}
	return strings.Join(parts, "  ")
}

const (
	// panelListMetaMaxFields caps how many tab/newline-delimited fields a meta command may emit.
	panelListMetaMaxFields = 8
	// panelMetaRawMaxBytes clips absurdly large stdout before splitting.
	panelMetaRawMaxBytes = 16384
)

// layoutMetaCells formats meta command stdout for the panel column.
// Tab (\t) and line feed (\n) delimit fields (after \r\n/\r normalization); if neither appears
// in the trimmed payload, the whole string is one legacy cell (width capped by panelListMetaMax).
// Cells that still overflow after shrinking are clipped with a trailing ellipsis.
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
		fields := parseMetaRaw(raw)
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

// parseMetaRaw splits one command's stdout into fields.
func parseMetaRaw(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	trimmed = clipMetaRawBytes(trimmed)

	if !strings.ContainsAny(trimmed, "\t\n") {
		return []string{trimmed}
	}

	norm := strings.ReplaceAll(trimmed, "\r\n", "\n")
	norm = strings.ReplaceAll(norm, "\r", "\n")
	norm = strings.ReplaceAll(norm, "\n", "\t")
	out := strings.Split(norm, "\t")
	if len(out) > panelListMetaMaxFields {
		out = out[:panelListMetaMaxFields]
	}
	return out
}

func clipMetaRawBytes(s string) string {
	if len(s) <= panelMetaRawMaxBytes {
		return s
	}
	s = s[:panelMetaRawMaxBytes]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func truncateMetaDisplay(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= w {
		return s
	}
	return runewidth.Truncate(s, w, string(primitive.Ellipsis))
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
		s = truncateMetaDisplay(s, w)
	}
	return runewidth.FillRight(s, w)
}

func alignMetaCell(s string, colW int, rightDigits bool) string {
	if colW <= 0 {
		return ""
	}
	t := s
	if runewidth.StringWidth(s) > colW {
		t = truncateMetaDisplay(s, colW)
	}
	if rightDigits && metaCellAllDigits(t) {
		return runewidth.FillLeft(t, colW)
	}
	return runewidth.FillRight(t, colW)
}
