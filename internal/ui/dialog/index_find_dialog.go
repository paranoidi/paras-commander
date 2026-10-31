package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Find-in-index dialog widget IDs. IndexFindQuery drives IndexFindList.
const (
	IndexFindQuery      = dlg.ID("index-find:query")
	IndexFindPattern    = dlg.ID("index-find:pattern")
	IndexFindBoolean    = dlg.ID("index-find:boolean")
	IndexFindOnlyDirs   = dlg.ID("index-find:only-dirs")
	IndexFindOnlyFiles  = dlg.ID("index-find:only-files")
	IndexFindHidden     = dlg.ID("index-find:hidden")
	IndexFindSelections = dlg.ID("index-find:selections")
	IndexFindDepth      = dlg.ID("index-find:depth")
	IndexFindList       = dlg.ID("index-find:list")
)

// IndexFindViewContext is what IndexFindDialogView needs besides the state.
type IndexFindViewContext struct {
	Layout         Layout
	Styles         theme.Theme
	ShowIcons      bool
	ScrollbarStyle uiscrollbar.Style
	// RowMarks resolves pin / job marks by a row's fileindex:// path (nil = none).
	RowMarks RowMarksFunc
}

// IndexFindListRows is the result list's visible row count on layout (0 when the dialog does not
// fit).
func IndexFindListRows(layout Layout) int {
	return fittedListHeight(IndexFindDialogView(&IndexFindDialogState{}, IndexFindViewContext{Layout: layout}), layout, IndexFindList)
}

func indexFindTitle(st *IndexFindDialogState, styles theme.Theme) string {
	title := "Find in index"
	if st.JumpMode {
		title = "Jump to directory"
	}
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

// IndexFindDialogView is the find-in-index dialog's widget spec: the query, the Pattern / Boolean and
// Only directories / Only files radios (hidden in jump mode), the result list driven by the
// query and OK / Cancel.
func IndexFindDialogView(st *IndexFindDialogState, ctx IndexFindViewContext) dlg.Spec {
	const gap = 5
	query := dlg.Item{ID: IndexFindQuery, Kind: dlg.KindInput, Grow: true, Drives: IndexFindList,
		Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll, LeadingIcon: ctx.Styles.IconSearchIcon()}
	depth := dlg.Item{ID: IndexFindDepth, Kind: dlg.KindInput, Width: findDialogDepthFieldWidth(ctx.Styles) + findDialogDepthGap,
		Value: st.DepthInput, Cursor: utf8.RuneCountInString(st.DepthInput), LeadingIcon: ctx.Styles.IconDialogDepth()}
	col := []dlg.Item{{Kind: dlg.KindRow, Cells: []dlg.Item{query, depth}}}
	options := []dlg.Item{}
	if !st.JumpMode {
		col = append(col, dlg.Item{Kind: dlg.KindRow, Cells: []dlg.Item{
			{ID: IndexFindPattern, Kind: dlg.KindRadio, Label: "Pattern", Mnemonic: 'P', On: !st.Boolean, Gap: gap},
			{ID: IndexFindBoolean, Kind: dlg.KindRadio, Label: "Boolean", Mnemonic: 'B', On: st.Boolean},
		}})
		options = append(options,
			dlg.Item{ID: IndexFindOnlyDirs, Kind: dlg.KindRadio, Label: "Only directories", Mnemonic: 'N', On: st.OnlyDirs, Disabled: st.Boolean, Gap: gap},
			dlg.Item{ID: IndexFindOnlyFiles, Kind: dlg.KindRadio, Label: "Only files", Mnemonic: 'Y', On: st.OnlyFiles, Gap: gap},
		)
	}
	options = append(options, dlg.Item{ID: IndexFindHidden, Kind: dlg.KindCheckbox, Label: "Include hidden", Mnemonic: 'I', On: st.IncludeHidden})
	col = append(col, dlg.Item{Kind: dlg.KindRow, Cells: options})
	if st.ShowSearchSelectionsOption {
		col = append(col, dlg.Item{ID: IndexFindSelections, Kind: dlg.KindCheckbox, Label: "Search only from selected directories", Mnemonic: 'S', On: st.SearchOnlySelections})
	}
	col = append(col, dlg.Item{Kind: dlg.KindSeparator})
	if st.Err != "" {
		col = append(col, dlg.Item{Kind: dlg.KindLabel, Label: st.Err, Style: dlg.TextError})
	}
	col = append(col, dlg.Item{ID: IndexFindList, Kind: dlg.KindList, Height: findListHeight(ctx.Layout.Height), MinHeight: findDialogMinListRows})
	spec := dlg.Spec{
		Title: indexFindTitle(st, ctx.Styles), Width: findDialogPreferredWidth, MinWidth: findDialogMinWidth,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{col}}},
		Buttons: dlg.OKCancelButtons(), ButtonsLabel: st.MarkedLabel(),
		PlainButtonKeys: true, ScrollbarStyle: ctx.ScrollbarStyle,
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
