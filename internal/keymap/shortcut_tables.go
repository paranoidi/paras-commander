package keymap

// TOML table names for keybindings.toml shortcut sections.
const (
	MainShortcutsTable           = "main"
	JobsShortcutsTable           = "jobs"
	CommandsShortcutsTable       = "commands"
	MessagesShortcutsTable       = "messages"
	FilePreviewShortcutsTable    = "file_preview"
	DialogShortcutsGroup         = "dialog"
	DialogInputShortcutsTable    = "dialog.input"
	DialogRenameShortcutsTable   = "dialog.rename"
	DialogMkdirShortcutsTable    = "dialog.mkdir"
	DialogBookmarkShortcutsTable = "dialog.bookmark"
	DialogFindShortcutsTable     = "dialog.find"
	DialogHistoryShortcutsTable  = "dialog.history"
	DialogFlattenShortcutsTable  = "dialog.flatten"
	CompareShortcutsTable        = "compare"
	DedupShortcutsTable          = "dedup"
)

var dialogShortcutSubtables = map[string]struct{}{
	"input":    {},
	"rename":   {},
	"mkdir":    {},
	"bookmark": {},
	"find":     {},
	"history":  {},
	"flatten":  {},
}

// AllShortcutTablePaths returns every shortcut table path (top-level and dialog.*).
func AllShortcutTablePaths() []string {
	return []string{
		MainShortcutsTable,
		JobsShortcutsTable,
		CommandsShortcutsTable,
		MessagesShortcutsTable,
		FilePreviewShortcutsTable,
		DialogInputShortcutsTable,
		DialogRenameShortcutsTable,
		DialogMkdirShortcutsTable,
		DialogBookmarkShortcutsTable,
		DialogFindShortcutsTable,
		DialogHistoryShortcutsTable,
		DialogFlattenShortcutsTable,
		CompareShortcutsTable,
		DedupShortcutsTable,
	}
}

// IsDialogShortcutSubtable reports whether name is a valid sub-table under [dialog].
func IsDialogShortcutSubtable(name string) bool {
	_, ok := dialogShortcutSubtables[name]
	return ok
}
