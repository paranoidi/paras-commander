package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
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
// hints, the Capitalize checkboxes, or nothing for the external editor), a rule and the reserved
// preview region (drawMassRenamePreview paints the before / after rows into it), above OK / Apply /
// Cancel. The mode radios are Tab stops of one segment with the option row; each input is a
// segment of its own.
func massRenameView(st FileDialogState, ctx FileDialogViewContext) dlg.Spec {
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
	spec.Groups = append(spec.Groups, dlg.Group{Columns: [][]dlg.Item{{
		{ID: massRenamePreview, Kind: dlg.KindSpacer, Height: MassRenamePreviewViewportRows(ctx.Layout.Height, st)},
	}}})
	return spec
}

// drawMassRenamePreview paints the before / after preview rows (or the compute error) into the
// reserved preview region, with the scrollbar on the dialog's right border.
func drawMassRenamePreview(screen tcell.Screen, p dlg.Placement, state FileDialogState, ctx DialogRenderContext) {
	region, ok := p.Item(massRenamePreview)
	if !ok {
		return
	}
	styles := ctx.Styles
	_, dbg, _ := styles.DialogSurface.Decompose()
	_, surface, _ := styles.DialogSurface.Decompose()
	borderStyle := styles.DialogFrame.Background(surface)
	beforeBase := styles.DialogMassRenameBefore.Background(dbg)
	beforeRemoved := styles.DialogMassRenameBeforeRemoved
	beforeReplaced := styles.DialogMassRenameBeforeReplaced
	afterBase := styles.DialogMassRenameAfter.Background(dbg)
	afterAdded := styles.DialogMassRenameAfterAdded
	afterError := styles.DialogMassRenameAfterError
	primaryCol := region.Rect.X + 1
	innerW := region.Rect.Width - 2
	if innerW <= 0 {
		return
	}
	vp := region.Rect.Height
	y := region.Rect.Y
	previewTopY := y
	if errMsg := strings.TrimSpace(state.MassRenameComputeError); errMsg != "" {
		primitive.Text(screen, primaryCol, y, innerW, errMsg, styles.MessageWarn.Background(dbg))
		draw.DrawDialogListScrollbar(screen, p.Rect, previewTopY, vp, 1, 0, ctx.ScrollbarStyle, borderStyle, styles)
		return
	}
	before := state.MassRenamePreviewBefore
	after := state.MassRenamePreviewAfter
	n := min(len(before), len(after))
	scroll := min(max(state.MassRenamePreviewScroll, 0), max(n-vp, 0))
	sepW := 0
	primaryW := innerW / 2
	secondaryW := innerW - primaryW
	if innerW >= 3 {
		sepW = 1
		avail := innerW - sepW
		primaryW = avail / 2
		secondaryW = avail - primaryW
	}
	if primaryW < 1 {
		primaryW = 1
		secondaryW = max(innerW-sepW-primaryW, 0)
	}
	for row := 0; row < vp; row++ {
		i := scroll + row
		if i >= n {
			break
		}
		var removed, replaced, added []search.Range
		var ignored search.Range
		if i < len(state.MassRenamePreviewBeforeIgnored) {
			ignored = state.MassRenamePreviewBeforeIgnored[i]
		}
		if i < len(state.MassRenamePreviewBeforeRemoved) {
			removed = state.MassRenamePreviewBeforeRemoved[i]
		}
		if i < len(state.MassRenamePreviewBeforeReplaced) {
			replaced = state.MassRenamePreviewBeforeReplaced[i]
		}
		if i < len(state.MassRenamePreviewAfterAdded) {
			added = state.MassRenamePreviewAfterAdded[i]
		}
		lbText, lbSpans := massRenameBeforePreviewRow(before[i], removed, replaced, ignored, primaryW, beforeBase, beforeRemoved, beforeReplaced, styles.DialogOptionDisabledStyle())
		rowError := i < len(state.MassRenamePreviewAfterError) && state.MassRenamePreviewAfterError[i]
		rbText, rbSpans := massRenameAfterPreviewRow(after[i], added, secondaryW, afterBase, afterAdded, afterError.Background(dbg), rowError)
		primitive.StyledText(screen, primaryCol, y, primaryW, lbText, beforeBase, lbSpans)
		if sepW == 1 {
			screen.SetContent(primaryCol+primaryW, y, ' ', nil, beforeBase)
		}
		primitive.StyledText(screen, primaryCol+primaryW+sepW, y, secondaryW, rbText, afterBase, rbSpans)
		y++
	}
	draw.DrawDialogListScrollbar(screen, p.Rect, previewTopY, vp, n, scroll, ctx.ScrollbarStyle, borderStyle, styles)
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
