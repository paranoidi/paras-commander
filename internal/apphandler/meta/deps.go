// Package meta owns the async per-entry metadata (meta.toml) feature: the checkbox
// picker dialog, worker-pool command dispatch per panel, result caching, and the
// F9 edit-config-from-dialog flow.
package meta

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Deps wires the meta handler at app construction.
type Deps struct {
	Host      Host
	Screen    tcell.Screen
	Model     *ui.Model
	Config    config.Config
	ConfigDir string
}

// Handler owns meta.toml command dispatch and the meta checkbox dialog.
type Handler struct {
	host      Host
	screen    tcell.Screen
	model     *ui.Model
	config    config.Config
	configDir string

	// activeEntries holds ordered entry names for active meta columns per panel.
	activeEntries [2][]string
	// navPath holds the last panel path for which meta was run (used to detect navigation).
	navPath [2]string
	// cancel holds the cancel function for the in-flight meta run per panel (nil if none).
	cancel [2]context.CancelFunc
	// runGen is a monotonically increasing generation counter per panel for meta runs.
	// Workers carry the generation; stale (cancelled) results are discarded by the wake handler.
	runGen [2]uint64
	// loadGen is a monotonically increasing generation counter per panel for async meta file loads.
	// Stale loads (navigated away before load finished) are discarded by the wake handler.
	loadGen [2]uint64
	// renderTimer debounces meta result renders; posted events call scheduleRenderDebounced
	// instead of rendering directly so burst results (large dirs) coalesce into few repaints.
	renderTimer *time.Timer
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
	Gen       uint64
}

// RenderFlushPayload is posted by the debounced meta render timer to coalesce frequent
// WakePayload renders into a single repaint (avoids one render per entry with large dirs).
type RenderFlushPayload struct{}

// LoadPayload carries the result of an async meta.toml resolve+load back to the UI goroutine.
type LoadPayload struct {
	PanelID     int
	LoadGen     uint64
	MF          *metacmds.MetaFile
	ActiveNames []string
	Warns       []string
	LoadErr     string // non-empty if LoadFile failed
}

// ExecFailedPayload notifies the main goroutine once per meta run when a shell command fails.
type ExecFailedPayload struct {
	PanelID     int
	Gen         uint64
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

// dispatchItem holds a single entry selected for meta command execution.
type dispatchItem struct {
	entry localfs.Entry
	cmd   string
}
