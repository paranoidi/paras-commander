package keymap

import "fmt"

// OverlaySpec describes one keymap overlay table (e.g. [jobs]).
type OverlaySpec struct {
	TableName string
	Defaults  func() map[string][]string
	Allowed   func(actionID string) bool
	// DisallowedActionError formats a validation error when Allowed rejects an action.
	DisallowedActionError func(source, action string) error
}

// overlayRegistry is the single source of overlay table metadata and ordering.
// Order matches Bundle overlay field assignment in buildBundle.
var overlayRegistry = []OverlaySpec{
	{
		TableName: JobsShortcutsTable,
		Defaults:  DefaultJobsOverlayKeys,
		Allowed:   AllowedInJobsOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [jobs] action %q is not allowed (jobs.* only)", source, action)
		},
	},
	{
		TableName: CommandsShortcutsTable,
		Defaults:  DefaultCommandsOverlayKeys,
		Allowed:   AllowedInCommandsOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [commands] action %q is not allowed (commands.* only)", source, action)
		},
	},
	{
		TableName: MessagesShortcutsTable,
		Defaults:  DefaultMessagesOverlayKeys,
		Allowed:   AllowedInMessagesOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [messages] action %q is not allowed (messages.* only)", source, action)
		},
	},
	{
		TableName: FilePreviewShortcutsTable,
		Defaults:  DefaultFilePreviewOverlayKeys,
		Allowed:   AllowedInFilePreviewOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [file_preview] action %q is not allowed (file.view.* only)", source, action)
		},
	},
	{
		TableName: DialogInputShortcutsTable,
		Defaults:  DefaultDialogInputOverlayKeys,
		Allowed:   AllowedInDialogInputOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [dialog.input] action %q is not allowed (ui.input.* only)", source, action)
		},
	},
	{
		TableName: DialogRenameShortcutsTable,
		Defaults:  DefaultRenameDialogOverlayKeys,
		Allowed:   AllowedInRenameDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [dialog.rename] action %q is not allowed (file.rename.open-* only)", source, action)
		},
	},
	{
		TableName: DialogMkdirShortcutsTable,
		Defaults:  DefaultMkdirDialogOverlayKeys,
		Allowed:   AllowedInMkdirDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [dialog.mkdir] action %q is not allowed (file.mkdir.extract-common-name only)", source, action)
		},
	},
	{
		TableName: DialogBookmarkShortcutsTable,
		Defaults:  DefaultBookmarkDialogOverlayKeys,
		Allowed:   AllowedInBookmarkDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [dialog.bookmark] action %q is not allowed (bookmark.delete only)", source, action)
		},
	},
	{
		TableName: DialogFindShortcutsTable,
		Defaults:  DefaultFindDialogOverlayKeys,
		Allowed:   AllowedInFindDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [dialog.find] action %q is not allowed (find.select-all, find.unselect-all, find.select-group, find.unselect-group only)", source, action)
		},
	},
	{
		TableName: DialogHistoryShortcutsTable,
		Defaults:  DefaultHistoryDialogOverlayKeys,
		Allowed:   AllowedInHistoryDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [dialog.history] action %q is not allowed (panel.history-both-panels only)", source, action)
		},
	},
	{
		TableName: DialogFlattenShortcutsTable,
		Defaults:  DefaultFlattenDialogOverlayKeys,
		Allowed:   AllowedInFlattenDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [dialog.flatten] action %q is not allowed (flatten.destination-active, flatten.destination-inactive only)", source, action)
		},
	},
	{
		TableName: CompareShortcutsTable,
		Defaults:  DefaultCompareOverlayKeys,
		Allowed:   AllowedInCompareOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [compare] action %q is not allowed (compare.* only)", source, action)
		},
	},
}

// OverlayTableNames returns all overlay TOML table names in registry order.
func OverlayTableNames() []string {
	names := make([]string, len(overlayRegistry))
	for i, spec := range overlayRegistry {
		names[i] = spec.TableName
	}
	return names
}

func validateOverlayKeysFromFile(keys map[string][]string, label string, spec OverlaySpec) error {
	if keys == nil {
		return nil
	}
	for action, chords := range keys {
		if len(chords) == 0 {
			return fmt.Errorf("parse keybindings %q: [%s] action %q has empty key list", label, spec.TableName, action)
		}
		if !spec.Allowed(action) {
			return fmt.Errorf("parse keybindings %q: [%s] action %q is not allowed (%s)", label, spec.TableName, action, overlayNotAllowedHint(spec))
		}
	}
	return nil
}

func overlayNotAllowedHint(spec OverlaySpec) string {
	switch spec.TableName {
	case JobsShortcutsTable:
		return "jobs.* only"
	case CommandsShortcutsTable:
		return "commands.* only"
	case MessagesShortcutsTable:
		return "messages.* only"
	case FilePreviewShortcutsTable:
		return "file.view.* only"
	case DialogInputShortcutsTable:
		return "ui.input.* only"
	case DialogRenameShortcutsTable:
		return "file.rename.open-* only"
	case DialogMkdirShortcutsTable:
		return "file.mkdir.extract-common-name only"
	case DialogBookmarkShortcutsTable:
		return "bookmark.delete only"
	case DialogFindShortcutsTable:
		return "find.select-all, find.unselect-all, find.select-group, find.unselect-group only"
	case DialogHistoryShortcutsTable:
		return "panel.history-both-panels only"
	case DialogFlattenShortcutsTable:
		return "flatten.destination-active, flatten.destination-inactive only"
	case CompareShortcutsTable:
		return "compare.* only"
	default:
		return "not allowed"
	}
}

func defaultOverlayLayers() []map[string][]string {
	layers := make([]map[string][]string, len(overlayRegistry))
	for i, spec := range overlayRegistry {
		layers[i] = spec.Defaults()
	}
	return layers
}
