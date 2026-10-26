package dlg

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// placeLabel positions a label row whose column starts at x (the option marker's cell) and ends
// before right. Text starts where the option marker's glyph does and keeps the one-cell right margin.
func placeLabel(it Item, x, right, y int) PlacedItem {
	x += 1 + it.Indent
	w := max(right-x-1, 0)
	var text string
	switch avail := max(w-runewidth.StringWidth(it.Prefix), 0); it.Fit {
	case FitMiddle:
		text = it.Prefix + primitive.TruncateMiddle(it.Label, avail)
	case FitPath:
		text = it.Prefix + primitive.FitPathForWidth(it.Label, avail)
	default:
		text = it.Prefix + primitive.TruncateRight(it.Label, avail)
	}
	if n := runewidth.StringWidth(text); n <= w {
		switch {
		case it.Right:
			x, w = x+w-n, n
		case it.Center:
			pad := (w - n) / 2
			x, w = x+pad, w-pad
		}
	}
	return PlacedItem{ID: it.ID, Kind: KindLabel, Rect: Rect{X: x, Y: y, Width: w, Height: 1}, Label: text}
}

// buttonRows splits the strip into its rows (Button.NewRow starts a new one).
func buttonRows(buttons []Button) [][]Button {
	var rows [][]Button
	for i, b := range buttons {
		if i == 0 || b.NewRow {
			rows = append(rows, nil)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], b)
	}
	return rows
}

func buttonLabels(row []Button) []string {
	labels := make([]string, len(row))
	for i, b := range row {
		labels[i] = b.Label
	}
	return labels
}

// Widgets returns the widgets of spec in layout order (group, column, item), with the cells of a
// row in place of the row. Place lays them out in this order and Draw walks them in it.
func Widgets(spec Spec) []Item { return leaves(spec) }

func leaves(spec Spec) []Item {
	var out []Item
	for _, g := range spec.Groups {
		for _, col := range g.Columns {
			for _, it := range col {
				if it.Kind == KindRow {
					out = append(out, it.Cells...)
					continue
				}
				out = append(out, it)
			}
		}
	}
	return out
}

// growing reports whether a row cell takes a share of the row's spare width.
func (it Item) growing() bool { return it.Width <= 0 && (it.Grow || it.Kind == KindInput) }

// boxWidth is the width of a row cell's box when it is not growing.
func (it Item) boxWidth(markerW int) int {
	switch {
	case it.Width > 0:
		return it.Width
	case it.Kind == KindLabel:
		return 1 + it.Indent + runewidth.StringWidth(it.Prefix+it.Label)
	case it.Kind == KindRadio || it.Kind == KindCheckbox:
		return markerW + runewidth.StringWidth(it.Label)
	}
	return 0
}

// placeRow lays the cells of a row out from x to one cell before right. Boxes are placed left to
// right; growing cells share what the others leave. A row too wide for its space first narrows its
// gaps (down to one cell), then a box that crosses the end is clipped.
func placeRow(row Item, x, right, y, markerW int) []PlacedItem {
	end := right - 1
	boxes := make([]int, len(row.Cells))
	gaps := make([]int, len(row.Cells))
	used, grows := 0, 0
	for i, c := range row.Cells {
		gaps[i] = c.Gap
		used += c.Gap
		if c.growing() {
			grows++
			continue
		}
		boxes[i] = c.boxWidth(markerW)
		used += boxes[i]
	}
	for narrowed := true; used > end-x && narrowed; {
		narrowed = false
		for i := range gaps {
			if gaps[i] > 1 && used > end-x {
				gaps[i]--
				used--
				narrowed = true
			}
		}
	}
	if grows > 0 {
		spare := max(end-x-used, 0)
		for i, c := range row.Cells {
			if c.growing() {
				boxes[i] = spare / grows
			}
		}
		for i := len(boxes) - 1; i >= 0; i-- { // the rounding remainder goes to the last growing cell
			if row.Cells[i].growing() {
				boxes[i] += spare - spare/grows*grows
				break
			}
		}
	}
	out := make([]PlacedItem, 0, len(row.Cells))
	bx := x
	for i, c := range row.Cells {
		boxEnd := min(bx+boxes[i], end)
		avail := max(boxEnd-bx, 0)
		var pi PlacedItem
		switch c.Kind {
		case KindLabel:
			pi = placeLabel(c, bx, boxEnd+1, y)
			if avail == 0 {
				pi.Rect.Width, pi.Label = 0, ""
			}
		case KindInput:
			pi = PlacedItem{ID: c.ID, Kind: KindInput, Rect: Rect{X: bx + 1, Y: y, Width: max(avail-1, 0), Height: 1}}
		case KindSpacer:
			pi = PlacedItem{ID: c.ID, Kind: KindSpacer, Rect: Rect{X: bx, Y: y, Width: avail, Height: 1}}
		default:
			pi = PlacedItem{ID: c.ID, Kind: c.Kind, Rect: Rect{X: bx, Y: y, Height: 1}}
			if avail >= markerW {
				pi.Label = primitive.TruncateRight(c.Label, avail-markerW)
				pi.Rect.Width = markerW + runewidth.StringWidth(pi.Label)
			}
		}
		pi.Disabled = c.Disabled
		out = append(out, pi)
		bx += boxes[i] + gaps[i]
	}
	return out
}

// shrinkToFit returns spec with its flexible items (lists, then spacers) shortened, down to one
// row each, until the groups and the button strip fit inside rect. Anything still too tall is cut
// off by Place/Draw above the button separator. spec itself is not modified.
func shrinkToFit(spec Spec, rect Rect) Spec {
	avail := rect.Height - 2
	if len(spec.Buttons) > 0 {
		avail -= 1 + len(buttonRows(spec.Buttons)) // separator + strip rows
	}
	need := 0
	for _, g := range spec.Groups {
		need += g.height()
	}
	if need <= avail {
		return spec
	}
	groups := make([]Group, len(spec.Groups))
	for gi, g := range spec.Groups {
		cols := make([][]Item, len(g.Columns))
		for ci, col := range g.Columns {
			cols[ci] = append([]Item(nil), col...)
		}
		groups[gi] = Group{Columns: cols, ColWidths: g.ColWidths}
	}
	spec.Groups = groups
	for _, kind := range []Kind{KindList, KindSpacer} {
		for need > avail {
			shrunk := false
			for gi := range spec.Groups {
				for ci := range spec.Groups[gi].Columns {
					col := spec.Groups[gi].Columns[ci]
					for i := range col {
						if col[i].Kind == kind && col[i].Height > 1 && need > avail {
							before := spec.Groups[gi].height()
							col[i].Height--
							need -= before - spec.Groups[gi].height()
							shrunk = true
						}
					}
				}
			}
			if !shrunk {
				break
			}
		}
	}
	return spec
}

func (g Group) height() int {
	h := 0
	for _, col := range g.Columns {
		n := 0
		for _, it := range col {
			n += it.span()
		}
		h = max(h, n)
	}
	return h
}

// columnX returns where column ci of g starts (the option marker's cell) and the x its cell ends
// before (the next column's start, or the right border for the last column).
func (g Group) columnX(rect Rect, ci int) (x, right int) {
	base := draw.DialogOptionX(rect)
	right = rect.X + rect.Width - 1 // stop one cell before the right border
	if len(g.ColWidths) > 0 {
		x = base
		for i := 0; i < ci && i < len(g.ColWidths); i++ {
			x += g.ColWidths[i]
		}
		if ci < len(g.ColWidths) {
			right = x + g.ColWidths[ci]
		}
		return x, right
	}
	colW := rect.Width / max(len(g.Columns), 1)
	x = base + ci*colW
	if ci+1 < len(g.Columns) {
		right = base + (ci+1)*colW
	}
	return x, right
}

// Place is the one layout pass: it clamps and centers the dialog on a screen of screenW x screenH
// and positions every widget. ok is false when the screen is too small to show the dialog.
func Place(screenW, screenH int, spec Spec) (Placement, bool) {
	prefH := 2
	for _, g := range spec.Groups {
		prefH += g.height()
	}
	rows := buttonRows(spec.Buttons)
	if len(rows) > 0 {
		prefH += 1 + len(rows) // separator + button rows
	}
	prefH = max(prefH, spec.MinHeight)
	prefW := spec.Width
	for _, row := range rows {
		prefW = max(prefW, draw.ButtonRowWidth(buttonLabels(row))+2) // each row must fit between the borders
	}
	layout := draw.Layout{Width: screenW, Height: screenH}
	var rect Rect
	if spec.Unclamped {
		w := min(prefW, screenW-4)
		if w < spec.MinWidth {
			w = min(spec.MinWidth, screenW-2)
		}
		rect = draw.CenteredDialogRect(layout, w, prefH)
	} else {
		var ok bool
		if rect, ok = draw.ClampCenteredDialogRect(layout, prefW, prefH, spec.MinWidth, spec.MinHeight); !ok {
			return Placement{}, false
		}
	}
	p := Placement{Rect: rect}
	spec = shrinkToFit(spec, rect)
	markerW := draw.DialogOptionWidth("")
	if spec.MarkerWidth > 0 {
		markerW = spec.MarkerWidth
	}
	y := rect.Y + 1
	for _, g := range spec.Groups {
		for ci, col := range g.Columns {
			x, right := g.columnX(rect, ci)
			ry := 0 // rows used so far in this column
			for _, it := range col {
				ri := ry
				ry += it.span()
				switch it.Kind {
				case KindRow:
					p.Items = append(p.Items, placeRow(it, x, right, y+ri, markerW)...)
					continue
				case KindLabel:
					p.Items = append(p.Items, placeLabel(it, x, right, y+ri))
					continue
				case KindBar:
					bar := it
					bar.Fit, bar.Center, bar.Right = FitPath, false, false
					pi := placeLabel(bar, x, right, y+ri)
					pi.Kind = KindBar
					p.Items = append(p.Items, pi)
					continue
				case KindSpacer:
					p.Items = append(p.Items, PlacedItem{
						ID: it.ID, Kind: KindSpacer, Rect: Rect{X: x, Y: y + ri, Width: max(right-x, 0), Height: it.span()},
					})
					continue
				case KindList:
					p.Items = append(p.Items, PlacedItem{
						ID: it.ID, Kind: KindList,
						Rect:   Rect{X: x + 1, Y: y + ri, Width: max(right-x-2, 0), Height: it.span()},
						Offset: it.ViewOffset(), Rows: it.rowCount(),
					})
					continue
				case KindSeparator:
					label := ""
					if it.Label != "" {
						label = " " + primitive.TruncateRight(it.Label, max(rect.Width-4, 0)) + " "
					}
					p.Items = append(p.Items, PlacedItem{
						ID: it.ID, Kind: KindSeparator,
						Rect: Rect{X: rect.X, Y: y + ri, Width: rect.Width, Height: 1}, Label: label,
					})
					continue
				case KindInput:
					p.Items = append(p.Items, PlacedItem{
						ID: it.ID, Kind: KindInput,
						Rect: Rect{X: draw.DialogTextX(rect), Y: y + ri, Width: draw.DialogContentWidth(rect), Height: 1},
					})
					continue
				}
				// The marker counts against the column too, or a long label runs past the border.
				label := primitive.TruncateRight(it.Label, max(right-x-draw.DialogOptionWidth(""), 0))
				p.Items = append(p.Items, PlacedItem{
					ID:       it.ID,
					Kind:     it.Kind,
					Rect:     Rect{X: x, Y: y + ri, Width: draw.DialogOptionWidth(label), Height: 1},
					Label:    label,
					Disabled: it.Disabled,
				})
			}
		}
		y += g.height()
	}
	p.ContentBottom = rect.Y + rect.Height - 1
	if len(spec.Buttons) > 0 {
		// The button strip always sits at the bottom of the frame; content that did not fit
		// (nothing flexible left to shrink) is cut off above the separator.
		y = min(y, rect.Y+rect.Height-2-len(rows))
		p.ContentBottom = y
		p.SepY, p.ButtonY = y, y+1
		for ri, row := range rows {
			for i, x := range draw.ButtonRowXs(rect, buttonLabels(row)) {
				b := row[i]
				p.Items = append(p.Items, PlacedItem{ID: b.ID, Rect: Rect{X: x, Y: p.ButtonY + ri, Width: draw.DialogButtonWidth(b.Label), Height: 1}, Label: b.Label})
			}
		}
	}
	return p, true
}

// MarkerWidth is the width of a radio or checkbox marker in the theme's icons, for Spec.MarkerWidth.
func MarkerWidth(styles theme.Theme) int {
	return max(
		utf8.RuneCountInString(" "+styles.IconDialogRadio(false)+" "),
		utf8.RuneCountInString(" "+styles.IconDialogCheckbox(false)+" "),
	)
}

// Draw paints spec with focus on the widget named by focus (an empty or unknown ID focuses the
// first widget). It returns the placement it drew, for callers that paint overlays on its rects;
// ok is false when the dialog does not fit and nothing was drawn.
func Draw(screen tcell.Screen, screenW, screenH int, spec Spec, focus ID, styles theme.Theme) (p Placement, ok bool) {
	p, ok = Place(screenW, screenH, spec)
	if !ok {
		return p, false
	}
	focus = Focused(spec, focus)
	borderStyle := draw.DrawDialogFrame(screen, p.Rect, spec.Title, styles)
	_, surface, _ := styles.DialogSurface.Decompose()
	bottom := p.ContentBottom         // content stops above the button separator (or the bottom border)
	for n, it := range leaves(spec) { // p.Items holds the group items in the same order
		pi := p.Items[n]
		if pi.Rect.Y >= bottom {
			continue
		}
		if it.Kind == KindList && pi.Rect.Y+pi.Rect.Height > bottom {
			pi.Rect.Height = bottom - pi.Rect.Y
		}
		switch it.Kind {
		case KindLabel:
			style := styles.DialogText.Background(surface)
			switch it.Style {
			case TextError:
				errFG, _, _ := styles.DialogInputActiveError.Decompose()
				style = style.Foreground(errFG)
			case TextMessageError:
				style = styles.MessageError.Background(surface)
			case TextWarn:
				style = styles.MessageWarn.Background(surface)
			}
			primitive.Text(screen, pi.Rect.X, pi.Rect.Y, pi.Rect.Width, pi.Label, style)
		case KindBar:
			drawBar(screen, pi.Rect, pi.Label, it.Frac, styles)
		case KindSeparator:
			if pi.Label == "" {
				draw.DrawDialogHSeparator(screen, pi.Rect, pi.Rect.Y, borderStyle)
				break
			}
			frameFG, _, _ := styles.DialogFrame.Decompose()
			labelStyle := styles.DialogTitle.Foreground(frameFG).Background(surface)
			draw.DrawDialogHSeparatorWithCenteredLabel(screen, pi.Rect, pi.Rect.Y, borderStyle, labelStyle, pi.Label)
		case KindInput:
			if pi.Rect.Width > 0 {
				drawInput(screen, pi.Rect, it, focus == it.ID, styles)
			}
		case KindSpacer:
			// reserved for the caller's overlay
		case KindList:
			drawList(screen, p.Rect, pi, it, listFocused(spec, focus, it.ID), spec.ScrollbarStyle, borderStyle, styles)
		case KindCheckbox:
			if pi.Rect.Width > 0 {
				draw.DrawDialogCheckbox(screen, pi.Rect.X, pi.Rect.Y, pi.Label, it.Mnemonic, it.On, focus == it.ID, it.Disabled || it.Inert, styles)
			}
		default:
			if pi.Rect.Width > 0 {
				draw.DrawDialogRadio(screen, pi.Rect.X, pi.Rect.Y, pi.Label, it.Mnemonic, it.On, focus == it.ID, it.Disabled || it.Inert, styles)
			}
		}
	}
	if len(spec.Buttons) == 0 {
		return p, true
	}
	if spec.ButtonsLabel == "" {
		draw.DrawDialogHSeparator(screen, p.Rect, p.SepY, borderStyle)
	} else {
		label := " " + primitive.TruncateRight(spec.ButtonsLabel, max(p.Rect.Width-4, 0)) + " "
		labelStyle := styles.DialogStatusSelectionSizeStyle()
		draw.DrawDialogHSeparatorWithCenteredLabel(screen, p.Rect, p.SepY, borderStyle, labelStyle, label)
	}
	for ri, row := range buttonRows(spec.Buttons) {
		specs := make([]draw.DialogButtonSpec, len(row))
		for i, b := range row {
			specs[i] = draw.DialogButtonSpec{Label: b.Label, Shortcut: b.Mnemonic, Focused: focus == b.ID, Destructive: b.Destructive}
		}
		draw.DrawDialogButtonRowCentered(screen, p.Rect, p.ButtonY+ri, specs, styles)
	}
	return p, true
}

func drawInput(screen tcell.Screen, r Rect, it Item, focused bool, styles theme.Theme) {
	if it.Simple {
		draw.DrawSimpleDialogInput(screen, r.X, r.Y, r.Width, it.Value, it.Cursor, focused, it.Invalid, styles)
		return
	}
	draw.PaintScrollingInputContent(screen, r.X, r.Y, r.Width, it.Value, it.CompletionSuffix, it.Cursor, it.Scroll,
		focused, it.Invalid, focused, it.Placeholder, it.LeadingIcon, styles)
}

// HitTest returns the widget under screen cell (x, y).
func HitTest(p Placement, x, y int) (ID, bool) {
	for _, it := range p.Items {
		r := it.Rect
		if it.Kind == KindLabel || it.Kind == KindSeparator || it.Kind == KindSpacer || it.Kind == KindBar || it.Disabled {
			continue
		}
		if r.Contains(x, y) {
			if it.Kind == KindList && it.Offset+(y-r.Y) >= it.Rows {
				continue // below the last row
			}
			return it.ID, true
		}
	}
	return "", false
}

// listFocused reports whether list id shows its cursor as focused: the list itself or an input
// that drives it holds focus.
func listFocused(spec Spec, focus, id ID) bool {
	if focus == id {
		return true
	}
	if p, ok := find(spec, focus); ok && !p.isButton() {
		it := widgetAt(spec, p)
		return it.Kind == KindInput && it.Drives == id
	}
	return false
}

// drawList paints the visible rows of a list (pi is its placement) and its scrollbar.
func drawList(screen tcell.Screen, dialogRect Rect, pi PlacedItem, it Item, focused bool, sbStyle uiscrollbar.Style, borderStyle tcell.Style, styles theme.Theme) {
	r := pi.Rect
	_, surface, _ := styles.DialogSurface.Decompose()
	for row := 0; row < r.Height; row++ {
		y := r.Y + row
		idx := pi.Offset + row
		if idx >= it.rowCount() {
			if !it.Markers { // option-row lists leave unused rows as bare surface
				w := r.Width
				if it.MarksWidth > 0 && !it.MarksAfterText { // keep the reserved marks column painted
					w = max(r.Width-it.MarksWidth, 1)
				}
				primitive.StyledText(screen, r.X, y, w, "", styles.DialogText.Background(surface), nil)
				if w < r.Width {
					DrawRowMarksSuffix(screen, r.X+w, y, it.MarksWidth, RowMarks{}, surface, styles)
				}
			}
			continue
		}
		rw := it.row(idx)
		isCursor := focused && idx == it.Cursor
		if it.Markers {
			style := styles.DialogOptionRowStyle(isCursor, rw.Marked)
			part := " " + styles.IconDialogRadio(rw.Marked) + " "
			partW := runewidth.StringWidth(part)
			primitive.Text(screen, r.X, y, partW, part, style)
			primitive.StyledText(screen, r.X+partW, y, max(r.Width-partW, 0), rw.Text, style, nil)
			continue
		}
		drawPlainRow(screen, r.X, y, r.Width, rw, it, isCursor, styles)
	}
	if it.Scrollbar {
		if sbStyle == "" {
			sbStyle = uiscrollbar.StyleThumb
		}
		draw.DrawDialogListScrollbar(screen, dialogRect, r.Y, r.Height, it.rowCount(), pi.Offset, sbStyle, borderStyle, styles)
	}
}

// drawPlainRow paints one row of a list without Markers: the optional icon, the fitted text with
// its fuzzy highlights over its own spans and the trailing marks, in the header, cursor, invalid
// or plain row style.
func drawPlainRow(screen tcell.Screen, x, y, w int, rw ListRow, it Item, isCursor bool, styles theme.Theme) {
	_, surface, _ := styles.DialogSurface.Decompose()
	base := styles.DialogText.Background(surface)
	match := styles.FuzzyHighlight
	switch {
	case rw.Header:
		base = styles.DialogHelpSection.Background(surface)
	case isCursor:
		base = styles.DialogOptionRowStyle(true, rw.Marked)
		match = styles.FuzzyHighlightCursor
	case rw.Missing:
		base = styles.DialogOptionInvalidStyle()
	case rw.Marked:
		base = styles.DialogOptionRowStyle(false, true)
	}
	_, bg, _ := base.Decompose()
	match = match.Background(bg)
	marks := rw.Marks
	marks.Missing = marks.Missing || rw.Missing
	marks.Cursor = isCursor
	marksW := RowMarksWidth(marks)
	if it.MarksWidth > 0 && !it.MarksAfterText {
		marksW = it.MarksWidth
	}
	area, markX := w, 0
	if !it.MarksAfterText {
		area = max(w-marksW, 1)
		markX = x + area
	}
	tx, tw := x, area
	if rw.Icon != "" {
		iw := min(runewidth.RuneWidth([]rune(rw.Icon)[0])+1, tw)
		iconStyle := base
		if it.IconOptionFG {
			fg, _, _ := styles.DialogOptionRowStyle(isCursor, false).Decompose()
			iconStyle = base.Foreground(fg)
		}
		primitive.Text(screen, tx, y, iw, rw.Icon+" ", iconStyle)
		tx, tw = tx+iw, tw-iw
	}
	if tw > 0 {
		fit := tw
		if it.MarksAfterText {
			fit = max(tw-marksW, 1)
		}
		text, spans := fitRow(rw, it.Fit, fit, match)
		if !isCursor && (it.Fit == FitNone || it.Fit == FitRight) {
			spans = append(spans, rw.Spans...) // the first span covering a rune wins: Match on top
		}
		primitive.StyledText(screen, tx, y, tw, text, base, spans)
		if it.MarksAfterText {
			markX = tx + min(runewidth.StringWidth(text), fit)
		}
	}
	if marksW > 0 {
		DrawRowMarksSuffix(screen, markX, y, marksW, marks, bg, styles)
	}
}

// fitRow shortens a row's text to width per fit and maps its match ranges onto the result.
func fitRow(rw ListRow, fit Fit, width int, match tcell.Style) (string, []primitive.Span) {
	if fit == FitNone {
		spans := make([]primitive.Span, 0, len(rw.Match))
		for _, r := range rw.Match {
			spans = append(spans, primitive.Span{Start: r.Start, End: r.End, Style: match})
		}
		return rw.Text, spans
	}
	if fit != FitRight {
		return draw.FuzzyRowContent(rw.Text, rw.Match, width, match, fit == FitPath)
	}
	text := primitive.TruncateRight(rw.Text, width)
	return text, draw.FuzzyHighlightSpans([]rune(rw.Text), []rune(text), rw.Match, match)
}
