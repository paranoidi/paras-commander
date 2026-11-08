package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
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
	// Pending is the in-flight marker (MetaColumnState.Pending); rows showing it are right-aligned.
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
	measured   bool // at least one cell held a value, so width and rightAlign reflect real data
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
		if col.LayoutGen == 0 || c == nil || c.gen != col.LayoutGen {
			w, formatted, rightAlign := layoutMetaCells(col.Results, col.Pending)
			c = &metaCellsCache{
				gen: col.LayoutGen, width: w, formatted: formatted, rightAlign: rightAlign,
				numeric: panel.MetaValuesNumeric(col.Results, col.Pending),
				levels:  metaCellLevels(col), measured: metaColumnHasValues(col),
			}
			if col.LayoutGen != 0 {
				col.layoutCache = c
			}
		}
		w := c.width
		// The title sets a floor so the header is not clipped while cells are still narrow
		// (e.g. every row showing the pending icon).
		w = max(w, min(runewidth.StringWidth(col.ColumnTitle), panelListMetaMax), col.Guess.Width)
		rightAlign := c.rightAlign
		if !c.measured && col.Guess.Width > 0 {
			rightAlign = col.Guess.RightAlign
		}
		layouts[i] = MetaColumnLayout{
			EntryName:  col.EntryName,
			Title:      col.ColumnTitle,
			Width:      w,
			Formatted:  c.formatted, // shared with later draws: read-only
			RightAlign: rightAlign,
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

// MeasuredMetaLayout is the layout col's values last came to, without the title or col.Guess; ok
// is false when no layout held a value (an empty listing, or one the column's command skips, says
// nothing about the next one).
func MeasuredMetaLayout(col *MetaColumnState) (guess MetaLayoutGuess, ok bool) {
	if c := col.layoutCache; c != nil && c.measured {
		return MetaLayoutGuess{Width: c.width, RightAlign: c.rightAlign}, true
	}
	return MetaLayoutGuess{}, false
}

// metaColumnHasValues reports whether any cell of col holds a real value.
func metaColumnHasValues(col *MetaColumnState) bool {
	for _, raw := range col.Results {
		if raw != col.Pending && strings.TrimSpace(raw) != "" {
			return true
		}
	}
	return false
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

// dropLastMetaColumn removes the rightmost layout and returns the remaining total width.
func dropLastMetaColumn(layouts []MetaColumnLayout, totalWidth int) ([]MetaColumnLayout, int) {
	last := len(layouts) - 1
	totalWidth -= layouts[last].Width
	if last > 0 {
		totalWidth -= len(metaColumnSep)
	}
	return layouts[:last], totalWidth
}

// FitCarouselMetaColumns lays out the meta columns the carousel center column has room for: the
// rightmost go first until its name column is at least config.MinPanelNameWidthWithMeta wide, or
// as wide as without any meta when a fit-mode column hugging short names is narrower than that.
// fitWidths comes from panelcarousel.MeasureFitColumnWidths; widening it for each candidate is
// arithmetic, so the listing is not scanned again.
func FitCarouselMetaColumns(frame Rect, layout panelcarousel.Layout, center panel.State, showIcons, showChild bool, style uiscrollbar.Style, visibleRows int, fitWidths [3]int, cols []MetaColumnState) ([]MetaColumnLayout, int) {
	nameWidth := func(measured [3]int, metaW int) int {
		return panelcarousel.CenterNameWidth(frame, layout, center, showIcons, showChild, style, visibleRows, measured, metaW)
	}
	floor := min(config.MinPanelNameWidthWithMeta, nameWidth(fitWidths, 0))
	layouts, totalWidth := LayoutMetaColumns(cols)
	for len(layouts) > 0 && nameWidth(panelcarousel.WithCenterMeta(fitWidths, totalWidth), totalWidth) < floor {
		layouts, totalWidth = dropLastMetaColumn(layouts, totalWidth)
	}
	return layouts, totalWidth
}

// PanelMetaFit describes the meta columns panelID draws in its file list frame.
type PanelMetaFit struct {
	// Width and NameRoom (the name column width with no meta at all) identify the available
	// space: when they change, a different Drawn is not down to the column contents.
	Width, NameRoom int
	Drawn           []MetaColumnLayout
}

// PanelMetaFitAt is the fit drawPanel applies to panelID in the frame file. hideInactive is the
// effective Model.HideInactivePanel of a render snapshot.
func PanelMetaFitAt(file Rect, model *Model, panelID int, hideInactive bool) PanelMetaFit {
	state := model.PanelForFileListRender(panelID)
	cols := model.MetaResults[panelID]
	fit := PanelMetaFit{Width: file.Width}
	if state.CarouselMode {
		rows := PanelListRows(file)
		plan, ok := newCarouselPanelPlan(file, state, model.QuickViewDriverPanelID() >= 0, hideInactive, model.CarouselLayout, model.UseNerdfontIcons, model.PanelScrollbar, rows)
		if ok {
			fit.NameRoom = panelcarousel.CenterNameWidth(file, model.CarouselLayout, state, model.UseNerdfontIcons, plan.ShowChild, model.PanelScrollbar, rows, plan.FitWidths, 0)
			fit.Drawn, _ = FitCarouselMetaColumns(file, model.CarouselLayout, state, model.UseNerdfontIcons, plan.ShowChild, model.PanelScrollbar, rows, plan.FitWidths, cols)
			return fit
		}
	}
	layoutRes := panelColumnLayout(file, state, PanelDisplayConfig{ShowIcons: model.UseNerdfontIcons, NarrowPanelsNameOnly: model.NarrowPanelsNameOnly})
	fit.NameRoom = panelNameColumnWidth(layoutRes, panel.EffectiveListFormat(state.ListFormat), false, 0)
	fit.Drawn, _ = fitPanelMetaColumns(layoutRes, state, cols)
	return fit
}

// CarouselMeta builds the carousel center column's meta segment from FitCarouselMetaColumns.
func CarouselMeta(layouts []MetaColumnLayout, totalWidth int, styles theme.Theme) panelcarousel.Meta {
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
			parts[i] = padMetaLineToWidth(text, lay.Width, true)
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
// Cells equal to pending (the in-flight marker) are kept verbatim (MetaRowText right-aligns them) and do
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
