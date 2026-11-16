package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Disk usage prompt button IDs. Cancel is dlg.Cancel (mnemonic C), so Clear takes its second
// letter.
const (
	DiskUsageRescan = dlg.ID("disk-usage-rescan")
	DiskUsageClear  = dlg.ID("disk-usage-clear")
)

// DiskUsagePromptState holds the question asked when a disk usage scan is requested while one
// is running or its data is on screen.
type DiskUsagePromptState struct {
	Open    bool
	Focus   dlg.ID // DiskUsageRescan (the default), DiskUsageClear or dlg.Cancel
	PanelID int    // the panel the scan request came from
	// Scanning: a scan was running when the dialog opened (picks the message).
	Scanning bool
}

// DiskUsagePromptView is the disk usage prompt's widget spec: one message line above
// Rescan / Clear / Cancel.
func DiskUsagePromptView(s DiskUsagePromptState) dlg.Spec {
	msg := "Disk usage data is shown."
	if s.Scanning {
		msg = "A disk usage scan is running."
	}
	return dlg.Spec{
		Title: "Disk usage", Width: 44, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: msg},
		}}}},
		Buttons: []dlg.Button{
			{ID: DiskUsageRescan, Label: "Rescan", Mnemonic: 'R'},
			{ID: DiskUsageClear, Label: "Clear", Mnemonic: 'L'},
			dlg.CancelButton,
		},
	}
}

// DrawDiskUsagePromptDialog paints the disk usage prompt from its widget spec.
func DrawDiskUsagePromptDialog(screen tcell.Screen, layout Layout, state DiskUsagePromptState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, DiskUsagePromptView(state), state.Focus, styles)
}
