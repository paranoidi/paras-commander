package app

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type remotePanelLoadPayload struct {
	panelID int
	gen     uint64
	req     panel.RemoteLoadRequest
	entries []fsbackend.Entry
	err     error
}

func (a *App) wireRemotePanelLoaders() {
	a.model.Left.ScheduleRemoteLoad = a.remoteLoadScheduler(ui.LeftPanel)
	a.model.Right.ScheduleRemoteLoad = a.remoteLoadScheduler(ui.RightPanel)
}

func (a *App) remoteLoadScheduler(panelID int) panel.RemoteLoadScheduler {
	return func(req panel.RemoteLoadRequest) bool {
		gen := a.remotePanelLoadGen[panelID].Add(1)
		showHidden := a.panelByID(panelID).ShowHidden
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.config.SFTP.ListTimeoutSecs)*time.Second)
			defer cancel()
			entries, err := panel.FetchRemoteListing(ctx, req.Loc, showHidden)
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(remotePanelLoadPayload{
				panelID: panelID,
				gen:     gen,
				req:     req,
				entries: entries,
				err:     err,
			}))
		}()
		return true
	}
}

func (a *App) applyRemotePanelLoad(p remotePanelLoadPayload) bool {
	if a.remotePanelLoadGen[p.panelID].Load() != p.gen {
		return false
	}
	pan := a.panelByID(p.panelID)
	pan.ListingPending = false
	if p.err != nil {
		if p.req.Rollback != nil {
			p.req.Rollback()
		}
		a.setErrorMessage("Remote list failed", p.err)
		return true
	}
	if err := pan.ApplyListing(p.req.Loc, p.entries, p.req.SelectedName, p.req.ViewportRows, p.req.IndexFallback); err != nil {
		if p.req.Rollback != nil {
			p.req.Rollback()
		}
		a.setErrorMessage("Remote list failed", err)
		return true
	}
	if p.req.SyncHistoryHead && pan.HistoryIndex == 0 && len(pan.History) > 0 {
		pan.History[0] = pan.PathString()
	}
	return true
}
