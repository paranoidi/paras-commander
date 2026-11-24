package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/clipboard"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) toggleCopyMenu() {
	if a.leaderMenuOpen(ui.LeaderMenuCopy) && !a.model.LeaderMenu.CopyGit {
		a.closeLeaderMenu()
		return
	}
	switch a.model.ViewMode {
	case ui.ViewBrowser, ui.ViewFilePreview, ui.ViewCompare, ui.ViewDedup, ui.ViewUnique:
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
	paths, _, dirPath, _, hasTarget := a.copyTargets()
	for _, e := range entries {
		label := e.Label
		disabled := false
		if e.ActionID == keymap.ActionClipboardGitMenu {
			// Same submenu marker as the user menu (userMenuLeaderMenuItems).
			label += " " + string(a.styles.IconTreeExpand())
			disabled = hasTarget && !gitAvailable(paths, dirPath)
		}
		if e.ActionID == keymap.ActionClipboardCopyFileContents {
			_, ok := a.fileContentsCopyTarget()
			disabled = !ok
		}
		items = append(items, ui.LeaderMenuItem{Key: e.Key, Label: label, Disabled: disabled})
		actions = append(actions, e.ActionID)
	}
	a.openLeaderMenuDispatch(items, actions, ui.LeaderMenuCopy, "Copy menu", a.dispatchActionLikeKeyboardShortcut)
}

// openCopyGitMenu opens the "Relative to git root" submenu, or warns when the
// target directory is not in a git work tree.
func (a *App) openCopyGitMenu() {
	paths, _, dirPath, _, ok := a.copyTargets()
	if !ok || a.keys == nil {
		return
	}
	if !gitAvailable(paths, dirPath) {
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
	return []string{pathloc.Canonical(path)}, []localfs.Entry{{Name: filepath.Base(path)}}, pathloc.Dir(path)
}

// copyTargets resolves the paths/entries/dirPath the copy actions operate on
// for the current view. ok is false when there is no target; warn then holds
// the message to show (empty for silent).
func (a *App) copyTargets() (paths []string, entries []localfs.Entry, dirPath, warn string, ok bool) {
	switch a.model.ViewMode {
	case ui.ViewBrowser:
		p := a.focusedPanel()
		return panelTargetPaths(p), panelTargetEntries(p), p.CursorDir().String(), "", true
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
	case ui.ViewUnique:
		path, _, found := a.uniqueCtrl.Target()
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

// gitAvailable reports whether the copy targets can be made git-relative: any target's directory,
// or dirPath (the cursor directory) when there are no targets, is inside a work tree.
func gitAvailable(paths []string, dirPath string) bool {
	if len(paths) == 0 {
		return gitignore.ValidWorkTreeRoot(dirPath) != ""
	}
	for _, p := range paths {
		if gitignore.ValidWorkTreeRoot(pathloc.Dir(p)) != "" {
			return true
		}
	}
	return false
}

// fileContentsCopyTarget is the file clipboard.copy-file-contents would copy: the text file shown in
// the F3 fullscreen preview (not a directory, no image payload), or in the browser the active panel's
// cursor entry when it is a regular local file that is neither media nor a listable archive. Name
// based, so the menu's disabled state costs no IO; readTextFileForClipboard refuses binary and
// oversized files at activation.
func (a *App) fileContentsCopyTarget() (string, bool) {
	switch a.model.ViewMode {
	case ui.ViewFilePreview:
		fp := &a.model.FullscreenFilePreview
		return fp.Path, fp.Path != "" && !fp.IsDir && fp.ImagePayload == ""
	case ui.ViewBrowser:
		e, ok := a.focusedPanel().CurrentEntry()
		if !ok || e.Name == ".." || e.Type != localfs.EntryFile {
			return "", false
		}
		if loc, err := pathloc.Parse(e.Path); err != nil || !loc.IsLocal() {
			return "", false
		}
		if localfs.IsMediaPath(e.Path) || archive.ListableName(e.Path) {
			return "", false
		}
		return e.Path, true
	}
	return "", false
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
		// Targets can live in different folders (tree layout, selection across folders), so each
		// is made relative to the work tree holding it; targets outside any repository are skipped.
		dirAction := actionID == keymap.ActionClipboardCopyGitDirPath
		rel := make([]string, 0, len(paths))
		seen := make(map[string]struct{}, len(paths))
		inWorkTree := false // some target's directory is inside a work tree
		// add makes target relative to the work tree holding dir.
		add := func(dir, target string) {
			root := gitignore.ValidWorkTreeRoot(dir)
			if root == "" {
				return
			}
			inWorkTree = true
			r, err := pathloc.Rel(root, target)
			if err != nil {
				return
			}
			if _, dup := seen[r]; dup {
				return
			}
			seen[r] = struct{}{}
			rel = append(rel, r)
		}
		for _, p := range paths {
			dir := pathloc.Dir(p)
			if dirAction {
				add(dir, dir)
			} else {
				add(dir, p)
			}
		}
		if len(paths) == 0 {
			// No targets: the cursor directory (the file-path action has nothing to make relative).
			if dirAction {
				add(dirPath, dirPath)
			} else {
				inWorkTree = gitignore.ValidWorkTreeRoot(dirPath) != ""
			}
		}
		if len(rel) == 0 && !inWorkTree {
			a.setTransientMessage("Copy: not in a git repository", ui.MessageUrgencyWarn)
			return
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
	preview := textutil.TruncateBannerRunes(strings.ReplaceAll(text, "\n", ", "), textutil.BannerMaxRunes)
	a.setClipboard(text, "Copied: "+preview)
}

// setClipboard puts text on the system clipboard and toasts done, or a warning when no clipboard
// tool is available.
func (a *App) setClipboard(text, done string) {
	a.setClipboardToast(text, done, ui.MessageUrgencyInfo)
}

// setClipboardToast is setClipboard with the urgency of the done toast.
func (a *App) setClipboardToast(text, done string, urgency ui.MessageUrgency) {
	if err := clipboard.Set(text); err != nil {
		a.setTransientMessage("Copy failed: no clipboard tool available", ui.MessageUrgencyWarn)
		return
	}
	a.setTransientMessage(done, urgency)
}

// copyPreviewSelection copies the text selected with the mouse in the fullscreen preview. The toast
// quotes a selection shorter than config.MouseCopyToastMaxRunes and counts the lines of a longer one.
func (a *App) copyPreviewSelection(text string) {
	if text == "" {
		return
	}
	done := "Copied: " + strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) >= config.MouseCopyToastMaxRunes || strings.TrimSpace(text) == "" {
		lines := strings.Count(strings.TrimSuffix(text, "\n"), "\n") + 1
		done = fmt.Sprintf("Copied %d %s", lines, jobbridge.Plural(lines, "line", "lines"))
	}
	a.setClipboardToast(text, done, ui.MessageUrgencySuccess)
}

// copyFileContentsToClipboard copies the previewed or cursor text file verbatim.
// ponytail: synchronous read, bounded by DefaultMaxPreviewBytes like the preview itself.
func (a *App) copyFileContentsToClipboard() {
	path, ok := a.fileContentsCopyTarget()
	if !ok {
		a.setTransientMessage("Copy: no text file under the cursor", ui.MessageUrgencyWarn)
		return
	}
	text, warn := readTextFileForClipboard(path)
	if warn != "" {
		a.setTransientMessage(warn, ui.MessageUrgencyWarn)
		return
	}
	a.setClipboard(text, "Copied contents of "+filepath.Base(path))
}
