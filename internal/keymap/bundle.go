package keymap

// Bundle holds the global keymap plus optional full-screen view overlays.
// When resolving keys in the jobs, Commands, Messages, or file preview view, consult the matching overlay first, then Global.
type Bundle struct {
	Global           *Map
	Jobs             *Map // may be nil when no overlay chords are configured
	Commands         *Map // Commands-view overlay; may be nil
	Messages         *Map // Messages-view overlay; may be nil
	FilePreview      *Map // F3 full-screen file view overlay; may be nil
	DialogInput      *Map // dialog input field actions (e.g. restore default placeholder)
	RenameDialog     *Map // main rename dialog (sanitize/slugify shortcuts)
	MkdirDialog      *Map // mkdir dialog (extract common name from selection)
	BookmarkDialog   *Map // bookmarks path picker (delete fzf-marks entry)
	FindDialog       *Map // find dialog (select all ranked results)
	HistoryDialog    *Map // history dialog (toggle both panels)
	FlattenDialog    *Map // flatten dialog (destination active/inactive panel)
	TransferDialog   *Map // copy/move dialog (destination active/inactive panel)
	Compare          *Map // Compare-view overlay; may be nil
	Dedup            *Map // find-duplicates view overlay; may be nil
	Terminal         *Map // embedded terminal panel overlay; may be nil
	MassRenameDialog *Map // mass-rename dialog (save/load/delete pattern shortcuts)
	RunForEachDialog *Map // run-for-each dialog (command history shortcut)
	// LeaderKey maps action ID → single-letter Esc function-menu leader key (merged defaults + user).
	LeaderKey map[string]string
	// CopyMenuKey maps action ID → single-letter `"` copy-menu key (merged defaults + user).
	CopyMenuKey map[string]string
	// PreviewMenuKey maps action ID → single-letter `:` fullscreen-preview-menu key (merged defaults + user).
	PreviewMenuKey map[string]string
}

// ActionForLeaderKey returns the action ID bound to the Esc function-menu leader key r,
// if any (reverse lookup over LeaderKey, which maps action ID → letter). Only considers
// actions in the browser's built-in leader menu (leaderMenuGroupActions) — the sole caller is
// vi-motion mode's browser-only "every leader-menu letter fires directly" shortcut (see
// internal/app/input.go), and per-view leader-menu actions (Compare/Dedup/Jobs/Commands/
// Messages) may legally reuse the same letter for a different action in their own closed
// scope, so an unscoped reverse lookup here would resolve ambiguously.
func (b *Bundle) ActionForLeaderKey(r rune) (string, bool) {
	if b == nil {
		return "", false
	}
	return b.actionForLeaderKeyInScope(r, flattenGroupActions(leaderMenuGroupActions))
}

// ActionForLeaderKeyInView returns the action ID bound to leader-menu letter r within vm's own
// per-view leader menu (leaderMenuViewSpecs[vm]), if any. Parallel to ActionForLeaderKey but
// scoped to one auxiliary view's closed letter set instead of the browser's — the sole caller is
// vi-motion mode's per-view "every leader-menu letter fires directly" shortcut (see
// internal/app/leader_menu.go), and different views may legally reuse the same letter for a
// different action in their own scope (e.g. Compare's `c` = Close vs. Dedup's `c` = Collapse), so
// an unscoped or browser-scoped reverse lookup would resolve ambiguously or miss entirely.
func (b *Bundle) ActionForLeaderKeyInView(r rune, vm HelpViews) (string, bool) {
	if b == nil {
		return "", false
	}
	spec, ok := leaderMenuViewSpecs[vm]
	if !ok {
		return "", false
	}
	return b.actionForLeaderKeyInScope(r, flattenGroupActions(spec.actions))
}

// actionForLeaderKeyInScope is the shared reverse lookup behind ActionForLeaderKey and
// ActionForLeaderKeyInView: the action ID within scope bound to leader-menu letter r, if any.
func (b *Bundle) actionForLeaderKeyInScope(r rune, scope map[string]struct{}) (string, bool) {
	for actionID, letter := range b.LeaderKey {
		if _, ok := scope[actionID]; !ok {
			continue
		}
		if letter == string(r) {
			return actionID, true
		}
	}
	return "", false
}
