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

// LoadFromPaths resolves the full Bundle (global + view + dialog overlays) using a layered merge per table:
//
//  1. built-in defaults (DefaultActionKeys / DefaultJobsOverlayKeys / DefaultCommandsOverlayKeys /
//     DefaultMessagesOverlayKeys / DefaultDialogInputOverlayKeys / DefaultRenameDialogOverlayKeys /
//     DefaultBookmarkDialogOverlayKeys / DefaultFindDialogOverlayKeys / DefaultHistoryDialogOverlayKeys /
//     DefaultFlattenDialogOverlayKeys)
//  2. keybindings.toml's matching tables (when present)
//
// keybindings.toml can be absent without failing startup; built-in defaults
// remain wherever no overrides are supplied. Layering applies independently
// per table so a user can override only the global map, only one overlay,
// or any combination.
func LoadFromPaths(paths config.Paths) (*Bundle, error) {
	globalLayer := DefaultActionKeys()
	overlayLayers := defaultOverlayLayers()

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

	mainUser, overlayUser, err := parseKeybindingsFile(raw, file)
	if err != nil {
		return nil, err
	}
	globalLayer = mergeBindings(globalLayer, mainUser)
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
		DialogInput:    overlayMaps[3],
		RenameDialog:   overlayMaps[4],
		BookmarkDialog: overlayMaps[5],
		FindDialog:     overlayMaps[6],
		HistoryDialog:  overlayMaps[7],
		FlattenDialog:  overlayMaps[8],
	}, nil
}

func parseKeybindingsFile(raw []byte, label string) (mainKeys map[string][]string, overlayKeys []map[string][]string, err error) {
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
	if err := validateKeybindingsTopLevel(top, label); err != nil {
		return nil, nil, err
	}

	mainKeys = map[string][]string{}
	if rawMain, ok := top[MainShortcutsTable]; ok {
		table, ok := rawMain.(map[string]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("parse keybindings %q: [main] must be a table", label)
		}
		if err := collectActionKeys(table, "", mainKeys); err != nil {
			return nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range mainKeys {
			if len(keys) == 0 {
				return nil, nil, fmt.Errorf("parse keybindings %q: action %q has empty key list", label, action)
			}
		}
	}

	overlayKeys = make([]map[string][]string, len(overlayRegistry))
	for i, spec := range overlayRegistry {
		overlayKeys[i] = map[string][]string{}
		rawTable, ok := resolveShortcutTableNode(top, spec.TableName)
		if !ok {
			continue
		}
		if err := collectActionKeys(rawTable, "", overlayKeys[i]); err != nil {
			return nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		if err := validateOverlayKeysFromFile(overlayKeys[i], label, spec); err != nil {
			return nil, nil, err
		}
	}

	return mainKeys, overlayKeys, nil
}

func validateKeybindingsTopLevel(top map[string]interface{}, label string) error {
	for k, v := range top {
		switch k {
		case MainShortcutsTable, JobsShortcutsTable, CommandsShortcutsTable, MessagesShortcutsTable:
			if _, ok := v.(map[string]interface{}); !ok {
				return fmt.Errorf("parse keybindings %q: [%s] must be a table", label, k)
			}
		case DialogShortcutsGroup:
			dialogTable, ok := v.(map[string]interface{})
			if !ok {
				return fmt.Errorf("parse keybindings %q: [dialog] must be a table", label)
			}
			for sub, subVal := range dialogTable {
				if !IsDialogShortcutSubtable(sub) {
					return fmt.Errorf("parse keybindings %q: unknown [dialog.%s] sub-table", label, sub)
				}
				if _, ok := subVal.(map[string]interface{}); !ok {
					return fmt.Errorf("parse keybindings %q: [dialog.%s] must be a table", label, sub)
				}
			}
		default:
			return fmt.Errorf("parse keybindings %q: unknown field %q (allowed: main, jobs, commands, messages, dialog)", label, k)
		}
	}
	return nil
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

type dialogShortcuts struct {
	Input    map[string][]string `toml:"input"`
	Rename   map[string][]string `toml:"rename"`
	Bookmark map[string][]string `toml:"bookmark"`
	Find     map[string][]string `toml:"find"`
	History  map[string][]string `toml:"history"`
	Flatten  map[string][]string `toml:"flatten"`
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
	header := "# Global shortcuts under [main]. Each value is a list of\n" +
		"# chord strings (single-stroke). See docs/keybindings.md for syntax.\n" +
		"#\n" +
		"# View overlays ([jobs], [commands], [messages]) take precedence over\n" +
		"# [main] while that view is focused.\n" +
		"#\n" +
		"# Dialog overlays ([dialog.input], [dialog.rename], …) apply only while\n" +
		"# the matching dialog context is focused.\n" +
		"#\n" +
		"# Fuzzy path picker on copy/move destination or symlink/hardlink path rows\n" +
		"# uses the same chords as bookmark.open under [main].\n" +
		"#\n" +
		"# [dialog.input] — ui.input.* only (e.g. restore default placeholder).\n" +
		"# [dialog.rename] — file.rename.open-sanitize and file.rename.open-slugify.\n" +
		"# [dialog.bookmark] — bookmark.delete (fzf-marks only).\n" +
		"# [dialog.find] — find.select-all, find.unselect-all, find.select-group, find.unselect-group.\n" +
		"# [dialog.history] — panel.history-both-panels.\n" +
		"# [dialog.flatten] — flatten.destination-active and flatten.destination-inactive.\n\n"
	if _, err := io.WriteString(w, header); err != nil {
		return fmt.Errorf("encode keybindings stub header: %w", err)
	}
	payload := struct {
		Main     map[string][]string `toml:"main"`
		Jobs     map[string][]string `toml:"jobs"`
		Commands map[string][]string `toml:"commands"`
		Messages map[string][]string `toml:"messages"`
		Dialog   dialogShortcuts     `toml:"dialog"`
	}{
		Main:     DefaultActionKeys(),
		Jobs:     DefaultJobsOverlayKeys(),
		Commands: DefaultCommandsOverlayKeys(),
		Messages: DefaultMessagesOverlayKeys(),
		Dialog: dialogShortcuts{
			Input:    DefaultDialogInputOverlayKeys(),
			Rename:   DefaultRenameDialogOverlayKeys(),
			Bookmark: DefaultBookmarkDialogOverlayKeys(),
			Find:     DefaultFindDialogOverlayKeys(),
			History:  DefaultHistoryDialogOverlayKeys(),
			Flatten:  DefaultFlattenDialogOverlayKeys(),
		},
	}
	if err := toml.NewEncoder(w).Encode(payload); err != nil {
		return fmt.Errorf("encode keybindings stub: %w", err)
	}
	return nil
}
