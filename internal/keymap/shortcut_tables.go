package keymap

// TOML table names for keybindings.toml shortcut sections.
const (
	MainShortcutsTable             = "main"
	JobsShortcutsTable             = "jobs"
	CommandsShortcutsTable         = "commands"
	MessagesShortcutsTable         = "messages"
	FilePreviewShortcutsTable      = "file_preview"
	DialogShortcutsGroup           = "dialog"
	DialogInputShortcutsTable      = "dialog.input"
	DialogRenameShortcutsTable     = "dialog.rename"
	DialogMassRenameShortcutsTable = "dialog.mass_rename"
	DialogRunForEachShortcutsTable = "dialog.run_for_each"
	DialogMkdirShortcutsTable      = "dialog.mkdir"
	DialogBookmarkShortcutsTable   = "dialog.bookmark"
	DialogFindShortcutsTable       = "dialog.find"
	DialogHistoryShortcutsTable    = "dialog.history"
	DialogFlattenShortcutsTable    = "dialog.flatten"
	DialogTransferShortcutsTable   = "dialog.transfer"
	CompareShortcutsTable          = "compare"
	DedupShortcutsTable            = "dedup"
	TerminalShortcutsTable         = "terminal"
	LeaderKeyShortcutsTable        = "leader_key"
	CopyMenuShortcutsTable         = "copy_menu"
	PreviewMenuShortcutsTable      = "preview_menu"
)

var dialogShortcutSubtables = map[string]struct{}{
	"input":        {},
	"rename":       {},
	"mass_rename":  {},
	"mkdir":        {},
	"bookmark":     {},
	"find":         {},
	"history":      {},
	"flatten":      {},
	"transfer":     {},
	"run_for_each": {},
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
		DialogMassRenameShortcutsTable,
		DialogMkdirShortcutsTable,
		DialogBookmarkShortcutsTable,
		DialogFindShortcutsTable,
		DialogHistoryShortcutsTable,
		DialogFlattenShortcutsTable,
		DialogTransferShortcutsTable,
		DialogRunForEachShortcutsTable,
		CompareShortcutsTable,
		DedupShortcutsTable,
		TerminalShortcutsTable,
	}
}

// IsDialogShortcutSubtable reports whether name is a valid sub-table under [dialog].
func IsDialogShortcutSubtable(name string) bool {
	_, ok := dialogShortcutSubtables[name]
	return ok
}
