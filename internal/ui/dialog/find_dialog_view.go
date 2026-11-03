package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// findDepthSyntax opens the depth input's tooltip in Find and Find in index (twins).
const findDepthSyntax = "At most this many path levels below the root: a is 1, a/b is 2.\n" +
	"A digit 1-9 sets it; 0, Backspace or Delete clear it for unlimited."

// findDepthTooltip explains Find's max depth input (see find.Handler.commitFindDepth).
const findDepthTooltip = findDepthSyntax + "\nA lower limit drops deeper results at once; a higher one re-indexes."

// Find dialog widget IDs. FindQuery drives FindList: list focus lives in the query input.
const (
	FindQuery       = dlg.ID("find:query")
	FindDepth       = dlg.ID("find:depth")
	FindOnlyDirs    = dlg.ID("find:only-dirs")
	FindOnlyFiles   = dlg.ID("find:only-files")
	FindHidden      = dlg.ID("find:hidden")
	FindStayOnVol   = dlg.ID("find:stay-on-volume")
	FindSelections  = dlg.ID("find:selections")
	FindList        = dlg.ID("find:list")
	findOptionsGap  = 5 // blank cells between the option row's radios and checkboxes
	findListIconPad = " "
)

// FindViewContext is what FindDialogView needs besides the state.
type FindViewContext struct {
	Layout         Layout
	Styles         theme.Theme
	IconLead       int // cells before each row's text for its devicon; 0 = no icons
	ScrollbarStyle uiscrollbar.Style
	// SelectionLabel is the marked-selection size shown in the rule above the buttons ("" = none).
	SelectionLabel string
	RowMarks       RowMarksFunc
}

// FindDialogView is the find (and jump-to-directory) dialog's widget spec: the query and depth
// inputs on one row, the result filters and scope options, the ranked result list driven by the
// query, and OK / Cancel. Row devicons are painted over the list's leading cells by
// DrawFindDialog (see FindRowIconPainter).
func FindDialogView(state *FindDialogState, ctx FindViewContext) dlg.Spec {
	styles := ctx.Styles
	adv := state.Advanced
	advanced := !state.JumpMode // jump mode is a plain fuzzy directory search

	queryIcon := styles.IconSearchIcon()
	if tag := adv.Mode.Tag(); advanced && tag != "" {
		queryIcon += " " + tag // the query is not fuzzy: say how it is read
	}
	query := dlg.Item{
		ID: FindQuery, Kind: dlg.KindInput, Grow: true, Drives: FindList, Invalid: state.QueryInvalid || state.NoMatches(),
		Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll, LeadingIcon: queryIcon,
	}
	depth := dlg.Item{
		ID: FindDepth, Kind: dlg.KindInput, Width: findDialogDepthFieldWidth(styles) + findDialogDepthGap,
		Value: state.DepthInput, Cursor: utf8.RuneCountInString(state.DepthInput), LeadingIcon: styles.IconDialogDepth(),
		Tooltip: findDepthTooltip,
	}
	queryRow := []dlg.Item{query, depth}
	if advanced {
		depth.Gap = 1
		queryRow = []dlg.Item{query, depth, {ID: FindAdvanced, Kind: dlg.KindCheckbox, Label: findAdvancedLabel, Mnemonic: state.AdvancedMnemonic, On: adv.Expanded, Seg: 1}}
	}
	var options []dlg.Item
	if !state.JumpMode {
		options = append(options,
			dlg.Item{ID: FindOnlyDirs, Kind: dlg.KindRadio, Label: "Only directories", Mnemonic: 'N', On: state.OnlyDirectories, Gap: findOptionsGap},
			dlg.Item{ID: FindOnlyFiles, Kind: dlg.KindRadio, Label: "Only files", Mnemonic: 'Y', On: state.OnlyFiles, Gap: findOptionsGap},
		)
	}
	options = append(options,
		dlg.Item{ID: FindHidden, Kind: dlg.KindCheckbox, Label: "Include hidden", Mnemonic: 'I', On: state.IncludeHidden, Gap: findOptionsGap},
		dlg.Item{ID: FindStayOnVol, Kind: dlg.KindCheckbox, Label: "Stay on current volume", Mnemonic: 'V', On: state.StayOnCurrentVolume},
	)
	col := []dlg.Item{
		{Kind: dlg.KindRow, Cells: queryRow},
		{Kind: dlg.KindSeparator},
		{Kind: dlg.KindRow, Cells: options},
	}
	if state.ShowSearchSelectionsOption {
		col = append(col, dlg.Item{ID: FindSelections, Kind: dlg.KindCheckbox, Label: "Search only from selected directories", Mnemonic: 'S', On: state.SearchOnlySelections})
	}
	frame := dlg.Spec{Width: findDialogPreferredWidth, MinWidth: findDialogMinWidth, Buttons: dlg.OKCancelButtons()}
	rowW := dlg.RowWidth(ctx.Layout.Width, ctx.Layout.Height, frame)
	groups := []dlg.Group{{Columns: [][]dlg.Item{col}}}
	listSep := dlg.Item{Kind: dlg.KindSeparator}
	list := dlg.Item{ID: FindList, Kind: dlg.KindList, Height: findListHeight(ctx.Layout.Height), MinHeight: findDialogMinListRows}
	switch {
	case advanced && adv.Expanded:
		advCol := append([]dlg.Item{{Kind: dlg.KindSeparator, Label: "Advanced"}}, findAdvancedRows(adv, rowW)...)
		groups = append(groups, dlg.Group{Columns: [][]dlg.Item{advCol}})
		// The open section takes its rows from the results, so the frame wants the height it
		// has with the section closed (down to a one-row list); fitting shrinks both alike.
		list.Height = max(list.Height-len(advCol), 1)
		list.MinHeight = 1
	case advanced:
		listSep.Label = adv.Summary(styles, rowW-4) // the separator pads its label by a cell each side
	}
	groups = append(groups, dlg.Group{Columns: [][]dlg.Item{{listSep, list}}})
	spec := dlg.Spec{
		Title: findDialogTitle(*state, styles), Width: frame.Width, MinWidth: frame.MinWidth,
		Groups:         groups,
		Buttons:        frame.Buttons,
		ButtonsLabel:   ctx.SelectionLabel,
		ScrollbarStyle: ctx.ScrollbarStyle,
	}
	// Only the visible results are built, so the list's fitted height comes first.
	spec, _ = fillList(spec, ctx.Layout, FindList, func(height, _ int) dlg.Item {
		from, to := dlg.ListWindow(state.Selected, state.ListScroll, height, len(state.Ranked))
		return dlg.Item{
			ID: FindList, Kind: dlg.KindList, Driven: true, Height: height, Fit: dlg.FitPath, MarksAfterText: true, Scrollbar: true,
			Inert:    state.AdvancedFocused(),
			RowCount: len(state.Ranked), Rows: findListRows(state, ctx, from, to), RowsFrom: from,
			Cursor: state.Selected, Offset: state.ListScroll,
		}
	})
	return spec
}

// findListRows turns ranked results [from, to) into list rows: the display line (indexing may
// show a line before its entry lands), match ranges, marked state and row marks.
func findListRows(state *FindDialogState, ctx FindViewContext, from, to int) []dlg.ListRow {
	rows := make([]dlg.ListRow, 0, to-from)
	icon := ""
	if ctx.IconLead > 0 {
		icon = strings.Repeat(findListIconPad, ctx.IconLead-1)
	}
	for i := from; i < to; i++ {
		entIdx := state.Ranked[i]
		row := dlg.ListRow{Icon: icon}
		var ranges []search.Range
		if ent, ok := state.FindEntryAt(entIdx); ok {
			row.Text = ent.RelLine
			absPath := ent.AbsPath(state.RootPath)
			row.Marked = state.MarkedPaths[absPath]
			if ctx.RowMarks != nil {
				row.Marks = ctx.RowMarks(absPath)
			}
			ranges = state.MatchRanges[entIdx]
		} else if entIdx >= 0 {
			ranges = state.MatchRanges[entIdx]
		}
		if i < len(state.RankDisplayLines) && state.RankDisplayLines[i] != "" {
			row.Text = state.RankDisplayLines[i]
		}
		row.Match = ranges
		rows = append(rows, row)
	}
	return rows
}
