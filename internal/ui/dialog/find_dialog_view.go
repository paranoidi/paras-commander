package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

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
	_, _, listH, _ := FindDialogMetrics(ctx.Layout, state.ShowSearchSelectionsOption)
	optionRows := 1
	if state.ShowSearchSelectionsOption {
		optionRows = 2
	}

	query := dlg.Item{
		ID: FindQuery, Kind: dlg.KindInput, Grow: true, Drives: FindList,
		Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll, LeadingIcon: styles.IconSearchIcon(),
	}
	depth := dlg.Item{
		ID: FindDepth, Kind: dlg.KindInput, Width: findDialogDepthFieldWidth(styles) + 1,
		Value: state.DepthInput, Cursor: utf8.RuneCountInString(state.DepthInput), LeadingIcon: styles.IconDialogDepth(),
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
		{Kind: dlg.KindRow, Cells: []dlg.Item{query, depth}},
		{Kind: dlg.KindSeparator},
		{Kind: dlg.KindRow, Cells: options, EntryLast: true}, // Up from below lands on Stay on current volume
	}
	if state.ShowSearchSelectionsOption {
		col = append(col, dlg.Item{ID: FindSelections, Kind: dlg.KindCheckbox, Label: "Search only from selected directories", Mnemonic: 'S', On: state.SearchOnlySelections})
	}
	col = append(col,
		dlg.Item{Kind: dlg.KindSeparator},
		dlg.Item{
			ID: FindList, Kind: dlg.KindList, Driven: true, Height: listH, Fit: dlg.FitPath, MarksAfterText: true, Scrollbar: true,
			Rows: findListRows(state, ctx), Cursor: state.Selected, Offset: state.ListScroll,
		},
	)
	return dlg.Spec{
		Title: findDialogTitle(*state, styles), Width: findDialogPreferredWidth,
		MinWidth: findDialogMinWidth, MinHeight: findDialogBaseHeight(optionRows) + findDialogMinListRows,
		Groups:          []dlg.Group{{Columns: [][]dlg.Item{col}}},
		Buttons:         findDialogButtons(),
		ButtonsLabel:    ctx.SelectionLabel,
		PlainButtonKeys: true, ScrollbarStyle: ctx.ScrollbarStyle,
	}
}

func findDialogButtons() []dlg.Button {
	return []dlg.Button{{ID: dlg.OK, Label: "OK", Mnemonic: 'O'}, {ID: dlg.Cancel, Label: "Cancel", Mnemonic: 'C'}}
}

// findListRows turns the ranked results into list rows: the display line (indexing may show a
// line before its entry lands), match ranges, marked state and row marks.
func findListRows(state *FindDialogState, ctx FindViewContext) []dlg.ListRow {
	rows := make([]dlg.ListRow, 0, len(state.Ranked))
	icon := ""
	if ctx.IconLead > 0 {
		icon = strings.Repeat(findListIconPad, ctx.IconLead-1)
	}
	for i, entIdx := range state.Ranked {
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
