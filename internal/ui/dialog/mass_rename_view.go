package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// Widget IDs of the mass rename main screen. The Find / Pattern and Replace inputs are
// FileFieldID(0) and FileFieldID(1); the buttons are dlg.OK, MassRenameApply and dlg.Cancel.
const (
	MassRenameRadioSimple     = dlg.ID("mode-simple")
	MassRenameRadioRegex      = dlg.ID("mode-regex")
	MassRenameRadioEditor     = dlg.ID("mode-editor")
	MassRenameRadioCapitalize = dlg.ID("mode-capitalize")
	MassRenameOptShowModified = dlg.ID("opt-show-modified")
	MassRenameOptTrim         = dlg.ID("opt-trim")
	MassRenameOptCase         = dlg.ID("opt-case")
	MassRenameOptIgnoreExt    = dlg.ID("opt-ignore-ext")
	MassRenameOptCapEachWord  = dlg.ID("opt-cap-each-word")
	MassRenameOptCapPunct     = dlg.ID("opt-cap-punct")
	MassRenameApply           = dlg.ID("apply")
	massRenamePreview         = dlg.ID("mass-rename-preview")
)

// MassRenameRadioID is the radio ID of a mode.
func MassRenameRadioID(m MassRenameModeUI) dlg.ID {
	switch m {
	case MassRenameModeUIRegex:
		return MassRenameRadioRegex
	case MassRenameModeUIExternalEditor:
		return MassRenameRadioEditor
	case MassRenameModeUICapitalize:
		return MassRenameRadioCapitalize
	default:
		return MassRenameRadioSimple
	}
}

// MassRenameModeFromRadio returns the mode of a MassRenameRadioID.
func MassRenameModeFromRadio(id dlg.ID) (MassRenameModeUI, bool) {
	for _, m := range []MassRenameModeUI{MassRenameModeUISimple, MassRenameModeUIRegex, MassRenameModeUIExternalEditor, MassRenameModeUICapitalize} {
		if MassRenameRadioID(m) == id {
			return m, true
		}
	}
	return 0, false
}

// massRenameColumnWidths returns each column's box width for widgets whose marker is markerW
// cells: as wide as the widest radio or checkbox of the column (column 2 also fits Ignore
// extension for Capitalize), independent of mode so switching never resizes. Columns are
// separated by a 2-cell gap (massRenameColumnGap).
func massRenameColumnWidths(markerW int) [4]int {
	w := func(labels ...string) int {
		m := 0
		for _, l := range labels {
			m = max(m, markerW+utf8.RuneCountInString(l))
		}
		return m
	}
	return [4]int{
		w("Simple (replace text)", "Show only modified"),
		w("Regular expression", "Trim whitespace"),
		w("External $EDITOR", "Case insensitive", "Ignore extension"),
		w("Capitalize", "Ignore extension"),
	}
}

const massRenameColumnGap = 2

// massRenameColumns returns each column's x offset from the first radio/checkbox and the total
// row width, for widgets whose marker (leading margin space + icon + space, e.g. " ( ) ") is
// markerW cells. The margin space of the next column's marker plus the gap leaves 3 blank cells
// between adjacent items.
func massRenameColumns(markerW int) (off [4]int, total int) {
	for i, cw := range massRenameColumnWidths(markerW) {
		off[i] = total
		total += cw + massRenameColumnGap
	}
	return off, total - massRenameColumnGap
}

// massRenameMarkerWidth is the marker width the view lays its columns out with.
func massRenameMarkerWidth(st FileDialogState) int {
	if st.MassRenameMarkerWidth > 0 {
		return st.MassRenameMarkerWidth
	}
	return utf8.RuneCountInString(draw.RadioText("", false)) // ASCII markers, the widest
}

// massRenameView is the widget spec of the mass rename main screen: the mode radios and the option
// checkboxes sharing four columns, a rule, the mode's section (Find / Replace inputs with their
// hints, the Capitalize checkboxes, or nothing for the external editor), a rule and the
// before / after preview (massRenamePreviewList, a ScrollOnly list), above OK / Apply /
// Cancel. The mode radios are Tab stops of one segment with the option row; each input is a
// segment of its own. The dialog is massRenameDialogHeight tall and the preview fills what the
// rows above it leave.
func massRenameView(st FileDialogState, ctx FileDialogViewContext) dlg.Spec {
	// The rows are pre-fitted into a before and an after column. The preview wants the whole
	// screen and gets what massRenameDialogHeight leaves it.
	spec := massRenameHead(st, ctx)
	spec.Groups = append(spec.Groups, massRenamePreviewGroup(ctx.Layout.Height))
	spec.MaxHeight = massRenameDialogHeight(st, ctx)
	spec.ScrollbarStyle = ctx.ScrollbarStyle
	spec, _ = fillList(spec, ctx.Layout, massRenamePreview, func(height, width int) dlg.Item {
		return massRenamePreviewList(st, height, width, ctx.Styles)
	})
	return spec
}

// massRenameDialogHeight is the mass rename dialog's outer height: what the Simple mode (Regex,
// with its hint row, in Regex mode) needs to show every preview row, at most the screen less a row
// above and below and at least 12 rows. Every mode gets this height, so switching modes does not
// resize the dialog: the modes with fewer rows above the preview give them to it.
func massRenameDialogHeight(st FileDialogState, ctx FileDialogViewContext) int {
	if st.MassRenameMode != MassRenameModeUIRegex {
		st.MassRenameMode = MassRenameModeUISimple
	}
	spec := massRenameHead(st, ctx)
	spec.Groups = append(spec.Groups, massRenamePreviewGroup(max(len(st.MassRenamePreviewBefore), 1)))
	spec.MaxHeight = ctx.Layout.Height - 2
	p, _ := dlg.Place(ctx.Layout.Width, ctx.Layout.Height, spec)
	return max(p.Rect.Height, 12)
}

// massRenamePreviewGroup is the group of an empty preview list height rows tall, for sizing.
func massRenamePreviewGroup(height int) dlg.Group {
	return dlg.Group{Columns: [][]dlg.Item{{{ID: massRenamePreview, Kind: dlg.KindList, Height: height}}}}
}

// massRenameHead is the mass rename spec without the preview.
func massRenameHead(st FileDialogState, ctx FileDialogViewContext) dlg.Spec {
	markerW := massRenameMarkerWidth(st)
	widths := massRenameColumnWidths(markerW)
	spec := dlg.Spec{
		Title: fileDialogOuterTitle(st), Width: fileDialogWidth(ctx.Layout.Width, st, ctx.IconLead), Unclamped: true, MarkerWidth: markerW,
		Buttons: []dlg.Button{
			dlg.OKButton,
			{ID: MassRenameApply, Label: "Apply", Mnemonic: 'A'},
			dlg.CancelButton,
		},
	}
	cell := func(id dlg.ID, kind dlg.Kind, label string, mnemonic rune, on bool, col int) dlg.Item {
		return dlg.Item{ID: id, Kind: kind, Label: label, Mnemonic: mnemonic, On: on, Width: widths[col], Gap: massRenameColumnGap}
	}
	radios := []dlg.Item{
		cell(MassRenameRadioSimple, dlg.KindRadio, "Simple (replace text)", 'S', st.MassRenameMode == MassRenameModeUISimple, 0),
		cell(MassRenameRadioRegex, dlg.KindRadio, "Regular expression", 'R', st.MassRenameMode == MassRenameModeUIRegex, 1),
		cell(MassRenameRadioEditor, dlg.KindRadio, "External $EDITOR", 'E', st.MassRenameMode == MassRenameModeUIExternalEditor, 2),
		cell(MassRenameRadioCapitalize, dlg.KindRadio, "Capitalize", 'z', st.MassRenameMode == MassRenameModeUICapitalize, 3),
	}
	options := []dlg.Item{
		cell(MassRenameOptShowModified, dlg.KindCheckbox, "Show only modified", 'm', st.MassRenameShowOnlyModified, 0),
		cell(MassRenameOptTrim, dlg.KindCheckbox, "Trim whitespace", 't', st.MassRenameStripSpaces, 1),
	}
	switch st.MassRenameMode {
	case MassRenameModeUICapitalize:
		options = append(options, cell(MassRenameOptIgnoreExt, dlg.KindCheckbox, "Ignore extension", 'x', st.MassRenameIgnoreExt, 2))
	case MassRenameModeUIExternalEditor:
	default:
		options = append(options,
			cell(MassRenameOptCase, dlg.KindCheckbox, "Case insensitive", 'i', st.MassRenameCaseFold, 2),
			cell(MassRenameOptIgnoreExt, dlg.KindCheckbox, "Ignore extension", 'x', st.MassRenameIgnoreExt, 3))
	}
	// The last cell of a row has no gap after it (a trailing gap would narrow the others).
	radios[len(radios)-1].Gap = 0
	options[len(options)-1].Gap = 0
	modes := []dlg.Item{
		{Kind: dlg.KindRow, Cells: radios, TabToSelected: true},
		{Kind: dlg.KindRow, Cells: options},
		{Kind: dlg.KindSeparator},
	}
	spec.Groups = append(spec.Groups, dlg.Group{Columns: [][]dlg.Item{modes}})

	switch st.MassRenameMode {
	case MassRenameModeUIExternalEditor:
	case MassRenameModeUICapitalize:
		spec.Groups = append(spec.Groups, dlg.Group{Columns: [][]dlg.Item{{
			{ID: MassRenameOptCapEachWord, Kind: dlg.KindCheckbox, Label: "Capitalize each word", Mnemonic: 'w', On: st.MassRenameCapEachWord},
			{ID: MassRenameOptCapPunct, Kind: dlg.KindCheckbox, Label: "Treat punctuation as separators", Mnemonic: 'p', On: st.MassRenameCapPunctSep},
			{Kind: dlg.KindSeparator},
		}}})
	default:
		var fields []dlg.Item
		for i := 0; i < 2 && i < len(st.Fields); i++ {
			f := st.Fields[i]
			label := f.Label + ":"
			if i == 0 {
				label = massRenamePatternLabelText(st)
			}
			labelItem := dlg.Item{Kind: dlg.KindLabel, Label: label}
			if hint := massRenamePatternHintText(st); i == 0 && hint != "" && utf8.RuneCountInString(label)+1+utf8.RuneCountInString(hint) <= spec.Width-4 {
				// The compile hint shares the label row, right-aligned.
				fields = append(fields, dlg.Item{Kind: dlg.KindRow, Cells: []dlg.Item{
					labelItem,
					{Kind: dlg.KindLabel, Label: hint, Style: dlg.TextError, Grow: true, Right: true},
				}})
			} else {
				fields = append(fields, labelItem)
			}
			fields = append(fields, fileFieldInput(FileFieldID(i), f, f.InputInvalid))
			if i == 1 && st.MassRenameMode == MassRenameModeUIRegex {
				if hint := massRenameReplacementHintText(st); hint != "" {
					fields = append(fields, dlg.Item{Kind: dlg.KindLabel, Label: ctx.Styles.IconDialogInfo() + " " + hint})
				}
			}
		}
		spec.Groups = append(spec.Groups, dlg.Group{Columns: [][]dlg.Item{append(fields, dlg.Item{Kind: dlg.KindSeparator})}})
	}
	return spec
}

// massRenamePreviewList is the before / after preview: a ScrollOnly list of rows pre-fitted to
// width cells (the compute error alone when there is one).
func massRenamePreviewList(st FileDialogState, height, width int, styles theme.Theme) dlg.Item {
	_, bg, _ := styles.DialogSurface.Decompose()
	if errMsg := strings.TrimSpace(st.MassRenameComputeError); errMsg != "" {
		return scrollList(massRenamePreview, 1, height, 0, func(int) dlg.ListRow {
			text := primitive.TruncateRight(errMsg, width)
			text += strings.Repeat(" ", max(width-runewidth.StringWidth(text), 0))
			return dlg.ListRow{Text: text, Spans: []primitive.Span{{End: utf8.RuneCountInString(text), Style: styles.MessageWarn.Background(bg)}}}
		})
	}
	n := min(len(st.MassRenamePreviewBefore), len(st.MassRenamePreviewAfter))
	primaryW, sepW, secondaryW := massRenamePreviewColumns(width)
	list := scrollList(massRenamePreview, n, height, st.MassRenamePreviewScroll, func(i int) dlg.ListRow {
		return massRenamePreviewRow(st, i, primaryW, sepW, secondaryW, bg, styles)
	})
	list.Fit = dlg.FitNone
	return list
}

// massRenamePreviewColumns splits the preview width into the before column, a one-cell separator
// (when there is room) and the after column.
func massRenamePreviewColumns(width int) (primaryW, sepW, secondaryW int) {
	primaryW = width / 2
	secondaryW = width - primaryW
	if width >= 3 {
		sepW = 1
		primaryW = (width - sepW) / 2
		secondaryW = width - sepW - primaryW
	}
	if primaryW < 1 {
		primaryW = 1
		secondaryW = max(width-sepW-primaryW, 0)
	}
	return primaryW, sepW, secondaryW
}

// massRenamePreviewRow is preview row i: the before name (removed / replaced / ignored parts
// styled) and the after name (added parts or the row error styled), each padded to its column in
// its base style, with the separator between, as one pre-styled row.
func massRenamePreviewRow(st FileDialogState, i, primaryW, sepW, secondaryW int, bg tcell.Color, styles theme.Theme) dlg.ListRow {
	beforeBase := styles.DialogMassRenameBefore.Background(bg)
	afterBase := styles.DialogMassRenameAfter.Background(bg)
	var removed, replaced, added []search.Range
	var ignored search.Range
	if i < len(st.MassRenamePreviewBeforeIgnored) {
		ignored = st.MassRenamePreviewBeforeIgnored[i]
	}
	if i < len(st.MassRenamePreviewBeforeRemoved) {
		removed = st.MassRenamePreviewBeforeRemoved[i]
	}
	if i < len(st.MassRenamePreviewBeforeReplaced) {
		replaced = st.MassRenamePreviewBeforeReplaced[i]
	}
	if i < len(st.MassRenamePreviewAfterAdded) {
		added = st.MassRenamePreviewAfterAdded[i]
	}
	before, beforeSpans := massRenameBeforePreviewRow(st.MassRenamePreviewBefore[i], removed, replaced, ignored, primaryW, beforeBase,
		styles.DialogMassRenameBeforeRemoved, styles.DialogMassRenameBeforeReplaced, styles.DialogOptionDisabledStyle())
	rowError := i < len(st.MassRenamePreviewAfterError) && st.MassRenamePreviewAfterError[i]
	after, afterSpans := massRenameAfterPreviewRow(st.MassRenamePreviewAfter[i], added, secondaryW, afterBase,
		styles.DialogMassRenameAfterAdded, styles.DialogMassRenameAfterError.Background(bg), rowError)
	pad := func(s string, w int) string {
		s = primitive.TruncateRight(s, w)
		return s + strings.Repeat(" ", max(w-runewidth.StringWidth(s), 0))
	}
	before = pad(before, primaryW)
	after = pad(after, secondaryW)
	afterStart := utf8.RuneCountInString(before) + sepW
	text := before + strings.Repeat(" ", sepW) + after
	spans := beforeSpans
	for _, sp := range afterSpans {
		spans = append(spans, primitive.Span{Start: sp.Start + afterStart, End: sp.End + afterStart, Style: sp.Style})
	}
	// Under the highlights (the first span covering a rune wins), each column's base style.
	spans = append(spans,
		primitive.Span{End: afterStart, Style: beforeBase},
		primitive.Span{Start: afterStart, End: afterStart + utf8.RuneCountInString(after), Style: afterBase})
	return dlg.ListRow{Text: text, Spans: spans}
}

// massRenamePatternPickerView is the spec of a pattern picker screen (load, history or overwrite).
func massRenamePatternPickerView(st FileDialogState, ctx FileDialogViewContext) dlg.Spec {
	pk := st.MassRenameLoadPicker
	if st.MassRenamePhase == MassRenamePhaseHistoryPicker {
		pk = st.MassRenameHistoryPicker
	}
	return pickerView(st, ctx, len(pk.Items), func(i int) string { return MassRenamePatternSearchLine(pk.Items[i]) },
		pk.Ranked, pk.MatchRanges, pk.Query, pk.QueryCursor, pk.QueryScroll, pk.Selected, pk.ListScroll)
}
