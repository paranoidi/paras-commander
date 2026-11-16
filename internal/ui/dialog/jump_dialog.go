package dialog

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Jump to directory widget IDs. JumpQuery drives JumpList: list focus lives in the query input.
const (
	JumpQuery     = dlg.ID("jump:query")
	JumpDepth     = dlg.ID("jump:depth")
	JumpHidden    = dlg.ID("jump:hidden")
	JumpStayOnVol = dlg.ID("jump:stay-on-volume")
	JumpList      = dlg.ID("jump:list")
)

// JumpViewContext is what JumpDialogView needs besides the state.
type JumpViewContext struct {
	Layout         Layout
	Styles theme.Theme
	// IconLead is the cells each row keeps free before its text for the devicon DrawJumpDialog
	// paints there (0 = no icons).
	IconLead       int
	ScrollbarStyle uiscrollbar.Style
	// RowMarks resolves pin / job marks by a row's panel path (nil = none).
	RowMarks func(r JumpResult) RowMarks
}

// JumpListRows is the result list's visible row count on layout (0 when the dialog does not fit).
func JumpListRows(layout Layout, st *JumpDialogState) int {
	shape := JumpDialogState{Source: st.Source, Err: st.Err}
	return fittedListHeight(JumpDialogView(&shape, JumpViewContext{Layout: layout}), layout, JumpList)
}

func jumpDialogTitle(st *JumpDialogState, styles theme.Theme) string {
	const name = "Jump to directory"
	switch {
	case st.Scanning:
		title := fmt.Sprintf("%s (%s%s)", name, formatIndexedCount(st.Scanned), string(primitive.Ellipsis))
		if st.LevelTotal > 0 {
			depth := styles.IconDialogDepth()
			if styles.UseNerdfontIcons {
				depth += " " // the glyph renders wide and crowds the number
			}
			title = fmt.Sprintf("%s %s %d · %d%% %c", title, depth, st.Depth, st.LevelDone*100/st.LevelTotal, styles.IconMenuJob("scanning"))
		}
		return title
	case st.Searching:
		return fmt.Sprintf("%s %c", name, styles.IconMenuJob("running"))
	case st.Source == JumpDisk && st.Scanned > 0 && st.Query == "":
		return fmt.Sprintf("%s (%s)", name, formatIndexedCount(st.Scanned))
	case st.Query != "" && st.Err == "":
		suffix := ""
		if st.Truncated() {
			suffix = "+"
		}
		return fmt.Sprintf("%s (%d%s)", name, len(st.Results), suffix)
	}
	return name
}

// jumpDepthTooltip explains the depth input for each source.
func jumpDepthTooltip(src JumpSource) string {
	if src == JumpIndex {
		return findDepthSyntax + "\nIt filters the indexed directories; nothing is read from disk."
	}
	return findDepthSyntax + "\nA lower limit drops deeper directories at once; a higher one rescans."
}

// JumpDialogView is the jump-to-directory dialog's widget spec: the query and depth inputs on
// one row, Include hidden and (on disk) Stay on current volume, the result list driven by the
// query, and OK / Cancel. It shares Find's frame and list sizes (docs/agents/dialogs.md, Shared
// dialog sizes).
func JumpDialogView(st *JumpDialogState, ctx JumpViewContext) dlg.Spec {
	const gap = 5
	query := dlg.Item{ID: JumpQuery, Kind: dlg.KindInput, Grow: true, Drives: JumpList, Invalid: st.NoMatches(),
		Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll, LeadingIcon: ctx.Styles.IconSearchIcon()}
	depth := dlg.Item{ID: JumpDepth, Kind: dlg.KindInput, Width: findDialogDepthFieldWidth(ctx.Styles) + findDialogDepthGap,
		Value: st.DepthInput, Cursor: utf8.RuneCountInString(st.DepthInput), LeadingIcon: ctx.Styles.IconDialogDepth(),
		Tooltip: jumpDepthTooltip(st.Source)}
	options := []dlg.Item{{ID: JumpHidden, Kind: dlg.KindCheckbox, Label: "Include hidden", Mnemonic: 'I', On: st.IncludeHidden, Gap: gap}}
	if st.Source == JumpDisk {
		options = append(options, dlg.Item{ID: JumpStayOnVol, Kind: dlg.KindCheckbox, Label: "Stay on current volume", Mnemonic: 'V', On: st.StayOnVolume})
	}
	col := []dlg.Item{
		{Kind: dlg.KindRow, Cells: []dlg.Item{query, depth}},
		{Kind: dlg.KindRow, Cells: options},
		{Kind: dlg.KindSeparator},
	}
	if st.Err != "" {
		col = append(col, dlg.Item{Kind: dlg.KindLabel, Label: st.Err, Style: dlg.TextError})
	}
	col = append(col, dlg.Item{ID: JumpList, Kind: dlg.KindList, Height: findListHeight(ctx.Layout.Height), MinHeight: findDialogMinListRows})
	spec := dlg.Spec{
		Title: jumpDialogTitle(st, ctx.Styles), Width: findDialogPreferredWidth, MinWidth: findDialogMinWidth,
		Groups:         []dlg.Group{{Columns: [][]dlg.Item{col}}},
		Buttons:        dlg.OKCancelButtons(),
		ScrollbarStyle: ctx.ScrollbarStyle,
	}
	spec, _ = fillList(spec, ctx.Layout, JumpList, func(height, _ int) dlg.Item {
		from, to := dlg.ListWindow(st.Selected, st.ListScroll, height, len(st.Results))
		rows := make([]dlg.ListRow, 0, to-from)
		icon := ""
		if ctx.IconLead > 0 {
			icon = strings.Repeat(findListIconPad, ctx.IconLead-1)
		}
		for _, r := range st.Results[from:to] {
			row := dlg.ListRow{Text: r.Rel, Match: r.Match, Icon: icon}
			if ctx.RowMarks != nil {
				row.Marks = ctx.RowMarks(r)
			}
			rows = append(rows, row)
		}
		return dlg.Item{
			ID: JumpList, Kind: dlg.KindList, Driven: true, Height: height, Fit: dlg.FitPath, MarksAfterText: true, Scrollbar: true,
			RowCount: len(st.Results), Rows: rows, RowsFrom: from, Cursor: st.Selected, Offset: st.ListScroll,
		}
	})
	return spec
}

// EnsureJumpListScroll keeps Selected in range and visible in a list of height listRows.
func EnsureJumpListScroll(st *JumpDialogState, listRows int) {
	EnsurePickerScroll(len(st.Results), &st.Selected, &st.ListScroll, listRows)
}

// JumpRowIconPainter draws the panel devicon for one jump dialog row; nil skips icons.
type JumpRowIconPainter func(screen tcell.Screen, x, y int, r JumpResult, styles theme.Theme)

// DrawJumpDialog paints the jump-to-directory dialog from its widget spec, then the row devicons
// over the leading cells each list row reserves for them (as DrawFindDialog). It returns the area
// it painted (zero when the dialog does not fit).
func DrawJumpDialog(screen tcell.Screen, layout Layout, st JumpDialogState, ctx DialogRenderContext, paintIcon JumpRowIconPainter, rowMarks func(JumpResult) RowMarks) PaintedArea {
	iconLead := ctx.IconLead
	if !ctx.ShowIcons || paintIcon == nil {
		iconLead = 0
	}
	spec := JumpDialogView(&st, JumpViewContext{
		Layout: layout, Styles: ctx.Styles, IconLead: iconLead, ScrollbarStyle: ctx.ScrollbarStyle, RowMarks: rowMarks,
	})
	p, ok := dlg.Draw(screen, layout.Width, layout.Height, spec, st.Focus, ctx.Styles)
	if !ok {
		return PaintedArea{}
	}
	if iconLead > 0 {
		list, _ := p.Item(JumpList)
		p.EachListRow(JumpList, func(row, y int) {
			paintIcon(screen, list.Rect.X, y, st.Results[row], ctx.Styles)
		})
	}
	return PaintedArea{Frame: p.Rect}
}
