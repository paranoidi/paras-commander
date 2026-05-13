package keymap

// Bundle holds the global keymap plus optional full-screen view overlays.
// When resolving keys in the jobs or Commands view, consult the matching overlay first, then Global.
type Bundle struct {
	Global         *Map
	Jobs           *Map // may be nil when no overlay chords are configured
	Commands       *Map // Commands-view overlay; may be nil
	PathPickerHost *Map // path-picker host dialogs (copy/move dest, symlink/hardlink paths)
	DialogInput    *Map // dialog input field actions (e.g. restore default placeholder)
	RenameDialog   *Map // main rename dialog (sanitize/slugify shortcuts)
}
