package app

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/clipboard"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) toggleCopyMenu() {
	if a.copyMenuOpen() {
		a.closeLeaderMenu()
		return
	}
	switch a.model.ViewMode {
	case ui.ViewBrowser, ui.ViewFilePreview, ui.ViewCompare, ui.ViewDedup:
	default:
		return
	}
	if a.keys == nil {
		return
	}
	entries := a.keys.CopyMenuEntries()
	if len(entries) == 0 {
		a.setTransientMessage("Copy menu: no entries configured", ui.MessageUrgencyWarn)
		return
	}
	var items []ui.LeaderMenuItem
	var actions []string
	for _, e := range entries {
		items = append(items, ui.LeaderMenuItem{Key: e.Key, Label: e.Label})
		actions = append(actions, e.ActionID)
	}
	a.openLeaderMenuDispatch(items, actions, false, true, "Copy menu", a.dispatchActionLikeKeyboardShortcut)
}

// singleCopyTarget builds the paths/entries/dirPath triple for a copy-menu
// invocation targeting one file (preview, compare, dedup selections).
func singleCopyTarget(path string) ([]string, []localfs.Entry, string) {
	return []string{canonicalTargetPath(path)}, []localfs.Entry{{Name: filepath.Base(path)}}, filepath.Dir(path)
}

func (a *App) copyToClipboard(actionID string) {
	var paths []string
	var entries []localfs.Entry
	var dirPath string
	switch a.model.ViewMode {
	case ui.ViewBrowser:
		p := a.activePanel()
		paths = panelTargetPaths(p)
		entries = panelTargetEntries(p)
		dirPath = p.PathString()
	case ui.ViewFilePreview:
		path := a.model.FullscreenFilePreview.Path
		if path == "" {
			return
		}
		paths, entries, dirPath = singleCopyTarget(path)
	case ui.ViewCompare:
		path, ok := a.compareCtrl.SelectedColumnPinTarget()
		if !ok {
			a.setTransientMessage("Copy: no file selected", ui.MessageUrgencyWarn)
			return
		}
		paths, entries, dirPath = singleCopyTarget(path)
	case ui.ViewDedup:
		path, _, ok := a.dedupCtrl.SelectedPinTarget()
		if !ok {
			a.setTransientMessage("Copy: no file selected", ui.MessageUrgencyWarn)
			return
		}
		paths, entries, dirPath = singleCopyTarget(path)
	default:
		return
	}

	var text string
	switch actionID {
	case keymap.ActionClipboardCopyFileURL:
		if len(paths) == 0 {
			a.setTransientMessage("Copy: no file selected", ui.MessageUrgencyWarn)
			return
		}
		text = clipboard.BuildFileURLs(paths)
	case keymap.ActionClipboardCopyDirURL:
		text = clipboard.BuildDirURLs(paths, dirPath)
		if text == "" {
			a.setTransientMessage("Copy: no directory available", ui.MessageUrgencyWarn)
			return
		}
	case keymap.ActionClipboardCopyFilename:
		if len(entries) == 0 {
			a.setTransientMessage("Copy: no file selected", ui.MessageUrgencyWarn)
			return
		}
		text = clipboard.BuildFilenames(entries)
	case keymap.ActionClipboardCopyFilenameWithoutExt:
		if len(entries) == 0 {
			a.setTransientMessage("Copy: no file selected", ui.MessageUrgencyWarn)
			return
		}
		text = clipboard.BuildFilenamesWithoutExt(entries)
	default:
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		a.setTransientMessage("Copy: nothing to copy", ui.MessageUrgencyWarn)
		return
	}
	if err := clipboard.Set(text); err != nil {
		a.setTransientMessage("Copy failed: no clipboard tool available", ui.MessageUrgencyWarn)
		return
	}
	preview := textutil.TruncateBannerRunes(strings.ReplaceAll(text, "\n", ", "), textutil.BannerMaxRunes)
	a.setTransientMessage("Copied: "+preview, ui.MessageUrgencyInfo)
}
