package jobs

import (
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Host supplies cross-cutting app services the jobs handler cannot import from internal/app.
type Host interface {
	host.LayoutHost
	host.MessageHost
	host.PanelHost
	host.ShellHost

	SetUnsupportedMessage(msg string)
	RefreshBothPanels()
	RequestBothPanelsVolumeSpaceRefreshAsync()
	OpenTransferDialogSelfCopyRename(kind dialog.TransferKind, absDest, source string)
	SetJobFailedTransientMessage(err error, fallback string)
	DevMode() bool
}
