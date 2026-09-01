// Package pin owns the ad-hoc pin list and the Pin dialog (fuzzy-filterable list of
// pinned files/directories, opened/toggled from the browser panel, Compare, and Dedup).
package pin

import (
	"github.com/gdamore/tcell/v2"
	comparectrl "github.com/paranoidi/paras-commander/internal/apphandler/compare"
	dedupctrl "github.com/paranoidi/paras-commander/internal/apphandler/dedup"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Deps wires the pin handler at app construction.
type Deps struct {
	Host   Host
	Screen tcell.Screen
	Model  *ui.Model

	// KeysPinDialog is the [dialog.pin] keymap overlay (view/open-primary/open-secondary/
	// remove chords). KeysDialogInput is the [dialog.input] overlay for the query field.
	KeysPinDialog   *keymap.Map
	KeysDialogInput *keymap.Map

	// Compare / Dedup / Preview are already-constructed sibling controllers, taken directly
	// (not through Host) to resolve the compare/dedup pin target and open the F3 fullscreen
	// preview, mirroring apphandler/dialog.Deps.Jobs/Commands/Preview/Dedup.
	Compare *comparectrl.Handler
	Dedup   *dedupctrl.Handler
	Preview *previewctrl.Handler
}

// Handler owns the ad-hoc pin list and its dialog.
type Handler struct {
	host            Host
	screen          tcell.Screen
	model           *ui.Model
	keysPinDialog   *keymap.Map
	keysDialogInput *keymap.Map
	compare         *comparectrl.Handler
	dedup           *dedupctrl.Handler
	preview         *previewctrl.Handler

	// reopenAfterPreview is set when the F3 fullscreen preview was launched from the Pin
	// dialog (ViewSelected), so ReopenAfterPreviewClose knows to reopen the dialog, restored
	// exactly, once the preview later closes, instead of dropping back to plain panel
	// browsing.
	reopenAfterPreview bool
}

// New constructs a Handler.
func New(d Deps) *Handler {
	return &Handler{
		host:            d.Host,
		screen:          d.Screen,
		model:           d.Model,
		keysPinDialog:   d.KeysPinDialog,
		keysDialogInput: d.KeysDialogInput,
		compare:         d.Compare,
		dedup:           d.Dedup,
		preview:         d.Preview,
	}
}
