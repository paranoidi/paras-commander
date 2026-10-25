package dialog

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Widget IDs of the spec-driven file dialogs. The buttons are dlg.OK and dlg.Cancel (Yes / No on
// the delete confirmation).
const (
	FileRenameFocusAfter    = dlg.ID("focus-after")
	FileRenameOpenOther     = dlg.ID("open-in-other")
	FileSanitizeDots        = dlg.ID("sanitize-dots")
	FileSanitizeUnderscores = dlg.ID("sanitize-underscores")
	FileSlugifyDot          = dlg.ID("slugify-dot")
	FileSlugifyUnderscore   = dlg.ID("slugify-underscore")
	fileDeleteList          = dlg.ID("delete-list")
	fileFieldPrefix         = "field-"
	fileEncodingPrefix      = "encoding-"
	fileMkdirPrefix         = "mkdir-"
)

// FileFieldID is the widget ID of text input i of a file dialog (index into Fields).
func FileFieldID(i int) dlg.ID { return dlg.ID(fileFieldPrefix + strconv.Itoa(i)) }

// FileFieldIndex returns the Fields index named by a FileFieldID.
func FileFieldIndex(id dlg.ID) (int, bool) {
	rest, ok := strings.CutPrefix(string(id), fileFieldPrefix)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(rest)
	return i, err == nil
}

// MkdirActionID is the radio ID of a mkdir post-action.
func MkdirActionID(a MkdirAction) dlg.ID { return dlg.ID(fileMkdirPrefix + strconv.Itoa(int(a))) }

// MkdirActionFromID returns the post-action of a MkdirActionID radio.
func MkdirActionFromID(id dlg.ID) (MkdirAction, bool) {
	rest, ok := strings.CutPrefix(string(id), fileMkdirPrefix)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(rest)
	return MkdirAction(i), err == nil
}

func renameEncodingID(i int) dlg.ID { return dlg.ID(fileEncodingPrefix + strconv.Itoa(i)) }

func renameEncodingIndex(id dlg.ID) (int, bool) {
	rest, ok := strings.CutPrefix(string(id), fileEncodingPrefix)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(rest)
	return i, err == nil
}

// FileDialogViewContext carries what FileDialogView needs besides the state: the screen layout
// (wide rename mode and the delete list height follow it) and the cells kept free before each
// delete row for an icon (0 = none).
type FileDialogViewContext struct {
	Layout   Layout
	IconLead int
	// Styles supplies the theme's search icon for the picker sub-screens.
	Styles theme.Theme
}

// FileDialogView is the widget spec of every file dialog, built from the same parts:
//   - the text inputs (label row, input row, a blank row between inputs) from state.Fields, each a
//     Tab segment of its own;
//   - below them, in one more group behind a separator, the rows only some members have: rename's
//     and duplicate's focus-after checkbox (plus rename's open-in-other-panel checkbox on a
//     directory) under a whitespace warning row, mkdir's post-action radios;
//   - the rename tool screens (Preview, then Sanitize checkboxes, Slugify radios or Encoding
//     radios) in place of the inputs;
//   - the delete confirmation: a reserved region for the name list (DrawFileDialog paints its rows,
//     icons and scrollbar), the summary and OK / Cancel as Yes / No;
//   - add bookmark's path head, the extract skipped-items note and the mass rename save prompt reuse
//     the input groups; run-for-each (runForEachView, its history picker) and the mass rename main
//     screen and pattern pickers (massRenameView, massRenamePatternPickerView) have their own views.
func FileDialogView(st FileDialogState, ctx FileDialogViewContext) dlg.Spec {
	if st.DialogType == FileDialogRunForEach {
		if st.RunForEachHistoryOpen {
			return runForEachHistoryView(st, ctx)
		}
		return runForEachView(st, ctx)
	}
	if st.DialogType == FileDialogMassRename {
		switch {
		case MassRenamePickerPhase(st.MassRenamePhase):
			return massRenamePatternPickerView(st, ctx)
		case st.MassRenamePhase == MassRenamePhaseMain:
			return massRenameView(st, ctx)
		}
	}
	spec := dlg.Spec{
		Title: fileDialogOuterTitle(st), Width: fileDialogWidth(ctx.Layout.Width, st, ctx.IconLead), Unclamped: true,
		Buttons: []dlg.Button{
			{ID: dlg.OK, Label: "OK", Mnemonic: 'O'},
			{ID: dlg.Cancel, Label: "Cancel", Mnemonic: 'C'},
		},
	}
	switch {
	case st.DialogType == FileDialogDelete:
		spec.Buttons = []dlg.Button{
			{ID: dlg.OK, Label: "Yes", Mnemonic: 'Y', Destructive: true},
			{ID: dlg.Cancel, Label: "No", Mnemonic: 'N'},
		}
		spec.Groups = deleteViewGroups(st, ctx)
	case renameToolActive(st):
		spec.Groups = renameToolViewGroups(st)
	case st.DialogType == FileDialogAddBookmark:
		spec.Groups = append(bookmarkHeadGroups(st), fieldViewGroups(st)...)
	default:
		spec.Groups = fieldViewGroups(st)
	}
	return spec
}

// fileFocusedFieldIndex is the index of the focused text input of a spec dialog, or -1.
func fileFocusedFieldIndex(st FileDialogState) int {
	if st.Focus == "" {
		if len(st.Fields) > 0 && !renameToolActive(st) && st.DialogType != FileDialogDelete {
			return 0
		}
		return -1
	}
	if i, ok := FileFieldIndex(st.Focus); ok && i < len(st.Fields) {
		return i
	}
	return -1
}

// FileDialogFocusedField returns the index into Fields of the focused text input of a
// file dialog (the first input while Focus is unset), or -1.
func (st FileDialogState) FileDialogFocusedField() int { return fileFocusedFieldIndex(st) }

// bookmarkHeadGroups is the add bookmark dialog's read-only head: the path being bookmarked
// (state.Message) and a rule above the Name input.
func bookmarkHeadGroups(st FileDialogState) []dlg.Group {
	return []dlg.Group{{Columns: [][]dlg.Item{{
		{Kind: dlg.KindLabel, Label: "Path:"},
		{Kind: dlg.KindLabel, Label: st.Message},
		{Kind: dlg.KindSeparator},
	}}}}
}

func fieldViewGroups(st FileDialogState) []dlg.Group {
	focused := fileFocusedFieldIndex(st)
	var items []dlg.Item
	for i, f := range st.Fields {
		label := dlg.Item{Kind: dlg.KindLabel, Label: f.Label + ":"}
		if st.Message != "" && i == focused && st.DialogType != FileDialogAddBookmark {
			label.Style = dlg.TextWarn
		}
		items = append(items, label, fileFieldInput(FileFieldID(i), f, f.InputInvalid))
		if i < len(st.Fields)-1 {
			items = append(items, dlg.Item{Kind: dlg.KindLabel})
		}
	}
	if st.DialogType == FileDialogExtract {
		// The note under the destination (skipped items) keeps the warning style.
		for _, line := range strings.Split(strings.TrimSpace(st.Message), "\n") {
			if line != "" {
				items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: line, Style: dlg.TextWarn})
			}
		}
	}
	groups := []dlg.Group{{Columns: [][]dlg.Item{items}}}
	var options []dlg.Item
	switch {
	case renameHasFocusCheckbox(st):
		options = append(options, dlg.Item{ID: FileRenameFocusAfter, Kind: dlg.KindCheckbox, Label: renameFocusCheckboxLabel(st), Mnemonic: 'A', On: st.RenameFocusAfter})
		if renameHasOpenOtherCheckbox(st) {
			options = append(options, dlg.Item{ID: FileRenameOpenOther, Kind: dlg.KindCheckbox, Label: renameOpenOtherLabel, Mnemonic: 'p', On: st.RenameOpenInOther})
		}
	case mkdirHasActions(st):
		for _, r := range MkdirActionRadioSpecs() {
			options = append(options, dlg.Item{ID: MkdirActionID(r.Action), Kind: dlg.KindRadio, Label: r.Label, Mnemonic: r.Shortcut, On: st.MkdirAction == r.Action})
		}
	}
	if len(options) == 0 {
		return groups
	}
	// The row under the last input holds the whitespace warning (empty when there is none).
	warn := dlg.Item{Kind: dlg.KindLabel, Style: dlg.TextError}
	if len(st.Fields) > 0 && focused != 0 {
		warn.Label = renameWhitespaceWarningText(st.Fields[0].Value)
	}
	if !renameHasFocusCheckbox(st) {
		warn.Label = ""
	}
	if warn.Label == "" {
		warn.Style = dlg.TextNormal // a blank row keeps the plain surface style
	}
	groups[0].Columns[0] = append(groups[0].Columns[0], warn, dlg.Item{Kind: dlg.KindSeparator})
	return append(groups, dlg.Group{Columns: [][]dlg.Item{options}})
}

// renameWhitespaceWarningText is the warning for a name with leading or trailing whitespace.
func renameWhitespaceWarningText(v string) string {
	lead := strings.TrimLeftFunc(v, unicode.IsSpace) != v
	trail := strings.TrimRightFunc(v, unicode.IsSpace) != v
	switch {
	case lead && trail:
		return "Name has leading and trailing whitespace"
	case lead:
		return "Name has leading whitespace"
	case trail:
		return "Name has trailing whitespace"
	}
	return ""
}

func renameToolViewGroups(st FileDialogState) []dlg.Group {
	head := []dlg.Item{
		{Kind: dlg.KindLabel, Label: "Preview:"},
		{Kind: dlg.KindLabel, Label: renameToolPreviewText(st)},
		{Kind: dlg.KindSeparator},
	}
	var options []dlg.Item
	switch st.RenamePhase {
	case RenamePhaseSanitize:
		options = []dlg.Item{
			{ID: FileSanitizeDots, Kind: dlg.KindCheckbox, Label: `Replace "." with space`, Mnemonic: '.', On: st.RenameSanitizeDots},
			{ID: FileSanitizeUnderscores, Kind: dlg.KindCheckbox, Label: `Replace "_" with space`, Mnemonic: '_', On: st.RenameSanitizeUnderscores},
		}
	case RenamePhaseSlugify:
		options = []dlg.Item{
			{ID: FileSlugifyDot, Kind: dlg.KindRadio, Label: `Replace space with "."`, Mnemonic: '.', On: st.RenameSlugifySep == RenameSlugifyDot},
			{ID: FileSlugifyUnderscore, Kind: dlg.KindRadio, Label: `Replace space with "_"`, Mnemonic: '_', On: st.RenameSlugifySep == RenameSlugifyUnderscore},
		}
	case RenamePhaseEncoding:
		for i := range st.RenameEncodingCandidates {
			options = append(options, dlg.Item{
				ID: renameEncodingID(i), Kind: dlg.KindRadio, Label: RenameEncodingOptionLabel(st, i),
				Mnemonic: RenameEncodingOptionShortcut(st, i), On: st.RenameEncodingSelected == i,
			})
		}
	}
	groups := []dlg.Group{{Columns: [][]dlg.Item{head}}}
	if len(options) > 0 {
		groups = append(groups, dlg.Group{Columns: [][]dlg.Item{options}})
	}
	return groups
}

func deleteViewGroups(st FileDialogState, ctx FileDialogViewContext) []dlg.Group {
	summary := []dlg.Item{{Kind: dlg.KindSeparator}}
	if strings.TrimSpace(st.DeleteWarning) != "" {
		summary = append(summary, dlg.Item{Kind: dlg.KindLabel, Label: strings.TrimSpace(st.DeleteWarning)})
	}
	summary = append(summary, dlg.Item{Kind: dlg.KindLabel, Label: st.DeleteSummary, Center: true})
	return []dlg.Group{
		{Columns: [][]dlg.Item{{{ID: fileDeleteList, Kind: dlg.KindSpacer, Height: DeleteDialogListViewportRows(ctx.Layout.Height, st)}}}},
		{Columns: [][]dlg.Item{summary}},
	}
}

// Apply folds an activation from dlg.HandleKey / dlg.Click into the state: the rename checkboxes
// toggle, mkdir radios select their action, the tool screen options toggle or select. The inputs
// and the buttons are the caller's to handle.
func (st *FileDialogState) Apply(ev dlg.Event) {
	if st.DialogType == FileDialogRunForEach {
		st.applyRunForEachEvent(ev)
		return
	}
	switch ev.ID {
	case FileRenameFocusAfter:
		st.RenameFocusAfter = !st.RenameFocusAfter
	case FileRenameOpenOther:
		st.RenameOpenInOther = !st.RenameOpenInOther
	case FileSanitizeDots:
		st.RenameSanitizeDots = !st.RenameSanitizeDots
	case FileSanitizeUnderscores:
		st.RenameSanitizeUnderscores = !st.RenameSanitizeUnderscores
	case FileSlugifyDot:
		st.RenameSlugifySep = RenameSlugifyDot
	case FileSlugifyUnderscore:
		st.RenameSlugifySep = RenameSlugifyUnderscore
	}
	if a, ok := MkdirActionFromID(ev.ID); ok {
		st.MkdirAction = a
	}
	if i, ok := renameEncodingIndex(ev.ID); ok {
		st.RenameEncodingSelected = i
	}
}

// SpecFocus is the focused widget of the screen FileDialogView describes: the form's Focus, or the
// open picker sub-screen's own.
func (st FileDialogState) SpecFocus() dlg.ID {
	switch {
	case st.DialogType == FileDialogRunForEach && st.RunForEachHistoryOpen:
		return st.RunForEachHistoryPicker.Focus
	case st.DialogType == FileDialogMassRename && st.MassRenamePhase == MassRenamePhaseHistoryPicker:
		return st.MassRenameHistoryPicker.Focus
	case st.DialogType == FileDialogMassRename && MassRenamePickerPhase(st.MassRenamePhase):
		return st.MassRenameLoadPicker.Focus
	}
	return st.Focus
}

// drawFileDialogSpec paints a file dialog: the spec, then what lives in reserved
// regions and on the input rects (the delete list with its icons and scrollbar, the focused path
// field's completion dropdown).
func drawFileDialogSpec(screen tcell.Screen, layout Layout, state FileDialogState, ctx DialogRenderContext, paintDeleteIcon DeleteRowIconPainter) {
	spec := FileDialogView(state, FileDialogViewContext{Layout: layout, IconLead: ctx.IconLead, Styles: ctx.Styles})
	if fileDialogWidth(layout.Width, state, ctx.IconLead) < 20 {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, spec, state.SpecFocus(), ctx.Styles)
	p, ok := dlg.Place(layout.Width, layout.Height, spec)
	if !ok {
		return
	}
	if state.DialogType == FileDialogDelete {
		drawDeleteListRows(screen, p, state, ctx, paintDeleteIcon)
		return
	}
	if state.DialogType == FileDialogMassRename && state.MassRenamePhase == MassRenamePhaseMain {
		drawMassRenamePreview(screen, p, state, ctx)
		return
	}
	if i := fileFocusedFieldIndex(state); i >= 0 && state.Fields[i].PathPicker && state.Fields[i].Completion.Open {
		if in, ok := p.Item(FileFieldID(i)); ok {
			f := state.Fields[i]
			drawPathCompletionDropdown(screen, in.Rect.X, in.Rect.Y+1, f.Scroll, f.Completion, ctx.ScrollbarStyle, ctx.Styles)
		}
	}
}

// drawDeleteListRows paints the visible slice of the delete name list into the reserved region:
// a devicon (when icons are shown), the fitted name, and the scrollbar on the right border.
func drawDeleteListRows(screen tcell.Screen, p dlg.Placement, state FileDialogState, ctx DialogRenderContext, paintIcon DeleteRowIconPainter) {
	region, ok := p.Item(fileDeleteList)
	if !ok {
		return
	}
	styles := ctx.Styles
	listStyle := styles.DialogOptionRowStyle(false, false)
	textX, textW := region.Rect.X+1, region.Rect.Width-2
	withIcons := ctx.ShowIcons && paintIcon != nil && ctx.IconLead > 0
	if withIcons {
		textX += ctx.IconLead
		textW = max(textW-ctx.IconLead, 1)
	}
	vp := region.Rect.Height
	n := len(state.DeleteEntries)
	scroll := min(max(state.DeleteListScroll, 0), max(n-vp, 0))
	for i := 0; i < vp && scroll+i < n; i++ {
		y := region.Rect.Y + i
		if y >= p.ContentBottom {
			break
		}
		e := state.DeleteEntries[scroll+i]
		if withIcons {
			paintIcon(screen, region.Rect.X+1, y, e, styles)
		}
		primitive.Text(screen, textX, y, textW, DeleteListEntryNameFitsWidth(e.Name, e.Path, textW), listStyle)
	}
	_, surface, _ := styles.DialogSurface.Decompose()
	sb := ctx.ScrollbarStyle
	if sb == "" {
		sb = uiscrollbar.StyleThumb
	}
	draw.DrawDialogListScrollbar(screen, p.Rect, region.Rect.Y, vp, n, scroll, sb, styles.DialogFrame.Background(surface), styles)
}
