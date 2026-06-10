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
//     DefaultRenameDialogOverlayKeys / DefaultBookmarkDialogOverlayKeys / DefaultFindDialogOverlayKeys /
//     DefaultHistoryDialogOverlayKeys / DefaultFlattenDialogOverlayKeys)
//  2. config.toml's [action_keys] / [jobs_action_keys] / [commands_action_keys] / [messages_action_keys] /
//     [path_picker_host_action_keys] (must be empty) / [dialog_input_action_keys] / [rename_dialog_action_keys] /
//     [bookmark_dialog_action_keys] / [find_dialog_action_keys] / [history_dialog_action_keys] /
//     [flatten_dialog_action_keys] (when present)
//  3. keybindings.toml's matching tables (when present) — wins over config.toml
//
// Any source can be absent without failing startup; built-in defaults
// remain wherever no overrides are supplied. Layering applies
// independently per table so a user can override only the global map,
// only one overlay, or any combination.
func LoadFromPaths(paths config.Paths) (*Bundle, error) {
	globalLayer := DefaultActionKeys()
	overlayLayers := defaultOverlayLayers()

	configActionKeys, err := config.ReadActionKeys(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	globalLayer = mergeBindings(globalLayer, configActionKeys)

	for i, spec := range overlayRegistry {
		configKeys, err := config.ReadOverlayActionKeys(paths.ConfigFile, spec.TableName)
		if err != nil {
			return nil, err
		}
		if err := validateOverlayKeys(configKeys, paths.ConfigFile, spec); err != nil {
			return nil, err
		}
		overlayLayers[i] = mergeBindings(overlayLayers[i], configKeys)
	}

	file := strings.TrimSpace(paths.KeybindingsFile)
	if file == "" && strings.TrimSpace(paths.ConfigDir) != "" {
		file = filepath.Join(paths.ConfigDir, "keybindings.toml")
	}
	if file == "" {
		return buildBundle(globalLayer, overlayLayers)
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return buildBundle(globalLayer, overlayLayers)
		}
		return nil, fmt.Errorf("read keybindings %q: %w", file, err)
	}

	actionUser, overlayUser, err := parseKeybindingsFile(raw, file)
	if err != nil {
		return nil, err
	}
	globalLayer = mergeBindings(globalLayer, actionUser)
	for i, userKeys := range overlayUser {
		overlayLayers[i] = mergeBindings(overlayLayers[i], userKeys)
	}
	return buildBundle(globalLayer, overlayLayers)
}

// DefaultBundle returns built-in global + overlay defaults (no keybindings file).
func DefaultBundle() (*Bundle, error) {
	return buildBundle(DefaultActionKeys(), defaultOverlayLayers())
}

func buildBundle(global map[string][]string, overlayLayers []map[string][]string) (*Bundle, error) {
	gMap, err := Build(global)
	if err != nil {
		return nil, err
	}
	overlayMaps := make([]*Map, len(overlayLayers))
	for i, layer := range overlayLayers {
		m, err := Build(layer)
		if err != nil {
			return nil, err
		}
		overlayMaps[i] = m
	}
	if len(overlayMaps) != len(overlayRegistry) {
		return nil, fmt.Errorf("keymap: overlay map count %d != registry %d", len(overlayMaps), len(overlayRegistry))
	}
	return &Bundle{
		Global:         gMap,
		Jobs:           overlayMaps[0],
		Commands:       overlayMaps[1],
		Messages:       overlayMaps[2],
		PathPickerHost: overlayMaps[3],
		DialogInput:    overlayMaps[4],
		RenameDialog:   overlayMaps[5],
		BookmarkDialog: overlayMaps[6],
		FindDialog:     overlayMaps[7],
		HistoryDialog:  overlayMaps[8],
		FlattenDialog:  overlayMaps[9],
	}, nil
}

func parseKeybindingsFile(raw []byte, label string) (actionKeys map[string][]string, overlayKeys []map[string][]string, err error) {
	var top map[string]interface{}
	if err := toml.Unmarshal(raw, &top); err != nil {
		return nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
	}
	if len(top) == 0 {
		emptyOverlays := make([]map[string][]string, len(overlayRegistry))
		for i := range emptyOverlays {
			emptyOverlays[i] = map[string][]string{}
		}
		return map[string][]string{}, emptyOverlays, nil
	}
	allowedTables := map[string]struct{}{config.ActionKeysTable: {}}
	for _, spec := range overlayRegistry {
		allowedTables[spec.TableName] = struct{}{}
	}
	for k := range top {
		if _, ok := allowedTables[k]; !ok {
			return nil, nil, fmt.Errorf("parse keybindings %q: unknown field %q (allowed: action_keys and overlay tables)", label, k)
		}
	}

	actionKeys = map[string][]string{}
	if rawAK, ok := top[config.ActionKeysTable]; ok {
		table, ok := rawAK.(map[string]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("parse keybindings %q: [action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", actionKeys); err != nil {
			return nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range actionKeys {
			if len(keys) == 0 {
				return nil, nil, fmt.Errorf("parse keybindings %q: action %q has empty key list", label, action)
			}
		}
	}

	overlayKeys = make([]map[string][]string, len(overlayRegistry))
	for i, spec := range overlayRegistry {
		overlayKeys[i] = map[string][]string{}
		rawTable, ok := top[spec.TableName]
		if !ok {
			continue
		}
		table, ok := rawTable.(map[string]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("parse keybindings %q: [%s] must be a table", label, spec.TableName)
		}
		if err := collectActionKeys(table, "", overlayKeys[i]); err != nil {
			return nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		if err := validateOverlayKeysFromFile(overlayKeys[i], label, spec); err != nil {
			return nil, nil, err
		}
	}

	return actionKeys, overlayKeys, nil
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
// automatically reflected in every stub written via `--config-stub`
// without further plumbing.
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
		"# sanitize/slugify helpers. Only file.rename.open-sanitize and file.rename.open-slugify.\n" +
		"#\n" +
		"# Bookmarks dialog uses [bookmark_dialog_action_keys] for delete (fzf-marks only).\n" +
		"# Only bookmark.delete is accepted.\n" +
		"#\n" +
		"# Find dialog uses [find_dialog_action_keys] for unselect-all / select-all / select-group / unselect-group.\n" +
		"# Only find.select-all, find.unselect-all, find.select-group, and find.unselect-group are accepted.\n" +
		"#\n" +
		"# History dialog uses [history_dialog_action_keys] for toggling both panels' histories.\n" +
		"# Only panel.history-both-panels is accepted.\n" +
		"#\n" +
		"# Flatten dialog uses [flatten_dialog_action_keys] for destination panel shortcuts.\n" +
		"# Only flatten.destination-active and flatten.destination-inactive are accepted.\n\n"
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
		BookmarkDialogActionKeys map[string][]string `toml:"bookmark_dialog_action_keys"`
		FindDialogActionKeys     map[string][]string `toml:"find_dialog_action_keys"`
		HistoryDialogActionKeys  map[string][]string `toml:"history_dialog_action_keys"`
		FlattenDialogActionKeys  map[string][]string `toml:"flatten_dialog_action_keys"`
	}{
		ActionKeys:               DefaultActionKeys(),
		JobsActionKeys:           DefaultJobsOverlayKeys(),
		CommandsActionKeys:       DefaultCommandsOverlayKeys(),
		MessagesActionKeys:       DefaultMessagesOverlayKeys(),
		PathPickerHostActionKeys: DefaultPathPickerHostOverlayKeys(),
		DialogInputActionKeys:    DefaultDialogInputOverlayKeys(),
		RenameDialogActionKeys:   DefaultRenameDialogOverlayKeys(),
		BookmarkDialogActionKeys: DefaultBookmarkDialogOverlayKeys(),
		FindDialogActionKeys:     DefaultFindDialogOverlayKeys(),
		HistoryDialogActionKeys:  DefaultHistoryDialogOverlayKeys(),
		FlattenDialogActionKeys:  DefaultFlattenDialogOverlayKeys(),
	}
	if err := toml.NewEncoder(w).Encode(payload); err != nil {
		return fmt.Errorf("encode keybindings stub: %w", err)
	}
	return nil
}
