package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/editor"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// externalEditorRunner runs the external editor on path; NewWithOptions copies it into App.runExternalEditor.
// When elevate is true and path is not writable, the editor runs via sudoedit.
var externalEditorRunner = func(ctx context.Context, path string, elevate bool) error {
	argv, env, elevated, err := editor.Command(editor.ResolveEditor(), path, elevate)
	if err != nil {
		return err
	}
	if elevated {
		fmt.Fprintf(os.Stderr, "pc: %s is not writable, editing via sudoedit\n", path)
	}
	dir := filepath.Dir(path)
	return editor.RunInteractive(ctx, argv, env, dir)
}

func (a *App) withTerminalReleased(fn func() error) error {
	// An Überzug++ window is not part of the terminal, so it would float over the subprocess.
	a.resetImageOverlay()
	// The child must not receive raw mouse reports; re-enabled once the TTY is back.
	defer a.releaseMouse()()
	// Suspend/Resume release the TTY for a subprocess; Fini/Init must not be used
	// here because Fini closes the TTY once (sync.Once) and breaks a second launch.
	if err := a.screen.Suspend(); err != nil {
		return fmt.Errorf("suspend terminal: %w", err)
	}
	resumed := false
	doResume := func() error {
		if resumed {
			return nil
		}
		resumed = true
		return a.screen.Resume()
	}
	defer func() { _ = doResume() }()
	runErr := fn()
	if resumeErr := doResume(); resumeErr != nil {
		if runErr != nil {
			return fmt.Errorf("resume terminal: %w (editor: %v)", resumeErr, runErr)
		}
		return fmt.Errorf("resume terminal: %w", resumeErr)
	}
	// Resume re-engages a cleared alt-screen; flush even when the logical buffer is unchanged
	// (ScreenRenderHashCache would otherwise skip Show and leave a blank terminal).
	a.resetImageOverlay()
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
		return a.runExternalEditor(context.Background(), path, a.config.Editor.Elevate)
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
	a.previewCtrl.CloseFilePreviewFullscreen()
	if err := a.activePanel().Refresh(a.activeViewportRows()); err != nil {
		a.setErrorMessage("Edit", err)
	}
}

func (a *App) resolveLocalEditFilePath() (string, error) {
	if a.model.ViewMode != ui.ViewBrowser {
		return "", fmt.Errorf("edit is only available in the file browser")
	}
	p := a.activePanel()
	if !p.Path.IsLocal() {
		return "", errors.New(pathloc.Unavailable("edit", p.Path))
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
	if a.model.ActiveSubFocus == ui.SubFocusSelectedStrip && p.SelectedStripCount() > 0 {
		selPath, ok := p.SelectedPathAtStripIndex(p.SelectedStripCursor)
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
