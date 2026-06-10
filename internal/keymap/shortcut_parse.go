package keymap

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// ReadMainShortcuts parses the optional [main] table from a keybindings.toml
// style file and returns the action -> chord-strings map it contains.
//
// When the file is missing or has no [main] table, the returned map is nil
// and the error is nil.
func ReadMainShortcuts(filename string) (map[string][]string, error) {
	return readShortcutTable(filename, MainShortcutsTable)
}

// ReadJobsShortcuts parses the optional [jobs] table from a keybindings file.
func ReadJobsShortcuts(filename string) (map[string][]string, error) {
	return readShortcutTable(filename, JobsShortcutsTable)
}

// ReadDialogInputShortcuts parses the optional [dialog.input] table.
func ReadDialogInputShortcuts(filename string) (map[string][]string, error) {
	return readShortcutTable(filename, DialogInputShortcutsTable)
}

// ReadDialogRenameShortcuts parses the optional [dialog.rename] table.
func ReadDialogRenameShortcuts(filename string) (map[string][]string, error) {
	return readShortcutTable(filename, DialogRenameShortcutsTable)
}

// resolveShortcutTableNode locates a shortcut table inside a decoded TOML root map.
// path may be a top-level name (e.g. "main") or a dotted path (e.g. "dialog.input").
func resolveShortcutTableNode(top map[string]interface{}, path string) (map[string]interface{}, bool) {
	parts := strings.Split(path, ".")
	var node interface{} = top
	for _, part := range parts {
		m, ok := node.(map[string]interface{})
		if !ok {
			return nil, false
		}
		child, ok := m[part]
		if !ok {
			return nil, false
		}
		node = child
	}
	table, ok := node.(map[string]interface{})
	return table, ok
}

func readShortcutTable(filename, table string) (map[string][]string, error) {
	if strings.TrimSpace(filename) == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read keybindings %q: %w", filename, err)
	}
	var top map[string]interface{}
	if err := toml.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("parse keybindings %q: %w", filename, err)
	}
	rawTable, ok := resolveShortcutTableNode(top, table)
	if !ok || len(rawTable) == 0 {
		return nil, nil
	}
	out := make(map[string][]string)
	if err := flattenShortcutTable(rawTable, table, "", out); err != nil {
		return nil, fmt.Errorf("parse keybindings %q: %w", filename, err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func flattenShortcutTable(node map[string]interface{}, table, prefix string, out map[string][]string) error {
	for key, value := range node {
		full := key
		if prefix != "" {
			full = prefix + "." + key
		}
		switch typed := value.(type) {
		case map[string]interface{}:
			if err := flattenShortcutTable(typed, table, full, out); err != nil {
				return err
			}
		case []interface{}:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				s, ok := item.(string)
				if !ok {
					return fmt.Errorf("[%s.%s]: expected string list elements", table, full)
				}
				items = append(items, s)
			}
			out[full] = items
		default:
			return fmt.Errorf("[%s.%s]: unsupported type %T", table, full, value)
		}
	}
	return nil
}
