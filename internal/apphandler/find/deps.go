package find

import (
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	findpkg "github.com/paranoidi/paras-commander/internal/find"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Deps wires the find handler at app construction.
type Deps struct {
	Host   Host
	Screen tcell.Screen
	Model  *ui.Model
	Config config.Config
	Keys   *keymap.Map
}

// Handler owns find-dialog indexing and navigation.
type Handler struct {
	host   Host
	screen tcell.Screen
	model  *ui.Model
	config config.Config
	keys   *keymap.Map

	sessionMu      sync.Mutex
	walks          map[string]*walk
	batchCh        chan []findpkg.Entry
	indexedPaths   map[string]struct{}
	completedRoots map[string]struct{}
}

type walk struct {
	root string
	sess *findpkg.Session
}
