package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/clipboard"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) toggleCopyMenu() {
	if a.copyMenuOpen() && !a.model.LeaderMenu.CopyGit {
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
	_, _, dirPath, _, hasTarget := a.copyTargets()
	for _, e := range entries {
		label := e.Label
		disabled := false
		if e.ActionID == keymap.ActionClipboardGitMenu {
			// Same submenu marker as the user menu (userMenuLeaderMenuItems).
			label += " " + string(a.styles.IconTreeExpand())
			disabled = hasTarget && gitignore.ValidWorkTreeRoot(dirPath) == ""
		}
		if e.ActionID == keymap.ActionClipboardCopyFileContents {
			disabled = !a.fullscreenTextPreviewOpen()
		}
		items = append(items, ui.LeaderMenuItem{Key: e.Key, Label: label, Disabled: disabled})
		actions = append(actions, e.ActionID)
	}
	a.openLeaderMenuDispatch(items, actions, ui.LeaderMenuCopy, "Copy menu", a.dispatchActionLikeKeyboardShortcut)
}

// openCopyGitMenu opens the "Relative to git root" submenu, or warns when the
// target directory is not in a git work tree.
func (a *App) openCopyGitMenu() {
	_, _, dirPath, _, ok := a.copyTargets()
	if !ok || a.keys == nil {
		return
	}
	if gitignore.ValidWorkTreeRoot(dirPath) == "" {
		a.setTransientMessage("Copy: not in a git repository", ui.MessageUrgencyWarn)
		return
	}
	var items []ui.LeaderMenuItem
	var actions []string
	for _, e := range a.keys.CopyMenuGitEntries() {
		items = append(items, ui.LeaderMenuItem{Key: e.Key, Label: e.Label})
		actions = append(actions, e.ActionID)
	}
	a.openLeaderMenuDispatch(items, actions, ui.LeaderMenuCopy, "Copy menu", a.dispatchActionLikeKeyboardShortcut)
	a.model.LeaderMenu.CopyGit = true
}

// singleCopyTarget builds the paths/entries/dirPath triple for a copy-menu
// invocation targeting one file (preview, compare, dedup selections).
func singleCopyTarget(path string) ([]string, []localfs.Entry, string) {
	return []string{canonicalTargetPath(path)}, []localfs.Entry{{Name: filepath.Base(path)}}, filepath.Dir(path)
}

// copyTargets resolves the paths/entries/dirPath the copy actions operate on
// for the current view. ok is false when there is no target; warn then holds
// the message to show (empty for silent).
func (a *App) copyTargets() (paths []string, entries []localfs.Entry, dirPath, warn string, ok bool) {
	switch a.model.ViewMode {
	case ui.ViewBrowser:
		p := a.activePanel()
		return panelTargetPaths(p), panelTargetEntries(p), p.PathString(), "", true
	case ui.ViewFilePreview:
		path := a.model.FullscreenFilePreview.Path
		if path == "" {
			return nil, nil, "", "", false
		}
		paths, entries, dirPath = singleCopyTarget(path)
		return paths, entries, dirPath, "", true
	case ui.ViewCompare:
		path, _, found := a.compareCtrl.SelectedColumnTarget()
		if !found {
			return nil, nil, "", "Copy: no file selected", false
		}
		paths, entries, dirPath = singleCopyTarget(path)
		return paths, entries, dirPath, "", true
	case ui.ViewDedup:
		path, _, found := a.dedupCtrl.SelectedPinTarget()
		if !found {
			return nil, nil, "", "Copy: no file selected", false
		}
		paths, entries, dirPath = singleCopyTarget(path)
		return paths, entries, dirPath, "", true
	}
	return nil, nil, "", "", false
}

// fullscreenTextPreviewOpen reports whether the F3 fullscreen preview shows a
// text file (not an image or a directory), the only place file contents copy.
func (a *App) fullscreenTextPreviewOpen() bool {
	fp := &a.model.FullscreenFilePreview
	return a.model.ViewMode == ui.ViewFilePreview && fp.Path != "" && !fp.IsDir && fp.ImagePayload == ""
}

// readTextFileForClipboard reads path for clipboard.copy-file-contents, refusing
// files over the preview read cap or containing NUL bytes (binary).
func readTextFileForClipboard(path string) (string, string) {
	info, err := os.Stat(path)
	if err != nil {
		return "", "Copy: cannot read file"
	}
	if info.Size() > config.DefaultMaxPreviewBytes {
		return "", "Copy: file too large"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "Copy: cannot read file"
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", "Copy: not a text file"
	}
	return string(data), ""
}

func (a *App) copyToClipboard(actionID string) {
	if actionID == keymap.ActionClipboardCopyFileContents {
		a.copyFileContentsToClipboard()
		return
	}
	paths, entries, dirPath, warn, ok := a.copyTargets()
	if !ok {
		if warn != "" {
			a.setTransientMessage(warn, ui.MessageUrgencyWarn)
		}
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
	case keymap.ActionClipboardCopyGitFilePath, keymap.ActionClipboardCopyGitDirPath:
		root := gitignore.ValidWorkTreeRoot(dirPath)
		if root == "" {
			a.setTransientMessage("Copy: not in a git repository", ui.MessageUrgencyWarn)
			return
		}
		if actionID == keymap.ActionClipboardCopyGitDirPath {
			dirs := make([]string, 0, len(paths))
			for _, p := range paths {
				dirs = append(dirs, filepath.Dir(p))
			}
			if len(dirs) == 0 {
				dirs = []string{dirPath}
			}
			paths = dirs
		}
		rel := make([]string, 0, len(paths))
		seen := make(map[string]struct{}, len(paths))
		for _, p := range paths {
			r, err := filepath.Rel(root, p)
			if err != nil {
				continue
			}
			if _, dup := seen[r]; dup {
				continue
			}
			seen[r] = struct{}{}
			rel = append(rel, r)
		}
		text = strings.Join(rel, "\n")
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

// copyFileContentsToClipboard copies the fullscreen-previewed text file verbatim.
// ponytail: synchronous read, bounded by DefaultMaxPreviewBytes like the preview itself.
func (a *App) copyFileContentsToClipboard() {
	if !a.fullscreenTextPreviewOpen() {
		a.setTransientMessage("Copy: file contents only from the fullscreen text preview", ui.MessageUrgencyWarn)
		return
	}
	path := a.model.FullscreenFilePreview.Path
	text, warn := readTextFileForClipboard(path)
	if warn != "" {
		a.setTransientMessage(warn, ui.MessageUrgencyWarn)
		return
	}
	if err := clipboard.Set(text); err != nil {
		a.setTransientMessage("Copy failed: no clipboard tool available", ui.MessageUrgencyWarn)
		return
	}
	a.setTransientMessage("Copied contents of "+filepath.Base(path), ui.MessageUrgencyInfo)
}
