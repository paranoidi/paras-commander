package keymap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/paranoidi/paras-commander/internal/config"
)

// LoadFromPaths resolves the full Bundle (global + jobs-view + Commands-view + Messages-view +
// path-picker-host + dialog-input + rename-dialog overlays) using a layered merge per table:
//
//  1. built-in defaults (DefaultActionKeys / DefaultJobsOverlayKeys / DefaultCommandsOverlayKeys /
//     DefaultMessagesOverlayKeys / DefaultPathPickerHostOverlayKeys / DefaultDialogInputOverlayKeys /
//     DefaultRenameDialogOverlayKeys)
//  2. config.toml's [action_keys] / [jobs_action_keys] / [commands_action_keys] / [messages_action_keys] /
//     [path_picker_host_action_keys] (must be empty) / [dialog_input_action_keys] / [rename_dialog_action_keys] (when present)
//  3. keybindings.toml's matching tables (when present) — wins over config.toml
//
// Any source can be absent without failing startup; built-in defaults
// remain wherever no overrides are supplied. Layering applies
// independently per table so a user can override only the global map,
// only one overlay, or any combination.
func LoadFromPaths(paths config.Paths) (*Bundle, error) {
	globalLayer := DefaultActionKeys()
	jobsLayer := DefaultJobsOverlayKeys()
	commandsLayer := DefaultCommandsOverlayKeys()
	messagesLayer := DefaultMessagesOverlayKeys()
	pathPickerHostLayer := DefaultPathPickerHostOverlayKeys()
	dialogInputLayer := DefaultDialogInputOverlayKeys()
	renameDialogLayer := DefaultRenameDialogOverlayKeys()

	configActionKeys, err := config.ReadActionKeys(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	globalLayer = mergeBindings(globalLayer, configActionKeys)

	configJobsKeys, err := config.ReadJobsActionKeys(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	if err := validateJobsOverlayKeys(configJobsKeys, paths.ConfigFile); err != nil {
		return nil, err
	}
	jobsLayer = mergeBindings(jobsLayer, configJobsKeys)

	configCommandsKeys, err := config.ReadCommandsActionKeys(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	if err := validateCommandsOverlayKeys(configCommandsKeys, paths.ConfigFile); err != nil {
		return nil, err
	}
	commandsLayer = mergeBindings(commandsLayer, configCommandsKeys)

	configMessagesKeys, err := config.ReadMessagesActionKeys(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	if err := validateMessagesOverlayKeys(configMessagesKeys, paths.ConfigFile); err != nil {
		return nil, err
	}
	messagesLayer = mergeBindings(messagesLayer, configMessagesKeys)

	configPathPickerHostKeys, err := config.ReadPathPickerHostActionKeys(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	if err := validatePathPickerHostOverlayKeys(configPathPickerHostKeys, paths.ConfigFile); err != nil {
		return nil, err
	}
	pathPickerHostLayer = mergeBindings(pathPickerHostLayer, configPathPickerHostKeys)

	configDialogInputKeys, err := config.ReadDialogInputActionKeys(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	if err := validateDialogInputOverlayKeys(configDialogInputKeys, paths.ConfigFile); err != nil {
		return nil, err
	}
	dialogInputLayer = mergeBindings(dialogInputLayer, configDialogInputKeys)

	configRenameDialogKeys, err := config.ReadRenameDialogActionKeys(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	if err := validateRenameDialogOverlayKeys(configRenameDialogKeys, paths.ConfigFile); err != nil {
		return nil, err
	}
	renameDialogLayer = mergeBindings(renameDialogLayer, configRenameDialogKeys)

	file := strings.TrimSpace(paths.KeybindingsFile)
	if file == "" && strings.TrimSpace(paths.ConfigDir) != "" {
		file = filepath.Join(paths.ConfigDir, "keybindings.toml")
	}
	if file == "" {
		return buildBundle(globalLayer, jobsLayer, commandsLayer, messagesLayer, pathPickerHostLayer, dialogInputLayer, renameDialogLayer)
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return buildBundle(globalLayer, jobsLayer, commandsLayer, messagesLayer, pathPickerHostLayer, dialogInputLayer, renameDialogLayer)
		}
		return nil, fmt.Errorf("read keybindings %q: %w", file, err)
	}

	actionUser, jobsUser, commandsUser, messagesUser, pathPickerHostUser, dialogInputUser, renameDialogUser, err := parseKeybindingsFile(raw, file)
	if err != nil {
		return nil, err
	}
	globalLayer = mergeBindings(globalLayer, actionUser)
	jobsLayer = mergeBindings(jobsLayer, jobsUser)
	commandsLayer = mergeBindings(commandsLayer, commandsUser)
	messagesLayer = mergeBindings(messagesLayer, messagesUser)
	pathPickerHostLayer = mergeBindings(pathPickerHostLayer, pathPickerHostUser)
	dialogInputLayer = mergeBindings(dialogInputLayer, dialogInputUser)
	renameDialogLayer = mergeBindings(renameDialogLayer, renameDialogUser)
	return buildBundle(globalLayer, jobsLayer, commandsLayer, messagesLayer, pathPickerHostLayer, dialogInputLayer, renameDialogLayer)
}

// DefaultBundle returns built-in global + overlay defaults (no keybindings file).
func DefaultBundle() (*Bundle, error) {
	return buildBundle(DefaultActionKeys(), DefaultJobsOverlayKeys(), DefaultCommandsOverlayKeys(), DefaultMessagesOverlayKeys(), DefaultPathPickerHostOverlayKeys(), DefaultDialogInputOverlayKeys(), DefaultRenameDialogOverlayKeys())
}

func buildBundle(global, jobs, commands, messages, pathPickerHost, dialogInput, renameDialog map[string][]string) (*Bundle, error) {
	gMap, err := Build(global)
	if err != nil {
		return nil, err
	}
	jMap, err := Build(jobs)
	if err != nil {
		return nil, err
	}
	cMap, err := Build(commands)
	if err != nil {
		return nil, err
	}
	msgMap, err := Build(messages)
	if err != nil {
		return nil, err
	}
	phMap, err := Build(pathPickerHost)
	if err != nil {
		return nil, err
	}
	diMap, err := Build(dialogInput)
	if err != nil {
		return nil, err
	}
	rdMap, err := Build(renameDialog)
	if err != nil {
		return nil, err
	}
	return &Bundle{Global: gMap, Jobs: jMap, Commands: cMap, Messages: msgMap, PathPickerHost: phMap, DialogInput: diMap, RenameDialog: rdMap}, nil
}

func validateCommandsOverlayKeys(keys map[string][]string, source string) error {
	if keys == nil {
		return nil
	}
	for action, chords := range keys {
		if len(chords) == 0 {
			return fmt.Errorf("parse config %q: [commands_action_keys] action %q has empty key list", source, action)
		}
		if !AllowedInCommandsOverlay(action) {
			return fmt.Errorf("parse config %q: [commands_action_keys] action %q is not allowed (commands.* only)", source, action)
		}
	}
	return nil
}

func validateMessagesOverlayKeys(keys map[string][]string, source string) error {
	if keys == nil {
		return nil
	}
	for action, chords := range keys {
		if len(chords) == 0 {
			return fmt.Errorf("parse config %q: [messages_action_keys] action %q has empty key list", source, action)
		}
		if !AllowedInMessagesOverlay(action) {
			return fmt.Errorf("parse config %q: [messages_action_keys] action %q is not allowed (messages.* only)", source, action)
		}
	}
	return nil
}

func validatePathPickerHostOverlayKeys(keys map[string][]string, source string) error {
	if keys == nil {
		return nil
	}
	for action, chords := range keys {
		if len(chords) == 0 {
			return fmt.Errorf("parse config %q: [path_picker_host_action_keys] action %q has empty key list", source, action)
		}
		if !AllowedInPathPickerHostOverlay(action) {
			return fmt.Errorf("parse config %q: [path_picker_host_action_keys] must be empty (got action %q); fuzzy path picker on destination/symlink path rows uses bookmark.open from [action_keys]", source, action)
		}
	}
	return nil
}

func validateDialogInputOverlayKeys(keys map[string][]string, source string) error {
	if keys == nil {
		return nil
	}
	for action, chords := range keys {
		if len(chords) == 0 {
			return fmt.Errorf("parse config %q: [dialog_input_action_keys] action %q has empty key list", source, action)
		}
		if !AllowedInDialogInputOverlay(action) {
			return fmt.Errorf("parse config %q: [dialog_input_action_keys] action %q is not allowed (ui.input.* only)", source, action)
		}
	}
	return nil
}

func validateRenameDialogOverlayKeys(keys map[string][]string, source string) error {
	if keys == nil {
		return nil
	}
	for action, chords := range keys {
		if len(chords) == 0 {
			return fmt.Errorf("parse config %q: [rename_dialog_action_keys] action %q has empty key list", source, action)
		}
		if !AllowedInRenameDialogOverlay(action) {
			return fmt.Errorf("parse config %q: [rename_dialog_action_keys] action %q is not allowed (file.rename.open-* only)", source, action)
		}
	}
	return nil
}

// validateJobsOverlayKeys enforces the jobs.* restriction for entries
// supplied via config.toml. The same rule is enforced inside
// parseKeybindingsFile for keybindings.toml; centralising the check via
// AllowedInJobsOverlay keeps both call sites in sync as the registry
// grows.
func validateJobsOverlayKeys(keys map[string][]string, source string) error {
	if keys == nil {
		return nil
	}
	for action, chords := range keys {
		if len(chords) == 0 {
			return fmt.Errorf("parse config %q: [jobs_action_keys] action %q has empty key list", source, action)
		}
		if !AllowedInJobsOverlay(action) {
			return fmt.Errorf("parse config %q: [jobs_action_keys] action %q is not allowed (jobs.* only)", source, action)
		}
	}
	return nil
}

func parseKeybindingsFile(raw []byte, label string) (actionKeys, jobsKeys, commandsKeys, messagesKeys, pathPickerHostKeys, dialogInputKeys, renameDialogKeys map[string][]string, err error) {
	var top map[string]interface{}
	if err := toml.Unmarshal(raw, &top); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
	}
	if len(top) == 0 {
		return map[string][]string{}, map[string][]string{}, map[string][]string{}, map[string][]string{}, map[string][]string{}, map[string][]string{}, map[string][]string{}, nil
	}
	for k := range top {
		if k != "action_keys" && k != "jobs_action_keys" && k != "commands_action_keys" && k != "messages_action_keys" && k != "path_picker_host_action_keys" && k != "dialog_input_action_keys" && k != "rename_dialog_action_keys" {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: unknown field %q (allowed: action_keys, jobs_action_keys, commands_action_keys, messages_action_keys, path_picker_host_action_keys, dialog_input_action_keys, rename_dialog_action_keys)", label, k)
		}
	}

	actionKeys = map[string][]string{}
	if rawAK, ok := top["action_keys"]; ok {
		table, ok := rawAK.(map[string]interface{})
		if !ok {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", actionKeys); err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range actionKeys {
			if len(keys) == 0 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: action %q has empty key list", label, action)
			}
		}
	}

	jobsKeys = map[string][]string{}
	if rawJK, ok := top["jobs_action_keys"]; ok {
		table, ok := rawJK.(map[string]interface{})
		if !ok {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [jobs_action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", jobsKeys); err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range jobsKeys {
			if len(keys) == 0 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [jobs_action_keys] action %q has empty key list", label, action)
			}
			if !AllowedInJobsOverlay(action) {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [jobs_action_keys] action %q is not allowed (jobs.* only)", label, action)
			}
		}
	}

	commandsKeys = map[string][]string{}
	if rawCK, ok := top["commands_action_keys"]; ok {
		table, ok := rawCK.(map[string]interface{})
		if !ok {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [commands_action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", commandsKeys); err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range commandsKeys {
			if len(keys) == 0 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [commands_action_keys] action %q has empty key list", label, action)
			}
			if !AllowedInCommandsOverlay(action) {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [commands_action_keys] action %q is not allowed (commands.* only)", label, action)
			}
		}
	}

	messagesKeys = map[string][]string{}
	if rawMK, ok := top["messages_action_keys"]; ok {
		table, ok := rawMK.(map[string]interface{})
		if !ok {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [messages_action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", messagesKeys); err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range messagesKeys {
			if len(keys) == 0 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [messages_action_keys] action %q has empty key list", label, action)
			}
			if !AllowedInMessagesOverlay(action) {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [messages_action_keys] action %q is not allowed (messages.* only)", label, action)
			}
		}
	}

	pathPickerHostKeys = map[string][]string{}
	if rawPP, ok := top["path_picker_host_action_keys"]; ok {
		table, ok := rawPP.(map[string]interface{})
		if !ok {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [path_picker_host_action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", pathPickerHostKeys); err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range pathPickerHostKeys {
			if len(keys) == 0 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [path_picker_host_action_keys] action %q has empty key list", label, action)
			}
			if !AllowedInPathPickerHostOverlay(action) {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [path_picker_host_action_keys] must be empty (got action %q); fuzzy path picker on destination/symlink path rows uses bookmark.open from [action_keys]", label, action)
			}
		}
	}

	dialogInputKeys = map[string][]string{}
	if rawDI, ok := top["dialog_input_action_keys"]; ok {
		table, ok := rawDI.(map[string]interface{})
		if !ok {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [dialog_input_action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", dialogInputKeys); err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range dialogInputKeys {
			if len(keys) == 0 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [dialog_input_action_keys] action %q has empty key list", label, action)
			}
			if !AllowedInDialogInputOverlay(action) {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [dialog_input_action_keys] action %q is not allowed (ui.input.* only)", label, action)
			}
		}
	}

	renameDialogKeys = map[string][]string{}
	if rawRD, ok := top["rename_dialog_action_keys"]; ok {
		table, ok := rawRD.(map[string]interface{})
		if !ok {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [rename_dialog_action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", renameDialogKeys); err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range renameDialogKeys {
			if len(keys) == 0 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [rename_dialog_action_keys] action %q has empty key list", label, action)
			}
			if !AllowedInRenameDialogOverlay(action) {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("parse keybindings %q: [rename_dialog_action_keys] action %q is not allowed (file.rename.open-* only)", label, action)
			}
		}
	}

	return actionKeys, jobsKeys, commandsKeys, messagesKeys, pathPickerHostKeys, dialogInputKeys, renameDialogKeys, nil
}

func collectActionKeys(node map[string]interface{}, prefix string, out map[string][]string) error {
	for k, v := range node {
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		switch vv := v.(type) {
		case map[string]interface{}:
			if err := collectActionKeys(vv, full, out); err != nil {
				return err
			}
		case []interface{}:
			strs := make([]string, 0, len(vv))
			for _, item := range vv {
				s, ok := item.(string)
				if !ok {
					return fmt.Errorf("key %q: expected string list elements", full)
				}
				strs = append(strs, s)
			}
			out[full] = strs
		default:
			return fmt.Errorf("key %q: unsupported type %T", full, v)
		}
	}
	return nil
}

// mergeBindings overlays user keys on defaults: for each action present in user, replace default chords entirely.
func mergeBindings(defaults, user map[string][]string) map[string][]string {
	out := make(map[string][]string)
	for k, v := range defaults {
		out[k] = append([]string(nil), v...)
	}
	for action, keys := range user {
		out[action] = append([]string(nil), keys...)
	}
	return out
}

// WriteDefaultStub writes default keybindings TOML to path.
func WriteDefaultStub(filename string) error {
	if filename == "" {
		return fmt.Errorf("keybindings stub filename is required")
	}
	var buf bytes.Buffer
	if err := EncodeDefaultStub(&buf); err != nil {
		return err
	}
	if err := os.WriteFile(filename, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write keybindings stub %q: %w", filename, err)
	}
	return nil
}

// EncodeDefaultStub writes the canonical keybindings TOML: a leading
// comment block describing the file and then shortcut tables as
// real (uncommented) entries.
//
// The output is sourced from DefaultActionKeys (includes jobs.open and commands.open via
// specs) and overlay defaults. Adding a new ActionSpec default chord or extending an overlay map is
// automatically reflected in every stub written via `--keybindings-stub`
// or `--config-stub` without further plumbing.
func EncodeDefaultStub(w io.Writer) error {
	header := "# Global shortcuts under [action_keys]. Each value is a list of\n" +
		"# chord strings (single-stroke). See docs/keybindings.md for syntax.\n" +
		"#\n" +
		"# Jobs-view-only chords under [jobs_action_keys] take precedence over\n" +
		"# [action_keys] while the jobs view is focused. Only jobs.* action IDs\n" +
		"# are accepted there.\n" +
		"#\n" +
		"# Commands-view-only chords under [commands_action_keys] take precedence\n" +
		"# while the Commands view is focused. Only commands.* action IDs are accepted.\n" +
		"#\n" +
		"# Messages-view-only chords under [messages_action_keys] take precedence\n" +
		"# while the Messages view is focused. Only messages.* action IDs are accepted.\n" +
		"#\n" +
		"# [path_picker_host_action_keys] must stay empty: opening the fuzzy path picker\n" +
		"# from copy/move destination or symlink/hardlink path rows uses the same chords\n" +
		"# as bookmark.open under [action_keys].\n" +
		"#\n" +
		"# Dialog input field actions (e.g. restore default placeholder) use\n" +
		"# [dialog_input_action_keys]. Only ui.input.* action IDs are accepted.\n" +
		"#\n" +
		"# Rename dialog (main name field only) uses [rename_dialog_action_keys] for\n" +
		"# sanitize/slugify helpers. Only file.rename.open-sanitize and file.rename.open-slugify.\n\n"
	if _, err := io.WriteString(w, header); err != nil {
		return fmt.Errorf("encode keybindings stub header: %w", err)
	}
	payload := struct {
		ActionKeys               map[string][]string `toml:"action_keys"`
		JobsActionKeys           map[string][]string `toml:"jobs_action_keys"`
		CommandsActionKeys       map[string][]string `toml:"commands_action_keys"`
		MessagesActionKeys       map[string][]string `toml:"messages_action_keys"`
		PathPickerHostActionKeys map[string][]string `toml:"path_picker_host_action_keys"`
		DialogInputActionKeys    map[string][]string `toml:"dialog_input_action_keys"`
		RenameDialogActionKeys   map[string][]string `toml:"rename_dialog_action_keys"`
	}{
		ActionKeys:               DefaultActionKeys(),
		JobsActionKeys:           DefaultJobsOverlayKeys(),
		CommandsActionKeys:       DefaultCommandsOverlayKeys(),
		MessagesActionKeys:       DefaultMessagesOverlayKeys(),
		PathPickerHostActionKeys: DefaultPathPickerHostOverlayKeys(),
		DialogInputActionKeys:    DefaultDialogInputOverlayKeys(),
		RenameDialogActionKeys:   DefaultRenameDialogOverlayKeys(),
	}
	if err := toml.NewEncoder(w).Encode(payload); err != nil {
		return fmt.Errorf("encode keybindings stub: %w", err)
	}
	return nil
}
