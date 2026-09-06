package app

import (
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fsvol"
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
// unfinished job source or destination (typical NAS during copy). Job volume devices
// are cached on the job at enqueue (jobs.Job.VolumeDevs), so this stats only the query
// path — never the job's own paths on the mount the job is saturating — and only once,
// when at least one unfinished job has cached devices.
func (a *App) pathVolumeContendsWithActiveJob(path string) bool {
	if !a.jobState.HasUnfinishedWork() {
		return false
	}
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return false
	}
	var dev uint64
	devOK := false
	for _, job := range a.jobState.AllJobs() {
		if job.Status.IsFinished() || len(job.VolumeDevs) == 0 {
			continue
		}
		if !devOK {
			if dev, devOK = diskusage.PathDevice(path); !devOK {
				return false
			}
		}
		if job.HasVolumeDev(dev) {
			return true
		}
	}
	return false
}

// filterJobContendedPaths drops paths that share a local device with an unfinished job's
// source/destination, so automatic disk-usage scans don't compete with a job already
// saturating that volume. Explicit user-requested scans intentionally don't call this.
func (a *App) filterJobContendedPaths(paths []string) []string {
	if len(paths) == 0 || !a.jobState.HasUnfinishedWork() {
		return paths
	}
	out := paths[:0:0]
	for _, p := range paths {
		if !a.pathVolumeContendsWithActiveJob(p) {
			out = append(out, p)
		}
	}
	return out
}

func (a *App) requestVolumeSpaceRefreshAsync(panelID int) {
	p := a.panelByID(panelID)
	if p == nil {
		return
	}
	path := filepath.Clean(p.PathString())
	if path == "" || path == "." {
		return
	}
	if panelID < 0 || panelID > ui.SecondaryPanel {
		return
	}
	if !a.volumeRefreshInFlight[panelID].CompareAndSwap(false, true) {
		return
	}
	go func(panelID int, path string) {
		// Contention suppression stats the panel path; on a mount saturated by a copy
		// that stat can block for seconds, so it runs here with the statfs, off the UI thread.
		if a.pathVolumeContendsWithActiveJob(path) {
			a.volumeRefreshInFlight[panelID].Store(false)
			return
		}
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
	a.requestVolumeSpaceRefreshAsync(ui.PrimaryPanel)
	a.requestVolumeSpaceRefreshAsync(ui.SecondaryPanel)
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
