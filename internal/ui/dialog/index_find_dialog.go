package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/findcrit"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Find-in-index dialog widget IDs. IndexFindQuery drives IndexFindList. The advanced section's
// widgets are IndexFindAdvModeID(m) and IndexFindAdvScopeID(sc).
const (
	IndexFindQuery     = dlg.ID("index-find:query")
	IndexFindOnlyDirs  = dlg.ID("index-find:only-dirs")
	IndexFindOnlyFiles = dlg.ID("index-find:only-files")
	IndexFindHidden    = dlg.ID("index-find:hidden")
	IndexFindDepth     = dlg.ID("index-find:depth")
	IndexFindList      = dlg.ID("index-find:list")
	// IndexFindAdvancedToggle is the Advanced checkbox on the query row.
	IndexFindAdvancedToggle = dlg.ID("index-find:advanced")

	indexFindAdvPrefix      = "index-find:adv-" // every advanced-section widget
	indexFindAdvModePrefix  = "index-find:adv-mode-"
	indexFindAdvScopePrefix = "index-find:adv-scope-"
)

// IndexFindAdvModeID is the ID of match mode radio m.
func IndexFindAdvModeID(m findcrit.IndexMode) dlg.ID {
	return dlg.IndexedID(indexFindAdvModePrefix, int(m))
}

// IndexFindAdvModeOf is the inverse of IndexFindAdvModeID.
func IndexFindAdvModeOf(id dlg.ID) (findcrit.IndexMode, bool) {
	i, ok := dlg.IDIndex(id, indexFindAdvModePrefix)
	return findcrit.IndexMode(i), ok && i >= 0 && i < len(findcrit.IndexModeLabels)
}

// IndexFindAdvScopeID is the ID of scope radio sc.
func IndexFindAdvScopeID(sc IndexFindScope) dlg.ID {
	return dlg.IndexedID(indexFindAdvScopePrefix, int(sc))
}

// IndexFindAdvScopeOf is the inverse of IndexFindAdvScopeID.
func IndexFindAdvScopeOf(id dlg.ID) (IndexFindScope, bool) {
	i, ok := dlg.IDIndex(id, indexFindAdvScopePrefix)
	return IndexFindScope(i), ok && i >= 0 && i < len(IndexScopeLabels)
}

// IndexFindViewContext is what IndexFindDialogView needs besides the state.
type IndexFindViewContext struct {
	Layout         Layout
	Styles         theme.Theme
	ShowIcons      bool
	ScrollbarStyle uiscrollbar.Style
	// RowMarks resolves pin / job marks by a row's fileindex:// path (nil = none).
	RowMarks RowMarksFunc
}

// IndexFindListRows is the result list's visible row count for st (its advanced section and
// error line take rows from the list) on layout (0 when the dialog does not fit).
func IndexFindListRows(st IndexFindDialogState, layout Layout) int {
	st.Results = nil
	return fittedListHeight(IndexFindDialogView(&st, IndexFindViewContext{Layout: layout}), layout, IndexFindList)
}

func indexFindTitle(st *IndexFindDialogState, styles theme.Theme) string {
	title := "Find in index"
	if st.Err == "" && st.Query != "" && !st.Searching {
		suffix := ""
		if st.Truncated {
			suffix = "+"
		}
		title = fmt.Sprintf("%s (%d%s)", title, len(st.Results), suffix)
	}
	if st.Searching {
		title = fmt.Sprintf("%s %c", title, styles.IconMenuJob("running"))
	}
	return title
}

// The match mode and scope tooltips explain the query syntax and the scope (fileindex.Index.Find).
const (
	indexFindTermsTooltip = "Space-separated terms match file and directory names, ignoring case.\n" +
		"beach 2019 - both, in any order\n" +
		"beach | lake - either; binds tighter than the space\n" +
		"!2019 - names without it\n" +
		"^img, .jpg$ - names starting, ending with it\n" +
		"*.jpg, report-?? - with * or ? the whole name must match\n" +
		"\"summer beach\" or summer\\ beach - literal text"
	indexFindPatternTooltip = "Matches file and directory names, ignoring case.\n" +
		"Plain text finds names containing it: holiday\n" +
		"With * (any run) or ? (one character) the whole name must match: *.jpg"
	indexFindBooleanTooltip = "Matches whole words in file names (no directories), ignoring case.\n" +
		"beach 2019 - both words (also beach AND 2019)\n" +
		"beach OR lake - either word\n" +
		"beach NOT 2019 - the first without the second\n" +
		"(beach OR lake) AND 2019 - parentheses group\n" +
		"holi* - words starting with holi\n" +
		"\"summer beach\" - words next to each other, in this order\n" +
		"AND, OR and NOT are uppercase. Quote text with punctuation: \"report.pdf\""
	indexFindWholeTooltip    = "Searches everything in the index, wherever the panel is.\nResults show their full path."
	indexFindCurrentTooltip  = "Searches the panel's directory and below.\nResults show their path from it."
	indexFindSelectedTooltip = "Searches the directories selected in the panel and below.\nResults show their path from the panel's directory."
	indexFindDepthTooltip    = findDepthSyntax + "\nIt filters the indexed results; nothing is read from disk.\n" +
		"A depth counts from the panel's directory, so setting one limits the search to the current directory."
)

var indexFindAdvTooltips = map[dlg.ID]string{
	IndexFindAdvModeID(findcrit.IndexTerms):   indexFindTermsTooltip,
	IndexFindAdvModeID(findcrit.IndexPattern): indexFindPatternTooltip,
	IndexFindAdvModeID(findcrit.IndexBoolean): indexFindBooleanTooltip,
	IndexFindAdvScopeID(IndexScopeWhole):      indexFindWholeTooltip,
	IndexFindAdvScopeID(IndexScopeCurrent):    indexFindCurrentTooltip,
	IndexFindAdvScopeID(IndexScopeSelected):   indexFindSelectedTooltip,
}

// indexFindAdvMnemonics are the letters of the advanced section's radios, live while focus is off
// the query and depth inputs. They avoid the letters of the rest of the dialog (N, Y, I, the
// Advanced key, OK, Cancel).
var indexFindAdvMnemonics = map[dlg.ID]rune{
	IndexFindAdvModeID(findcrit.IndexTerms):   'T',
	IndexFindAdvModeID(findcrit.IndexPattern): 'P',
	IndexFindAdvModeID(findcrit.IndexBoolean): 'B',
	IndexFindAdvScopeID(IndexScopeWhole):      'W',
	IndexFindAdvScopeID(IndexScopeCurrent):    'D',
	IndexFindAdvScopeID(IndexScopeSelected):   'S',
}

// indexFindAdvancedRows are the advanced section's Match and Search in rows for a row width of rowW
// cells; Selected directories is offered only when the panel had selected directories.
func indexFindAdvancedRows(st *IndexFindDialogState, rowW int) []dlg.Item {
	adv := st.Advanced
	var modes, scopes []dlg.Item
	for m, text := range findcrit.IndexModeLabels {
		modes = append(modes, dlg.Item{ID: IndexFindAdvModeID(findcrit.IndexMode(m)), Kind: dlg.KindRadio, Label: text, On: adv.Mode == findcrit.IndexMode(m)})
	}
	for sc, text := range IndexScopeLabels {
		if IndexFindScope(sc) == IndexScopeSelected && !st.ShowSelectedScope {
			continue
		}
		scopes = append(scopes, dlg.Item{ID: IndexFindAdvScopeID(IndexFindScope(sc)), Kind: dlg.KindRadio, Label: text, On: adv.Scope == IndexFindScope(sc), Seg: 1})
	}
	rows := optionGridRows([]string{"Match", "Search in"}, [][]dlg.Item{modes, scopes}, rowW)
	for i := range rows {
		for j := range rows[i].Cells {
			c := &rows[i].Cells[j]
			if r, ok := indexFindAdvMnemonics[c.ID]; ok {
				c.Mnemonic, c.MnemonicOutsideInputs = r, true
				c.Tooltip = indexFindAdvTooltips[c.ID]
			}
		}
	}
	return rows
}

// IndexFindDialogView is the find-in-index dialog's widget spec: the query, depth input and
// Advanced checkbox on one row, the Only directories / Only files / Include hidden options, the
// advanced section (match mode and scope) when expanded, the result list driven by the query and
// OK / Cancel.
func IndexFindDialogView(st *IndexFindDialogState, ctx IndexFindViewContext) dlg.Spec {
	adv := st.Advanced
	queryIcon := ctx.Styles.IconSearchIcon()
	if tag := adv.Mode.Tag(); tag != "" {
		queryIcon += " " + tag // the query is not read as terms: say how
	}
	query := dlg.Item{ID: IndexFindQuery, Kind: dlg.KindInput, Grow: true, Drives: IndexFindList, Invalid: st.NoMatches(),
		Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll, LeadingIcon: queryIcon}
	depth := dlg.Item{ID: IndexFindDepth, Kind: dlg.KindInput, Width: findDialogDepthFieldWidth(ctx.Styles) + findDialogDepthGap,
		Value: st.DepthInput, Cursor: utf8.RuneCountInString(st.DepthInput), LeadingIcon: ctx.Styles.IconDialogDepth(),
		Gap: 1, Tooltip: indexFindDepthTooltip}
	queryRow := []dlg.Item{query, depth, {ID: IndexFindAdvancedToggle, Kind: dlg.KindCheckbox, Label: findAdvancedLabel, Mnemonic: st.AdvancedMnemonic, On: adv.Expanded, Seg: 1}}
	options := []dlg.Item{
		{ID: IndexFindOnlyDirs, Kind: dlg.KindRadio, Label: "Only directories", Mnemonic: 'N', On: st.OnlyDirs, Disabled: adv.Mode == findcrit.IndexBoolean, Gap: findOptionsGap},
		{ID: IndexFindOnlyFiles, Kind: dlg.KindRadio, Label: "Only files", Mnemonic: 'Y', On: st.OnlyFiles, Gap: findOptionsGap},
		{ID: IndexFindHidden, Kind: dlg.KindCheckbox, Label: "Include hidden", Mnemonic: 'I', On: st.IncludeHidden},
	}
	col := []dlg.Item{
		{Kind: dlg.KindRow, Cells: queryRow},
		{Kind: dlg.KindSeparator},
		{Kind: dlg.KindRow, Cells: options},
	}
	frame := dlg.Spec{Width: findDialogPreferredWidth, MinWidth: findDialogMinWidth, Buttons: dlg.OKCancelButtons()}
	rowW := dlg.RowWidth(ctx.Layout.Width, ctx.Layout.Height, frame)
	groups := []dlg.Group{{Columns: [][]dlg.Item{col}}}
	listSep := dlg.Item{Kind: dlg.KindSeparator}
	list := dlg.Item{ID: IndexFindList, Kind: dlg.KindList, Height: findListHeight(ctx.Layout.Height), MinHeight: findDialogMinListRows}
	if adv.Expanded {
		advCol := append([]dlg.Item{{Kind: dlg.KindSeparator, Label: findAdvancedLabel}}, indexFindAdvancedRows(st, rowW)...)
		groups = append(groups, dlg.Group{Columns: [][]dlg.Item{advCol}})
		// The open section takes its rows from the results (see FindDialogView).
		list.Height = max(list.Height-len(advCol), 1)
		list.MinHeight = 1
	} else {
		listSep.Label = adv.Summary(ctx.Styles, rowW-4) // the separator pads its label by a cell each side
	}
	tail := []dlg.Item{listSep}
	if st.Err != "" {
		tail = append(tail, dlg.Item{Kind: dlg.KindLabel, Label: st.Err, Style: dlg.TextError})
	}
	groups = append(groups, dlg.Group{Columns: [][]dlg.Item{append(tail, list)}})
	spec := dlg.Spec{
		Title: indexFindTitle(st, ctx.Styles), Width: frame.Width, MinWidth: frame.MinWidth,
		Groups:  groups,
		Buttons: frame.Buttons, ButtonsLabel: st.MarkedLabel(),
		ScrollbarStyle: ctx.ScrollbarStyle,
	}
	spec, _ = fillList(spec, ctx.Layout, IndexFindList, func(height, _ int) dlg.Item {
		from, to := dlg.ListWindow(st.Selected, st.ListScroll, height, len(st.Results))
		rows := make([]dlg.ListRow, 0, to-from)
		for _, r := range st.Results[from:to] {
			row := dlg.ListRow{Text: r.Rel, Marked: st.Marked[r.Path]}
			if ctx.ShowIcons {
				row.Icon = ctx.Styles.IconFile()
				if r.IsDir {
					row.Icon = ctx.Styles.IconFolder()
				}
			}
			if ctx.RowMarks != nil {
				row.Marks = ctx.RowMarks(pathloc.FileIndexPrefix + r.Path)
			}
			rows = append(rows, row)
		}
		return dlg.Item{
			ID: IndexFindList, Kind: dlg.KindList, Driven: true, Height: height, Fit: dlg.FitPath, MarksAfterText: true, Scrollbar: true,
			Inert:    st.AdvancedFocused(),
			RowCount: len(st.Results), Rows: rows, RowsFrom: from, Cursor: st.Selected, Offset: st.ListScroll,
		}
	})
	return spec
}

// EnsureIndexFindListScroll keeps Selected in range and visible in a list of height listRows.
func EnsureIndexFindListScroll(st *IndexFindDialogState, listRows int) {
	EnsurePickerScroll(len(st.Results), &st.Selected, &st.ListScroll, listRows)
}

// DrawIndexFindDialog paints the find-in-index dialog.
func DrawIndexFindDialog(screen tcell.Screen, layout Layout, st IndexFindDialogState, ctx DialogRenderContext, rowMarks RowMarksFunc) {
	spec := IndexFindDialogView(&st, IndexFindViewContext{
		Layout: layout, Styles: ctx.Styles, ShowIcons: ctx.ShowIcons, ScrollbarStyle: ctx.ScrollbarStyle, RowMarks: rowMarks,
	})
	dlg.Draw(screen, layout.Width, layout.Height, spec, st.Focus, ctx.Styles)
}
