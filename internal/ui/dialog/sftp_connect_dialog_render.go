package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Widget IDs of the SFTP connect dialog. The host list is driven by the filter input and has no
// focus of its own.
const (
	SFTPFilter   dlg.ID = "sftp-filter"
	SFTPHosts    dlg.ID = "sftp-hosts"
	SFTPLocation dlg.ID = "sftp-location"
)

// sftpConnectMinWidth is the narrowest terminal-clamped dialog width that still draws.
const sftpConnectMinWidth = 40

// SFTPConnectListRows is how many host rows the dialog shows on layout (the list grows with the
// terminal, up to 12 rows).
func SFTPConnectListRows(layout Layout) int {
	return fittedListHeight(sftpConnectFrame(SFTPConnectDialogState{}, layout, theme.Theme{}), layout, SFTPHosts)
}

// sftpHostList is the host list item: the ranked display lines with their fuzzy highlights.
func sftpHostList(state SFTPConnectDialogState, listRows int) dlg.Item {
	list := dlg.Item{
		ID: SFTPHosts, Kind: dlg.KindList, Driven: true, Fit: dlg.FitMiddle,
		Cursor: state.Selected, Offset: state.ListScroll, Height: listRows,
	}
	for _, ent := range state.Ranked {
		var row dlg.ListRow
		if ent >= 0 && ent < len(state.DisplayLines) {
			row.Text = state.DisplayLines[ent]
			if ent < len(state.MatchRanges) {
				row.Match = state.MatchRanges[ent]
			}
		}
		list.Rows = append(list.Rows, row)
	}
	return list
}

// EnsureSFTPConnectListScroll keeps Selected in range and visible in the host list.
func EnsureSFTPConnectListScroll(state *SFTPConnectDialogState, listRows int) {
	EnsurePickerScroll(len(state.Ranked), &state.Selected, &state.ListScroll, listRows)
}

// SFTPConnectView describes the SFTP dialog: a filter input that steers the host list below it,
// the location input and OK / Cancel.
func SFTPConnectView(state SFTPConnectDialogState, layout Layout, styles theme.Theme) dlg.Spec {
	spec, _ := fillList(sftpConnectFrame(state, layout, styles), layout, SFTPHosts, func(height, _ int) dlg.Item {
		return sftpHostList(state, height)
	})
	return spec
}

// sftpConnectFrame is the SFTP spec with an empty host list as tall as it wants (3..12 rows,
// leaving a margin around the dialog); on a screen too short for that the list shrinks first.
func sftpConnectFrame(state SFTPConnectDialogState, layout Layout, styles theme.Theme) dlg.Spec {
	filter := dlg.Item{
		ID: SFTPFilter, Kind: dlg.KindInput, Drives: SFTPHosts, LeadingIcon: styles.IconSearchIcon(),
		Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll,
	}
	return dlg.Spec{
		Title: "SFTP", Width: min(78, layout.Width-4), Unclamped: true,
		Groups: []dlg.Group{
			{Columns: [][]dlg.Item{{filter, {Kind: dlg.KindSeparator}, {ID: SFTPHosts, Kind: dlg.KindList, Height: min(max(layout.Height-16, 3), 12)}}}},
			{Columns: [][]dlg.Item{{
				{Kind: dlg.KindSeparator},
				{Kind: dlg.KindLabel, Label: "Location:"},
				fileFieldInput(SFTPLocation, state.Location, false),
			}}},
		},
		Buttons: dlg.OKCancelButtons(),
	}
}

func DrawSFTPConnectDialog(screen tcell.Screen, layout Layout, state SFTPConnectDialogState, styles theme.Theme) {
	if min(78, layout.Width-4) < sftpConnectMinWidth {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, SFTPConnectView(state, layout, styles), state.Focus, styles)
}
