package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// resolveExistingStartPath expands ~, makes the path absolute, and stats it.
// Missing paths return a clear error (no fallback to parent).
func resolveExistingStartPath(raw, homeDir string) (string, os.FileInfo, error) {
	path, err := expandUserPath(raw, homeDir)
	if err != nil {
		return "", nil, err
	}
	if !filepath.IsAbs(path) {
		path, err = filepath.Abs(path)
		if err != nil {
			return "", nil, err
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, fmt.Errorf("%s: no such file or directory", path)
		}
		return "", nil, err
	}
	return path, info, nil
}

type startPathKind int

const (
	startPathDir startPathKind = iota
	startPathFile
)

type resolvedStartPath struct {
	path string
	kind startPathKind
}

func resolveStartPathArgs(rawPaths []string, homeDir string) ([]resolvedStartPath, error) {
	if len(rawPaths) > 2 {
		return nil, fmt.Errorf("too many path arguments")
	}
	out := make([]resolvedStartPath, 0, len(rawPaths))
	for _, raw := range rawPaths {
		path, info, err := resolveExistingStartPath(raw, homeDir)
		if err != nil {
			return nil, err
		}
		kind := startPathFile
		if info.IsDir() {
			kind = startPathDir
		}
		out = append(out, resolvedStartPath{path: path, kind: kind})
	}
	if len(out) == 2 && out[0].kind == startPathFile && out[1].kind == startPathFile {
		return nil, fmt.Errorf("both arguments are files; provide at most one file")
	}
	return out, nil
}

// applyStartPaths navigates panels from CLI path arguments.
//
//	1 directory  → primary panel
//	1 file       → primary parent + select, then fullscreen preview
//	2 paths      → primary then secondary; a single file side enables Quick View
func (a *App) applyStartPaths(rawPaths []string) error {
	resolved, err := resolveStartPathArgs(rawPaths, a.model.UserHomeDir)
	if err != nil {
		return err
	}
	if len(resolved) == 0 {
		return nil
	}

	if len(resolved) == 1 {
		r := resolved[0]
		if r.kind == startPathDir {
			return a.model.Primary.NavigateTo(r.path, "", a.panelViewportRows(ui.PrimaryPanel))
		}
		if err := localfs.CheckFilePreviewable(r.path); err != nil &&
			!errors.Is(err, localfs.ErrFilePreviewImage) &&
			!errors.Is(err, localfs.ErrFilePreviewMedia) {
			return fmt.Errorf("preview %s: %w", r.path, err)
		}
		if err := a.model.Primary.NavigateTo(
			filepath.Dir(r.path),
			filepath.Base(r.path),
			a.panelViewportRows(ui.PrimaryPanel),
		); err != nil {
			return err
		}
		a.model.ActivePanel = ui.PrimaryPanel
		a.launchedFileViewer = true
		return a.previewCtrl.OpenFullscreenFilePreviewAt(r.path)
	}

	filePanel := -1
	for i, r := range resolved {
		panelID := ui.PrimaryPanel
		if i == 1 {
			panelID = ui.SecondaryPanel
		}
		p := a.panelByID(panelID)
		vr := a.panelViewportRows(panelID)
		if r.kind == startPathDir {
			if err := p.NavigateTo(r.path, "", vr); err != nil {
				return err
			}
			continue
		}
		if err := p.NavigateTo(filepath.Dir(r.path), filepath.Base(r.path), vr); err != nil {
			return err
		}
		filePanel = panelID
	}
	if filePanel >= 0 {
		a.model.ActivePanel = filePanel
		a.model.QuickViewEnabled = true
		a.model.QuickViewPanel = filePanel
		a.previewCtrl.ApplyQuickViewPreviewImmediately()
	}
	return nil
}
