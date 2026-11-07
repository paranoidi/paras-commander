package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// metaColumnSep separates meta columns in the header and rows.
const metaColumnSep = "  "

// MetaColumnLayout is a rendered meta column ready for panel list rows.
type MetaColumnLayout struct {
	// EntryName identifies the source meta column (MetaColumnState.EntryName), used to match
	// the panel's SortState.MetaColumn when drawing the sort arrow on the right header.
	EntryName  string
	Title      string
	Width      int
	Formatted  map[string]string
	RightAlign bool
	// Numeric mirrors panel.MetaValuesNumeric: numeric columns sort high-first by default.
	Numeric bool
	// SortArrow is ↑/↓ when the panel sorts by this column, 0 otherwise.
	SortArrow rune
	// Pending is the in-flight marker (MetaColumnState.Pending); rows showing it are centered.
	Pending string
	// PendingHidden mirrors MetaColumnState.PendingHidden: rows showing Pending paint blank.
	PendingHidden bool
	// Levels holds the alert level of each alerting row; nil when none alerts. Shared with later
	// draws: read-only.
	Levels map[string]panel.MetaAlertLevel
}

// metaCellsCache is the part of a column's layout derived from Results and Pending alone.
type metaCellsCache struct {
	gen        uint64
	width      int
	formatted  map[string]string
	rightAlign bool
	numeric    bool
	levels     map[string]panel.MetaAlertLevel
}

// LayoutMetaColumns formats each active meta column and returns layouts plus total terminal width
// (including two-cell gaps between columns).
func LayoutMetaColumns(cols []MetaColumnState) (layouts []MetaColumnLayout, totalWidth int) {
	if len(cols) == 0 {
		return nil, 0
	}
	layouts = make([]MetaColumnLayout, len(cols))
	for i := range cols {
		col := &cols[i]
		c := col.layoutCache
		if col.ResultsGen == 0 || c == nil || c.gen != col.ResultsGen {
			w, formatted, rightAlign := layoutMetaCells(col.Results, col.Pending)
			c = &metaCellsCache{
				gen: col.ResultsGen, width: w, formatted: formatted, rightAlign: rightAlign,
				numeric: panel.MetaValuesNumeric(col.Results, col.Pending),
				levels:  metaCellLevels(col),
			}
			if col.ResultsGen != 0 {
				col.layoutCache = c
			}
		}
		w := c.width
		// The title sets a floor so the header is not clipped while cells are still narrow
		// (e.g. every row showing the pending icon).
		w = max(w, min(runewidth.StringWidth(col.ColumnTitle), panelListMetaMax))
		layouts[i] = MetaColumnLayout{
			EntryName:  col.EntryName,
			Title:      col.ColumnTitle,
			Width:      w,
			Formatted:  c.formatted, // shared with later draws: read-only
			RightAlign: c.rightAlign,
			Numeric:    c.numeric,
			Pending:    col.Pending,

			PendingHidden: col.PendingHidden,
			Levels:        c.levels,
		}
		if i > 0 {
			totalWidth += len(metaColumnSep)
		}
		totalWidth += w
	}
	return layouts, totalWidth
}

// metaCellLevels evaluates col's alert rules on its raw values; still-pending cells never alert.
func metaCellLevels(col *MetaColumnState) map[string]panel.MetaAlertLevel {
	if col.Alerts.Empty() {
		return nil
	}
	var levels map[string]panel.MetaAlertLevel
	for path, raw := range col.Results {
		if col.Pending != "" && raw == col.Pending {
			continue
		}
		if level := col.Alerts.Level(raw); level != panel.MetaAlertNone {
			if levels == nil {
				levels = make(map[string]panel.MetaAlertLevel)
			}
			levels[path] = level
		}
	}
	return levels
}

// MetaHeaderText returns the padded header segment for meta columns.
func MetaHeaderText(layouts []MetaColumnLayout) string {
	text, _ := metaHeader(layouts)
	return text
}

// metaHeader is MetaHeaderText plus the cell offset where each column's cell (its spilled sort
// arrow included) starts in it.
func metaHeader(layouts []MetaColumnLayout) (string, []int) {
	if len(layouts) == 0 {
		return "", nil
	}
	var b strings.Builder
	starts := make([]int, len(layouts))
	for i, lay := range layouts {
		sep := metaColumnSep
		cell := padMetaLineToWidth(lay.Title, lay.Width, lay.RightAlign)
		if lay.SortArrow != 0 {
			arrowed := string(lay.SortArrow) + lay.Title
			if runewidth.StringWidth(arrowed) <= lay.Width {
				cell = padMetaLineToWidth(arrowed, lay.Width, lay.RightAlign)
			} else {
				// No room inside the column: the arrow takes the last cell of the gap before it.
				// For the first column the header grows one cell; panelListHeader eats that
				// from the name→meta gap.
				sep = " "
				cell = string(lay.SortArrow) + cell
			}
		}
		if i > 0 {
			b.WriteString(sep)
		}
		starts[i] = runewidth.StringWidth(b.String())
		b.WriteString(cell)
	}
	return b.String(), starts
}

// CarouselMeta lays out active meta columns for the carousel center column.
func CarouselMeta(cols []MetaColumnState, styles theme.Theme) panelcarousel.Meta {
	layouts, totalWidth := LayoutMetaColumns(cols)
	if len(layouts) == 0 {
		return panelcarousel.Meta{}
	}
	return panelcarousel.Meta{
		Width:  totalWidth,
		Header: MetaHeaderText(layouts),
		Row: func(path string) string {
			return MetaRowText(layouts, path)
		},
		Alerts: func(path string) []panellist.MetaAlert {
			return MetaRowAlerts(layouts, path, styles)
		},
	}
}

// MetaRowText returns the padded meta segment for one file row. Each column's text is padded to
// lay.Width on the same side as its header (RightAlign), so a numeric column whose content is
// narrower than the minimum column width stays flush with its right-aligned title.
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
		if lay.Pending != "" && text == lay.Pending {
			if lay.PendingHidden {
				parts[i] = strings.Repeat(" ", lay.Width)
				continue
			}
			parts[i] = centerMetaLine(text, lay.Width)
			continue
		}
		parts[i] = padMetaLineToWidth(text, lay.Width, lay.RightAlign)
	}
	return strings.Join(parts, metaColumnSep)
}

// MetaRowAlerts returns the alerting columns of one file row as cell ranges in the MetaRowText
// segment; nil when none alerts.
func MetaRowAlerts(layouts []MetaColumnLayout, path string, styles theme.Theme) []panellist.MetaAlert {
	var alerts []panellist.MetaAlert
	start := 0
	for _, lay := range layouts {
		if level := lay.Levels[path]; level != panel.MetaAlertNone {
			alerts = append(alerts, panellist.MetaAlert{Start: start, End: start + lay.Width, Style: metaAlertStyle(level, styles)})
		}
		start += lay.Width + len(metaColumnSep)
	}
	return alerts
}

func metaAlertStyle(level panel.MetaAlertLevel, styles theme.Theme) tcell.Style {
	if level == panel.MetaAlertCritical {
		return styles.PanelMetaCritical
	}
	return styles.PanelMetaWarn
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
// Cells equal to pending (the in-flight marker) are kept verbatim (MetaRowText centers them) and do
// not vote on alignment, so a numeric column is right-aligned from the first frame instead of
// flipping once results arrive.
func layoutMetaCells(metaResults map[string]string, pending string) (metaColW int, formatted map[string]string, rightAlign bool) {
	formatted = make(map[string]string, len(metaResults))
	// ponytail: an empty column is treated as numeric (vacuously all-digit) so a numeric
	// column's header never jumps left in a directory with no results yet; a text column's
	// header flips left once content arrives. Upgrade path: an explicit align key in meta.toml.
	if len(metaResults) == 0 {
		return panelListMetaMinCells, formatted, true
	}

	parsed := make(map[string][]string, len(metaResults))
	nCols := 0

	for path, raw := range metaResults {
		if raw == "" || (pending != "" && raw == pending) {
			formatted[path] = raw
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
		return clampMetaColW(panelListMetaMinCells), formatted, true
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
			cells = append(cells, alignMetaCell(cell, colW[j], metaCellNumeric(cell)))
		}
		formatted[path] = strings.Join(cells, " ")
	}

	metaColW = panelListMetaMinCells
	for _, s := range formatted {
		if w := runewidth.StringWidth(s); w > metaColW {
			metaColW = w
		}
	}

	rightAlign = nCols == 1
	for _, row := range parsed {
		if !metaCellNumeric(row[0]) {
			rightAlign = false
			break
		}
	}

	return clampMetaColW(metaColW), formatted, rightAlign
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

// metaCellNumeric reports whether a cell reads as a number the way meta sorting does
// (panel.MetaNumber), so numeric and size cells right-align.
func metaCellNumeric(s string) bool {
	_, ok := panel.MetaNumber(s)
	return ok
}

// padMetaLineToWidth pads or truncates the full meta column string to w terminal cells.
// right pads on the left (right-aligning the text) instead of the default left-align.
func padMetaLineToWidth(s string, w int, right bool) string {
	if w <= 0 {
		return s
	}
	if runewidth.StringWidth(s) > w {
		s = truncateMetaDisplay(s, w)
	}
	if right {
		return runewidth.FillLeft(s, w)
	}
	return runewidth.FillRight(s, w)
}

// centerMetaLine centers s within w terminal cells.
func centerMetaLine(s string, w int) string {
	s = truncateMetaDisplay(s, w)
	left := (w - runewidth.StringWidth(s)) / 2
	return runewidth.FillRight(strings.Repeat(" ", left)+s, w)
}

func alignMetaCell(s string, colW int, rightDigits bool) string {
	if colW <= 0 {
		return ""
	}
	t := s
	if runewidth.StringWidth(s) > colW {
		t = truncateMetaDisplay(s, colW)
	}
	if rightDigits && metaCellNumeric(t) {
		return runewidth.FillLeft(t, colW)
	}
	return runewidth.FillRight(t, colW)
}
