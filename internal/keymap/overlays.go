package keymap

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/config"
)

// OverlaySpec describes one keymap overlay table (e.g. [jobs_action_keys]).
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
		TableName: config.JobsActionKeysTable,
		Defaults:  DefaultJobsOverlayKeys,
		Allowed:   AllowedInJobsOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [jobs_action_keys] action %q is not allowed (jobs.* only)", source, action)
		},
	},
	{
		TableName: config.CommandsActionKeysTable,
		Defaults:  DefaultCommandsOverlayKeys,
		Allowed:   AllowedInCommandsOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [commands_action_keys] action %q is not allowed (commands.* only)", source, action)
		},
	},
	{
		TableName: config.MessagesActionKeysTable,
		Defaults:  DefaultMessagesOverlayKeys,
		Allowed:   AllowedInMessagesOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [messages_action_keys] action %q is not allowed (messages.* only)", source, action)
		},
	},
	{
		TableName: config.PathPickerHostActionKeysTable,
		Defaults:  DefaultPathPickerHostOverlayKeys,
		Allowed:   AllowedInPathPickerHostOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [path_picker_host_action_keys] must be empty (got action %q); fuzzy path picker on destination/symlink path rows uses bookmark.open from [action_keys]", source, action)
		},
	},
	{
		TableName: config.DialogInputActionKeysTable,
		Defaults:  DefaultDialogInputOverlayKeys,
		Allowed:   AllowedInDialogInputOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [dialog_input_action_keys] action %q is not allowed (ui.input.* only)", source, action)
		},
	},
	{
		TableName: config.RenameDialogActionKeysTable,
		Defaults:  DefaultRenameDialogOverlayKeys,
		Allowed:   AllowedInRenameDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [rename_dialog_action_keys] action %q is not allowed (file.rename.open-* only)", source, action)
		},
	},
	{
		TableName: config.BookmarkDialogActionKeysTable,
		Defaults:  DefaultBookmarkDialogOverlayKeys,
		Allowed:   AllowedInBookmarkDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [bookmark_dialog_action_keys] action %q is not allowed (bookmark.delete only)", source, action)
		},
	},
	{
		TableName: config.FindDialogActionKeysTable,
		Defaults:  DefaultFindDialogOverlayKeys,
		Allowed:   AllowedInFindDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [find_dialog_action_keys] action %q is not allowed (find.select-all, find.unselect-all, find.select-group, find.unselect-group only)", source, action)
		},
	},
	{
		TableName: config.HistoryDialogActionKeysTable,
		Defaults:  DefaultHistoryDialogOverlayKeys,
		Allowed:   AllowedInHistoryDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [history_dialog_action_keys] action %q is not allowed (panel.history-both-panels only)", source, action)
		},
	},
	{
		TableName: config.FlattenDialogActionKeysTable,
		Defaults:  DefaultFlattenDialogOverlayKeys,
		Allowed:   AllowedInFlattenDialogOverlay,
		DisallowedActionError: func(source, action string) error {
			return fmt.Errorf("parse config %q: [flatten_dialog_action_keys] action %q is not allowed (flatten.destination-active, flatten.destination-inactive only)", source, action)
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

func validateOverlayKeys(keys map[string][]string, source string, spec OverlaySpec) error {
	if keys == nil {
		return nil
	}
	for action, chords := range keys {
		if len(chords) == 0 {
			return fmt.Errorf("parse config %q: [%s] action %q has empty key list", source, spec.TableName, action)
		}
		if !spec.Allowed(action) {
			return spec.DisallowedActionError(source, action)
		}
	}
	return nil
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
	case config.JobsActionKeysTable:
		return "jobs.* only"
	case config.CommandsActionKeysTable:
		return "commands.* only"
	case config.MessagesActionKeysTable:
		return "messages.* only"
	case config.PathPickerHostActionKeysTable:
		return "must be empty; fuzzy path picker uses bookmark.open from [action_keys]"
	case config.DialogInputActionKeysTable:
		return "ui.input.* only"
	case config.RenameDialogActionKeysTable:
		return "file.rename.open-* only"
	case config.BookmarkDialogActionKeysTable:
		return "bookmark.delete only"
	case config.FindDialogActionKeysTable:
		return "find.select-all, find.unselect-all, find.select-group, find.unselect-group only"
	case config.HistoryDialogActionKeysTable:
		return "panel.history-both-panels only"
	case config.FlattenDialogActionKeysTable:
		return "flatten.destination-active, flatten.destination-inactive only"
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
