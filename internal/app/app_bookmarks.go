package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/bookmarks"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) openBookmarkDialog() {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return
	}
	if a.inQuickFilterUI() {
		a.activePanel().CancelFilter(a.activeViewportRows())
	}
	path, err := bookmarks.ResolveFile(a.config.Bookmarks.File, a.model.UserHomeDir)
	if err != nil {
		a.setErrorMessage("Bookmarks", err)
		return
	}
	marks, err := bookmarks.Load(path)
	if err != nil {
		a.setErrorMessage("Bookmarks", err)
		return
	}
	entries := make([]ui.BookmarkEntry, len(marks))
	for i := range marks {
		entries[i] = ui.BookmarkEntry{Name: marks[i].Name, Path: marks[i].Path, Line: marks[i].Line}
	}
	a.model.BookmarkDialog = ui.BookmarkDialogState{
		Open:      true,
		Query:     "",
		Entries:   entries,
		Focus:     0,
		Selected:  0,
		ListScroll: 0,
		MarksPath: path,
	}
	a.syncBookmarkDialogRanks()
}

func (a *App) closeBookmarkDialog() {
	a.model.BookmarkDialog = ui.BookmarkDialogState{}
}

func (a *App) syncBookmarkDialogRanks() {
	st := &a.model.BookmarkDialog
	if !st.Open {
		return
	}
	lines := make([]string, len(st.Entries))
	for i, e := range st.Entries {
		lines[i] = e.Line
	}
	q := search.Parse(st.Query)
	opts := search.Options{CaseInsensitive: a.config.CaseInsensitiveFilter}
	ranked := q.Rank(lines, opts)
	st.Ranked = make([]int, len(ranked))
	st.MatchRanges = make([][]search.Range, len(st.Entries))
	for i := range st.MatchRanges {
		st.MatchRanges[i] = nil
	}
	for i, r := range ranked {
		st.Ranked[i] = r.Index
		if r.Index >= 0 && r.Index < len(st.MatchRanges) {
			st.MatchRanges[r.Index] = r.Result.Ranges
		}
	}
	if st.Selected >= len(st.Ranked) {
		if len(st.Ranked) == 0 {
			st.Selected = 0
		} else {
			st.Selected = len(st.Ranked) - 1
		}
	}
	if st.Selected < 0 {
		st.Selected = 0
	}
	ui.EnsureBookmarkListScroll(st, a.bookmarkDialogListRows())
}

func (a *App) bookmarkDialogListRows() int {
	termW, termH := a.screen.Size()
	layout := a.layoutForTerminalSize(termW, termH)
	listH := layout.Height - 12
	switch {
	case listH > 18:
		listH = 18
	case listH < 4:
		listH = 4
	}
	dialogHeight := 9 + listH
	if dialogHeight > layout.Height-2 {
		listH = layout.Height - 2 - 9
		if listH < 4 {
			return 4
		}
	}
	return listH
}

func (a *App) activateBookmarkSelection() {
	st := &a.model.BookmarkDialog
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Entries) {
		return
	}
	path := filepath.Clean(st.Entries[entIdx].Path)
	p := a.activePanel()
	if err := a.navigatePanelToDirectory(a.model.ActivePanel, path, ""); err != nil {
		a.setErrorMessage("Bookmark", err)
		return
	}
	p.EnsureCursorVisible(a.activeViewportRows())
	a.closeBookmarkDialog()
	a.setTransientMessage(path, ui.MessageUrgencyInfo)
}

func (a *App) handleBookmarkDialogKey(event *tcell.EventKey) {
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModAlt {
		switch event.Rune() {
		case 'o', 'O':
			a.activateBookmarkSelection()
			return
		case 'c', 'C':
			a.closeBookmarkDialog()
			return
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeBookmarkDialog()
	case tcell.KeyEnter:
		switch a.model.BookmarkDialog.Focus {
		case 2:
			a.closeBookmarkDialog()
		default:
			a.activateBookmarkSelection()
		}
	case tcell.KeyTab:
		a.model.BookmarkDialog.Focus = (a.model.BookmarkDialog.Focus + 1) % 3
	case tcell.KeyBacktab:
		a.model.BookmarkDialog.Focus = (a.model.BookmarkDialog.Focus + 2) % 3
	case tcell.KeyUp:
		switch a.model.BookmarkDialog.Focus {
		case 0:
			if len(a.model.BookmarkDialog.Ranked) > 0 {
				if a.model.BookmarkDialog.Selected > 0 {
					a.model.BookmarkDialog.Selected--
				}
				ui.EnsureBookmarkListScroll(&a.model.BookmarkDialog, a.bookmarkDialogListRows())
			}
		default:
			a.model.BookmarkDialog.Focus = 0
			ui.EnsureBookmarkListScroll(&a.model.BookmarkDialog, a.bookmarkDialogListRows())
		}
	case tcell.KeyDown:
		switch a.model.BookmarkDialog.Focus {
		case 0:
			if len(a.model.BookmarkDialog.Ranked) > 0 {
				if a.model.BookmarkDialog.Selected < len(a.model.BookmarkDialog.Ranked)-1 {
					a.model.BookmarkDialog.Selected++
				}
				ui.EnsureBookmarkListScroll(&a.model.BookmarkDialog, a.bookmarkDialogListRows())
			}
		case 1:
			a.model.BookmarkDialog.Focus = 2
		}
	case tcell.KeyHome:
		if a.model.BookmarkDialog.Focus == 0 && len(a.model.BookmarkDialog.Ranked) > 0 {
			a.model.BookmarkDialog.Selected = 0
			ui.EnsureBookmarkListScroll(&a.model.BookmarkDialog, a.bookmarkDialogListRows())
		}
	case tcell.KeyEnd:
		if a.model.BookmarkDialog.Focus == 0 && len(a.model.BookmarkDialog.Ranked) > 0 {
			a.model.BookmarkDialog.Selected = len(a.model.BookmarkDialog.Ranked) - 1
			ui.EnsureBookmarkListScroll(&a.model.BookmarkDialog, a.bookmarkDialogListRows())
		}
	case tcell.KeyPgUp:
		if a.model.BookmarkDialog.Focus == 0 && len(a.model.BookmarkDialog.Ranked) > 0 {
			step := max(1, a.bookmarkDialogListRows()-1)
			a.model.BookmarkDialog.Selected = max(0, a.model.BookmarkDialog.Selected-step)
			ui.EnsureBookmarkListScroll(&a.model.BookmarkDialog, a.bookmarkDialogListRows())
		}
	case tcell.KeyPgDn:
		if a.model.BookmarkDialog.Focus == 0 && len(a.model.BookmarkDialog.Ranked) > 0 {
			step := max(1, a.bookmarkDialogListRows()-1)
			maxSel := len(a.model.BookmarkDialog.Ranked) - 1
			a.model.BookmarkDialog.Selected = min(maxSel, a.model.BookmarkDialog.Selected+step)
			ui.EnsureBookmarkListScroll(&a.model.BookmarkDialog, a.bookmarkDialogListRows())
		}
	case tcell.KeyLeft:
		switch a.model.BookmarkDialog.Focus {
		case 1:
			a.model.BookmarkDialog.Focus = 0
		case 2:
			a.model.BookmarkDialog.Focus = 1
		}
	case tcell.KeyRight:
		if a.model.BookmarkDialog.Focus == 1 {
			a.model.BookmarkDialog.Focus = 2
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		// Picker/filter focus: all printable runes extend the query only (Enter / Alt+O confirms).
		// Bare o/c/space must not activate OK/Cancel while typing.
		if a.model.BookmarkDialog.Focus == 0 {
			if unicode.IsPrint(event.Rune()) {
				a.model.BookmarkDialog.Query += string(event.Rune())
				a.syncBookmarkDialogRanks()
				a.model.BookmarkDialog.Selected = 0
				ui.EnsureBookmarkListScroll(&a.model.BookmarkDialog, a.bookmarkDialogListRows())
			}
			break
		}
		switch event.Rune() {
		case 'o', 'O':
			a.activateBookmarkSelection()
		case 'c', 'C':
			a.closeBookmarkDialog()
		case ' ':
			switch a.model.BookmarkDialog.Focus {
			case 1:
				a.activateBookmarkSelection()
			case 2:
				a.closeBookmarkDialog()
			}
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if a.model.BookmarkDialog.Focus != 0 {
			break
		}
		r := []rune(a.model.BookmarkDialog.Query)
		if len(r) > 0 {
			a.model.BookmarkDialog.Query = string(r[:len(r)-1])
			a.syncBookmarkDialogRanks()
			a.model.BookmarkDialog.Selected = 0
			ui.EnsureBookmarkListScroll(&a.model.BookmarkDialog, a.bookmarkDialogListRows())
		}
	case tcell.KeyCtrlU:
		if a.model.BookmarkDialog.Focus != 0 {
			break
		}
		if a.model.BookmarkDialog.Query != "" {
			a.model.BookmarkDialog.Query = ""
			a.syncBookmarkDialogRanks()
			a.model.BookmarkDialog.Selected = 0
			ui.EnsureBookmarkListScroll(&a.model.BookmarkDialog, a.bookmarkDialogListRows())
		}
	}
}

// openAddBookmarkDialog presents the centered dialog to append a new fzf-marks entry
// for the active panel directory. Refuses while jobs view is active and cancels any
// open quick filter (mirroring openBookmarkDialog).
func (a *App) openAddBookmarkDialog() {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return
	}
	if a.inQuickFilterUI() {
		a.activePanel().CancelFilter(a.activeViewportRows())
	}
	path := a.activePanel().Path
	if strings.TrimSpace(path) == "" {
		a.setErrorMessage("Add bookmark", fmt.Errorf("no active panel path"))
		return
	}
	defaultName := defaultBookmarkName(path)
	cursor := len([]rune(defaultName))
	pending := defaultName != ""
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogAddBookmark,
		Fields: []ui.FileDialogField{
			{
				Label:          "Name",
				Value:          defaultName,
				Prefill:        defaultName,
				Cursor:         cursor,
				PrefillPending: pending,
			},
		},
		Message: path,
	}
}

// defaultBookmarkName returns a sensible suggested mark name for path.
// Uses the basename, falling back to "root" for "/" or empty results.
func defaultBookmarkName(path string) string {
	base := filepath.Base(filepath.Clean(path))
	switch base {
	case "", ".", string(filepath.Separator):
		return "root"
	}
	return base
}

// addBookmarkDialogInputField returns the name field for Add bookmark, including when
// keyboard focus is on OK/Cancel (focusedField returns nil in that case).
func (a *App) addBookmarkDialogInputField() *ui.FileDialogField {
	d := &a.model.FileDialog
	if !d.Open || d.DialogType != ui.FileDialogAddBookmark || len(d.Fields) != 1 {
		return nil
	}
	if d.FocusedField >= 0 && d.FocusedField < len(d.Fields) {
		return &d.Fields[d.FocusedField]
	}
	return &d.Fields[0]
}

// executeAddBookmark validates the input, resolves the marks file, and appends
// a new mark line. Closes the dialog with a transient banner on success or error.
func (a *App) executeAddBookmark() {
	field := a.addBookmarkDialogInputField()
	if field == nil {
		a.closeFileDialog()
		return
	}
	name := strings.TrimSpace(field.Value)
	path := strings.TrimSpace(a.model.FileDialog.Message)
	if name == "" {
		a.setErrorMessage("Add bookmark", fmt.Errorf("name is required"))
		a.closeFileDialog()
		return
	}
	if path == "" {
		a.setErrorMessage("Add bookmark", fmt.Errorf("missing target path"))
		a.closeFileDialog()
		return
	}
	marksPath, err := bookmarks.ResolveFile(a.config.Bookmarks.File, a.model.UserHomeDir)
	if err != nil {
		a.setErrorMessage("Add bookmark", err)
		a.closeFileDialog()
		return
	}
	if err := bookmarks.Append(marksPath, bookmarks.Mark{Name: name, Path: path}); err != nil {
		a.setErrorMessage("Add bookmark", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.setTransientMessage(fmt.Sprintf("Bookmark added: %s → %s", name, marksPath), ui.MessageUrgencyInfo)
}
