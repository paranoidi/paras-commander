package keymap

// Bundle holds the global keymap plus optional full-screen view overlays.
// When resolving keys in the jobs, Commands, Messages, or file preview view, consult the matching overlay first, then Global.
type Bundle struct {
	Global         *Map
	Jobs           *Map // may be nil when no overlay chords are configured
	Commands       *Map // Commands-view overlay; may be nil
	Messages       *Map // Messages-view overlay; may be nil
	FilePreview    *Map // F3 full-screen file view overlay; may be nil
	DialogInput    *Map // dialog input field actions (e.g. restore default placeholder)
	RenameDialog   *Map // main rename dialog (sanitize/slugify shortcuts)
	MkdirDialog    *Map // mkdir dialog (extract common name from selection)
	BookmarkDialog *Map // bookmarks path picker (delete fzf-marks entry)
	FindDialog     *Map // find dialog (select all ranked results)
	HistoryDialog  *Map // history dialog (toggle both panels)
	FlattenDialog  *Map // flatten dialog (destination active/inactive panel)
	Compare        *Map // Compare-view overlay; may be nil
	Dedup          *Map // find-duplicates view overlay; may be nil
}
