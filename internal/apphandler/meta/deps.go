// Package meta owns the async per-entry metadata (meta.toml) feature and the Columns dialog:
// column profiles, worker-pool command dispatch per panel, result caching, and the F9
// edit-config-from-dialog flow.
package meta

import (
	"context"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/columnprofile"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/sched"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Deps wires the meta handler at app construction.
type Deps struct {
	Host      Host
	Screen    tcell.Screen
	Model     *ui.Model
	Config    config.Config
	ConfigDir string
	// Providers are the Go column providers offered beside the meta.toml entries.
	Providers []*metacmds.Provider
	// KeysColumnsDialog is the [dialog.columns] overlay (column profiles); nil means no binding.
	KeysColumnsDialog *keymap.Map
	// KeysDialogInput edits the Columns dialog's text inputs.
	KeysDialogInput *keymap.Map
	// Profiles are the saved column profiles, read at startup.
	Profiles []columnprofile.Profile
}

// Handler owns meta.toml command dispatch, the Columns dialog and the application of column
// profiles.
type Handler struct {
	host      Host
	screen    tcell.Screen
	model     *ui.Model
	config    config.Config
	configDir string
	providers []*metacmds.Provider
	// keysColumnsDialog resolves the dialog overlay chords before the widget engine sees the key.
	keysColumnsDialog *keymap.Map
	keysDialogInput   *keymap.Map
	// profiles are the saved column profiles: read at startup and whenever the dialog opens or
	// changes them, so the profile cycle needs no I/O.
	profiles []columnprofile.Profile

	// activeEntries holds ordered entry names for active meta columns per panel.
	activeEntries [2][]string
	// navPath holds the last panel path for which meta was run (used to detect navigation).
	navPath [2]string
	// inIndexView is whether the panel was in an index view (fileindex:// or Browse with
	// fileindex) at the last dir-change check, so default columns switch on only when it enters one.
	inIndexView [2]bool
	// autoColumns is non-nil while the panel's columns are [fileindex].default_profile, applied
	// on entering an index view (cleared when the user changes columns). It holds what leaving
	// the index views restores.
	autoColumns [2]*columnSnapshot
	// sortCheck holds the meta names active before the last column change while its load is
	// pending, so HandleLoad can apply sort_on_activation to the newly active ones.
	sortCheck [2]map[string]bool
	// run is the in-flight meta run per panel (nil if none), kept so ReconcileForPanel can
	// dispatch only newly appeared entries into it.
	run [2]*metaRun
	// runGen is a monotonically increasing generation counter per panel for meta runs.
	// Workers carry the generation; stale (cancelled) results are discarded by the wake handler.
	runGen [2]uint64
	// treeWalked is the TreeChildrenGen at which ReconcileForPanel last walked the panel's loaded
	// tree children (or a run covered them), gating the allocating walk.
	treeWalked [2]uint64
	// loadGen is a monotonically increasing generation counter per panel for async meta file loads.
	// Stale loads (navigated away before load finished) are discarded by the wake handler.
	loadGen [2]uint64
	// loadPending is true while an async meta.toml load is in flight for that panel.
	loadPending [2]bool
	// renderDebounce paces each panel's flushes at config.MetaLayoutInterval. The timer callback
	// only posts RenderFlushPayload; HandleRenderFlush consumes it on the event loop.
	renderDebounce [2]sched.Debouncer
	// resortPending marks panels whose SortMeta sort target column received a new value since
	// the last render flush; HandleRenderFlush signals those panels once, coalesced with the
	// repaint itself.
	resortPending [2]bool
	// pendingReveal fires once per busy period after [panels].pending_marker_delay_ms and posts
	// PendingRevealPayload; HandlePendingReveal then shows the markers (clears PendingHidden).
	pendingReveal [2]sched.Debouncer
	// layoutGuess holds each panel's last measured column layouts by EntryName, the Guess of its
	// next run.
	layoutGuess [2]map[string]ui.MetaLayoutGuess
	// fitWatch tracks the drawn columns per panel so a column lost to incoming results is
	// reported (reportLostColumns).
	fitWatch [2]columnFitWatch
	// gone holds rows whose path a filesystem change reported gone, per panel; their cells leave
	// once the listing no longer holds them (pruneGone).
	gone [2]map[string]struct{}
	// goneDirty is set when new reports arrived; goneChecked is the listing gone was last checked
	// against. Together they keep pruneGone from re-reading an unchanged listing.
	goneDirty   [2]bool
	goneChecked [2]goneListing
	// providerCancel cancels the panel's RefreshProviders in flight, so a newer one replaces it.
	providerCancel [2]context.CancelFunc
	// cache stores computed meta results by [cmdName][absPath] for entries with cache = true.
	// Nil until first caching write. Protected by cacheMu.
	cache   map[string]map[string]string
	cacheMu sync.RWMutex
}

// New constructs a meta Handler.
func New(deps Deps) *Handler {
	return &Handler{
		host:      deps.Host,
		screen:    deps.Screen,
		model:     deps.Model,
		config:    deps.Config,
		configDir: strings.TrimSpace(deps.ConfigDir),
		providers: deps.Providers,

		keysColumnsDialog: deps.KeysColumnsDialog,
		keysDialogInput:   deps.KeysDialogInput,
		profiles:          deps.Profiles,
	}
}

// WakePayload wakes PollEvent after a meta background worker completes one entry.
// It carries the result so the event handler can apply it on the main goroutine, preventing
// concurrent map access between background workers and the render path.
type WakePayload struct {
	PanelID   int
	EntryName string
	Path      string
	Value     string
	// Values, when non-nil, is a batch (path -> value) applied in one pass instead of Path/Value;
	// a provider column posts one event for the whole listing.
	Values map[string]string
	Gen    uint64
}

// RenderFlushPayload is posted by a panel's paced meta render timer to coalesce frequent
// WakePayload events; the event loop publishes the panel's layout and repaints.
type RenderFlushPayload struct{ PanelID int }

// PendingRevealPayload is posted by the pendingReveal timer when a meta command has been running
// for the pending marker delay; Gen is the panel's run generation when the timer was armed.
type PendingRevealPayload struct {
	PanelID int
	Gen     uint64
}

// LoadPayload carries the result of an async meta.toml resolve+load back to the UI goroutine.
type LoadPayload struct {
	PanelID     int
	LoadGen     uint64
	Path        string // panel path when the load started
	MF          *metacmds.MetaFile
	ActiveNames []string
	Warns       []string
	LoadErr     string // non-empty if LoadFile failed
}

// ExecFailedPayload notifies the main goroutine once per meta run when a shell command fails,
// or when a Go column provider (Provider set) returns an error (Stderr).
type ExecFailedPayload struct {
	PanelID     int
	Gen         uint64
	Provider    string // name of the failed Go column provider; empty for a shell command
	ExitCode    int
	Stderr      string
	ConfCmd     string // original template from meta.toml
	ExpandedCmd string // substituted command actually run
}

// runFailure wraps a command execution failure with details for the messages log.
type runFailure struct {
	ExitCode    int
	Stderr      string
	ConfCmd     string // original template from meta.toml (e.g. "git log %f")
	ExpandedCmd string // substituted command actually run (e.g. "git log /path/to/file")
	err         error
}

func (e *runFailure) Error() string { return e.err.Error() }
func (e *runFailure) Unwrap() error { return e.err }

// dispatchItem holds a single entry selected for meta command execution, with the directory it
// is listed in.
type dispatchItem struct {
	entry localfs.Entry
	dir   string
	cmd   string
}

// dirEntries is a listing directory and its entries: the panel root, or one loaded tree node.
type dirEntries struct {
	dir     string
	entries []localfs.Entry
}

// metaRun is one in-flight meta run for a panel: its context, cancel func and resolved command defs.
type metaRun struct {
	ctx    context.Context
	cancel context.CancelFunc
	defs   []metacmds.MetaEntry
}
