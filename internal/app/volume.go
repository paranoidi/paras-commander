package app

import (
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fsvol"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// volumeSpaceRefreshPayload delivers async statfs results to the main loop.
type volumeSpaceRefreshPayload struct {
	PanelID int
	Path    string
	Avail   uint64
	Total   uint64
	OK      bool
}

// pathVolumeContendsWithActiveJob reports whether path sits on the same volume as an
// unfinished job source or destination (typical NAS during copy).
func (a *App) pathVolumeContendsWithActiveJob(path string) bool {
	if !a.jobState.HasUnfinishedWork() {
		return false
	}
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return false
	}
	for _, job := range a.jobState.AllJobs() {
		if job.Status.IsFinished() {
			continue
		}
		if panelSharesVolumeWithJob(path, job) {
			return true
		}
	}
	return false
}

// panelVolumeRefreshSuppressed skips statfs on a panel while a job hammers the same volume
// (typical NAS source panel during copy). Browsing another mount stays refreshable.
func (a *App) panelVolumeRefreshSuppressed(panelID int) bool {
	p := a.panelByID(panelID)
	if p == nil {
		return false
	}
	return a.pathVolumeContendsWithActiveJob(p.PathString())
}

func panelSharesVolumeWithJob(panelPath string, job *jobs.Job) bool {
	if job.Destination.IsRemote() {
		return false
	}
	panelDev, panelOK := diskusage.PathDevice(panelPath)
	if !panelOK {
		return false
	}
	check := func(p string) bool {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			return false
		}
		dev, ok := diskusage.PathDevice(p)
		return ok && dev == panelDev
	}
	for _, src := range job.Sources {
		host, err := src.FilePath()
		if err != nil {
			continue
		}
		if check(host) {
			return true
		}
	}
	destHost, err := job.Destination.FilePath()
	if err != nil {
		return false
	}
	return check(destHost)
}

func (a *App) requestVolumeSpaceRefreshAsync(panelID int) {
	p := a.panelByID(panelID)
	if p == nil {
		return
	}
	if a.panelVolumeRefreshSuppressed(panelID) {
		return
	}
	path := filepath.Clean(p.PathString())
	if path == "" || path == "." {
		return
	}
	if panelID < 0 || panelID > ui.RightPanel {
		return
	}
	if !a.volumeRefreshInFlight[panelID].CompareAndSwap(false, true) {
		return
	}
	go func(panelID int, path string) {
		avail, total, ok := fsvol.VolumeBytes(path)
		a.volumeRefreshInFlight[panelID].Store(false)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(volumeSpaceRefreshPayload{
			PanelID: panelID,
			Path:    path,
			Avail:   avail,
			Total:   total,
			OK:      ok,
		}))
	}(panelID, path)
}

func (a *App) requestBothPanelsVolumeSpaceRefreshAsync() {
	a.requestVolumeSpaceRefreshAsync(ui.LeftPanel)
	a.requestVolumeSpaceRefreshAsync(ui.RightPanel)
}

// applyVolumeSpaceRefresh merges async statfs into panel state. It returns true when
// displayed free-space values changed (caller may repaint).
func (a *App) applyVolumeSpaceRefresh(d volumeSpaceRefreshPayload) bool {
	p := a.panelByID(d.PanelID)
	if p == nil {
		return false
	}
	if filepath.Clean(p.PathString()) != filepath.Clean(d.Path) {
		return false
	}
	if p.VolumeSpaceOK == d.OK && p.VolumeAvailBytes == d.Avail && p.VolumeTotalBytes == d.Total {
		return false
	}
	p.VolumeSpaceOK = d.OK
	p.VolumeAvailBytes = d.Avail
	p.VolumeTotalBytes = d.Total
	return true
}

func (a *App) runVolumeSpaceTicker(interval time.Duration, stop <-chan struct{}) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if !a.jobState.HasUnfinishedWork() {
				continue
			}
			// Avoid periodic statfs + EventInterrupt while browsing during copy unless the user
			// opted into progress-driven refresh (same mount contention as copy I/O).
			if a.model.ViewMode != ui.ViewJobs && !a.config.Jobs.FreeSpaceOnProgressWake {
				continue
			}
			a.requestBothPanelsVolumeSpaceRefreshAsync()
		}
	}
}
