package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/editor"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// externalEditorRunner runs the external editor on path. Tests may replace this.
var externalEditorRunner = func(ctx context.Context, path string) error {
	argv, err := editor.EditorArgv(editor.ResolveEditor(), path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	return editor.RunInteractive(ctx, argv, dir)
}

func (a *App) withTerminalReleased(fn func() error) error {
	// Suspend/Resume release the TTY for a subprocess; Fini/Init must not be used
	// here because Fini closes the TTY once (sync.Once) and breaks a second launch.
	if err := a.screen.Suspend(); err != nil {
		return fmt.Errorf("suspend terminal: %w", err)
	}
	runErr := fn()
	if resumeErr := a.screen.Resume(); resumeErr != nil {
		if runErr != nil {
			return fmt.Errorf("resume terminal: %w (editor: %v)", resumeErr, runErr)
		}
		return fmt.Errorf("resume terminal: %w", resumeErr)
	}
	// Resume re-engages a cleared alt-screen; flush even when the logical buffer is unchanged
	// (ScreenRenderHashCache would otherwise skip Show and leave a blank terminal).
	a.lastScreenContentHash = 0
	a.screen.HideCursor()
	a.render()
	return runErr
}

func (a *App) openFileInExternalEditor(path string) error {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return fmt.Errorf("no file path")
	}
	return a.withTerminalReleased(func() error {
		return externalEditorRunner(context.Background(), path)
	})
}

func (a *App) editActiveFile() {
	path, err := a.resolveLocalEditFilePath()
	if err != nil {
		a.setErrorMessage("Edit", err)
		return
	}
	if err := a.openFileInExternalEditor(path); err != nil {
		a.setErrorMessage("Edit", err)
		return
	}
	if a.model.ViewMode == ui.ViewBrowser {
		if err := a.activePanel().Refresh(a.activeViewportRows()); err != nil {
			a.setErrorMessage("Edit", err)
		}
	}
}

func (a *App) editFullscreenPreviewFile() {
	a.commandsMu.RLock()
	st := a.model.FullscreenFilePreview
	a.commandsMu.RUnlock()
	if !st.Open || st.Path == "" {
		a.setTransientMessage("Edit: no file", ui.MessageUrgencyWarn)
		return
	}
	path := filepath.Clean(st.Path)
	if err := a.openFileInExternalEditor(path); err != nil {
		a.setErrorMessage("Edit", err)
		return
	}
	a.closeFilePreviewFullscreen()
	if err := a.activePanel().Refresh(a.activeViewportRows()); err != nil {
		a.setErrorMessage("Edit", err)
	}
}

func (a *App) resolveLocalEditFilePath() (string, error) {
	if a.model.ViewMode != ui.ViewBrowser {
		return "", fmt.Errorf("edit is only available in the file browser")
	}
	p := a.activePanel()
	if p.Path.IsRemote() {
		return "", fmt.Errorf("edit is not available on remote panels")
	}
	path, mode := a.editTargetFilePath(p)
	switch mode {
	case editTargetNone:
		return "", fmt.Errorf("no file selected")
	case editTargetDir:
		return "", fmt.Errorf("cannot edit a directory")
	case editTargetMissing:
		return "", fmt.Errorf("file not found")
	case editTargetFile:
		return path, nil
	default:
		return "", fmt.Errorf("no file selected")
	}
}

type editTargetMode int

const (
	editTargetNone editTargetMode = iota
	editTargetFile
	editTargetDir
	editTargetMissing
)

func (a *App) editTargetFilePath(p *panel.State) (path string, mode editTargetMode) {
	if a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip && p.SelectionsStripCount() > 0 {
		selPath, ok := p.SelectedPathAtStripIndex(p.SelectionsStripCursor)
		if !ok {
			return "", editTargetNone
		}
		return classifyEditPath(filepath.Clean(selPath))
	}
	entry, ok := p.CurrentEntry()
	if !ok {
		return "", editTargetNone
	}
	if entry.Type == localfs.EntryDirectory {
		return "", editTargetDir
	}
	return classifyEditPath(filepath.Clean(entry.Path))
}

func classifyEditPath(path string) (string, editTargetMode) {
	if path == "" || path == "." {
		return "", editTargetNone
	}
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, editTargetMissing
		}
		return path, editTargetNone
	}
	if fi.IsDir() {
		return path, editTargetDir
	}
	return path, editTargetFile
}
