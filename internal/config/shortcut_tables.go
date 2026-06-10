package config

// Optional top-level TOML tables holding keybindings inside config.toml.
// They mirror the canonical contents of keybindings.toml and are owned
// by the keymap package; config only tolerates them as pass-through so
// a single bootstrap file can carry both general settings and the full
// shortcut map (global plus per-view overlays).
const (
	ActionKeysTable               = "action_keys"
	JobsActionKeysTable           = "jobs_action_keys"
	CommandsActionKeysTable       = "commands_action_keys"
	MessagesActionKeysTable       = "messages_action_keys"
	PathPickerHostActionKeysTable = "path_picker_host_action_keys"
	DialogInputActionKeysTable    = "dialog_input_action_keys"
	RenameDialogActionKeysTable   = "rename_dialog_action_keys"
	BookmarkDialogActionKeysTable = "bookmark_dialog_action_keys"
	FindDialogActionKeysTable     = "find_dialog_action_keys"
	HistoryDialogActionKeysTable  = "history_dialog_action_keys"
	FlattenDialogActionKeysTable  = "flatten_dialog_action_keys"
)

var shortcutPassThroughSet = map[string]struct{}{
	ActionKeysTable:               {},
	JobsActionKeysTable:           {},
	CommandsActionKeysTable:       {},
	MessagesActionKeysTable:       {},
	PathPickerHostActionKeysTable: {},
	DialogInputActionKeysTable:    {},
	RenameDialogActionKeysTable:   {},
	BookmarkDialogActionKeysTable: {},
	FindDialogActionKeysTable:     {},
	HistoryDialogActionKeysTable:  {},
	FlattenDialogActionKeysTable:  {},
}

// IsShortcutPassThroughTable reports whether name is a top-level TOML key
// for keybindings tables tolerated inside config.toml during strict decode.
func IsShortcutPassThroughTable(name string) bool {
	_, ok := shortcutPassThroughSet[name]
	return ok
}
