package app

import (
	"fmt"
	"io"

	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// imageOverlay tracks the currently locked/emitted terminal image on the tty.
type imageOverlay struct {
	last        previewpanel.ImagePlacement
	lastSet     bool
	lastCols    int
	lastRows    int
	pendingEmit bool
	// pendingDelete is set when a previously emitted Kitty image must be deleted
	// before the next Show (plan cleared or replaced).
	pendingDelete bool
}

func (a *App) imageOverlaySuppressed() bool {
	return a.model.ModalDialogOpen() || a.model.Menu.Open
}

func (a *App) cellPixelDims() (cw, ch int) {
	tty, ok := a.screen.Tty()
	if !ok {
		return 10, 20
	}
	ws, err := tty.WindowSize()
	if err != nil {
		return 10, 20
	}
	cw, ch = ws.CellDimensions()
	if cw <= 0 || ch <= 0 {
		return 10, 20
	}
	return cw, ch
}

// reconcileImageBeforeShow updates LockRegion state for the upcoming Show().
// Returns true when the locked region changed (forces Show past the hash cache).
func (a *App) reconcileImageBeforeShow(plan *previewpanel.ImagePlacement) (forceShow bool) {
	var cols, rows int
	if plan != nil {
		cw, ch := a.cellPixelDims()
		cols = (plan.PxW + cw - 1) / cw
		rows = (plan.PxH + ch - 1) / ch
		if cols < 1 {
			cols = 1
		}
		if rows < 1 {
			rows = 1
		}
		if cols > plan.MaxCols || rows > plan.MaxRows {
			plan = nil
		}
	}

	// Path is ignored: during stale-while-revalidate the held payload is shown under the
	// new filename; tearing down the overlay for a path-only change flashes the cell buffer.
	if plan != nil && a.image.lastSet &&
		plan.Payload == a.image.last.Payload &&
		plan.Protocol == a.image.last.Protocol &&
		plan.X == a.image.last.X &&
		plan.Y == a.image.last.Y &&
		cols == a.image.lastCols &&
		rows == a.image.lastRows {
		a.image.last.Path = plan.Path
		return false
	}

	if a.image.lastSet {
		wasKitty := a.image.last.Protocol == previewpanel.ImageProtocolKitty
		a.screen.LockRegion(a.image.last.X, a.image.last.Y, a.image.lastCols, a.image.lastRows, false)
		a.image.lastSet = false
		a.image.last = previewpanel.ImagePlacement{}
		a.image.lastCols = 0
		a.image.lastRows = 0
		// Kitty a=T with the same image id replaces in place — only delete when clearing
		// (no new plan). Deleting before a replacement exposes the cell buffer underneath.
		if wasKitty && plan == nil {
			a.image.pendingDelete = true
		}
		forceShow = true
	}

	if plan != nil {
		a.screen.LockRegion(plan.X, plan.Y, cols, rows, true)
		a.image.last = *plan
		a.image.lastSet = true
		a.image.lastCols = cols
		a.image.lastRows = rows
		a.image.pendingEmit = true
		forceShow = true
	}
	return forceShow
}

// emitImageAfterShow writes pending Kitty deletes and/or the image payload to the tty.
func (a *App) emitImageAfterShow() {
	tty, ok := a.screen.Tty()
	if !ok {
		a.image.pendingEmit = false
		a.image.pendingDelete = false
		return
	}
	if a.image.pendingDelete {
		a.image.pendingDelete = false
		writeKittyDelete(tty)
	}
	if !a.image.pendingEmit {
		return
	}
	a.image.pendingEmit = false
	p := a.image.last
	_, _ = fmt.Fprintf(tty, "\x1b[?2026h\x1b7\x1b[%d;%dH%s\x1b8\x1b[?2026l", p.Y+1, p.X+1, p.Payload)
}

func writeKittyDelete(w io.Writer) {
	_, _ = fmt.Fprintf(w, "\x1b_Ga=d,d=I,i=%d\x1b\\", previewpanel.KittyGraphicsImageID)
}

// resetImageOverlay unlocks any locked region, deletes a Kitty image if needed, and clears state.
func (a *App) resetImageOverlay() {
	if a.image.lastSet {
		if a.image.last.Protocol == previewpanel.ImageProtocolKitty {
			if tty, ok := a.screen.Tty(); ok {
				writeKittyDelete(tty)
			}
		}
		a.screen.LockRegion(a.image.last.X, a.image.last.Y, a.image.lastCols, a.image.lastRows, false)
	}
	a.image = imageOverlay{}
}
