package keymap

// Bundle holds the global keymap plus optional full-screen view overlays.
// When resolving keys in the jobs, Commands, or Messages view, consult the matching overlay first, then Global.
type Bundle struct {
	Global         *Map
	Jobs           *Map // may be nil when no overlay chords are configured
	Commands       *Map // Commands-view overlay; may be nil
	Messages       *Map // Messages-view overlay; may be nil
	PathPickerHost *Map // path-picker host dialogs (copy/move dest, symlink/hardlink paths)
	DialogInput    *Map // dialog input field actions (e.g. restore default placeholder)
	RenameDialog   *Map // main rename dialog (sanitize/slugify shortcuts)
	BookmarkDialog *Map // bookmarks path picker (delete fzf-marks entry)
	FindDialog     *Map // find dialog (select all ranked results)
}
