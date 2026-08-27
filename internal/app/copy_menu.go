package app

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/clipboard"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) openCopyMenu() {
	if a.model.ViewMode != ui.ViewBrowser {
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

func (a *App) copyToClipboard(actionID string) {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	p := a.activePanel()
	paths := panelTargetPaths(p)
	entries := panelTargetEntries(p)

	var text string
	switch actionID {
	case keymap.ActionClipboardCopyFileURL:
		if len(paths) == 0 {
			a.setTransientMessage("Copy: no file selected", ui.MessageUrgencyWarn)
			return
		}
		text = clipboard.BuildFileURLs(paths)
	case keymap.ActionClipboardCopyDirURL:
		text = clipboard.BuildDirURLs(paths, p.PathString())
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
