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

// LoadFromPaths resolves the full Bundle (global + jobs-view + Commands-view overlays)
// using a layered merge per table:
//
//  1. built-in defaults (DefaultActionKeys / DefaultJobsOverlayKeys / DefaultCommandsOverlayKeys)
//  2. config.toml's [action_keys] / [jobs_action_keys] / [commands_action_keys] (when present)
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

	file := strings.TrimSpace(paths.KeybindingsFile)
	if file == "" && strings.TrimSpace(paths.ConfigDir) != "" {
		file = filepath.Join(paths.ConfigDir, "keybindings.toml")
	}
	if file == "" {
		return buildBundle(globalLayer, jobsLayer, commandsLayer)
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return buildBundle(globalLayer, jobsLayer, commandsLayer)
		}
		return nil, fmt.Errorf("read keybindings %q: %w", file, err)
	}

	actionUser, jobsUser, commandsUser, err := parseKeybindingsFile(raw, file)
	if err != nil {
		return nil, err
	}
	globalLayer = mergeBindings(globalLayer, actionUser)
	jobsLayer = mergeBindings(jobsLayer, jobsUser)
	commandsLayer = mergeBindings(commandsLayer, commandsUser)
	return buildBundle(globalLayer, jobsLayer, commandsLayer)
}

// DefaultBundle returns built-in global + overlay defaults (no keybindings file).
func DefaultBundle() (*Bundle, error) {
	return buildBundle(DefaultActionKeys(), DefaultJobsOverlayKeys(), DefaultCommandsOverlayKeys())
}

func buildBundle(global, jobs, commands map[string][]string) (*Bundle, error) {
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
	return &Bundle{Global: gMap, Jobs: jMap, Commands: cMap}, nil
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

// validateJobsOverlayKeys enforces the jobs.* restriction for entries
// supplied via config.toml. The same rule is enforced inside
// parseKeybindingsFile for keybindings.toml; centralising the check via
// AllowedInJobsOverlay keeps both call sites in sync as the registry
// grows.
func validateJobsOverlayKeys(keys map[string][]string, source string) error {
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

func parseKeybindingsFile(raw []byte, label string) (actionKeys, jobsKeys, commandsKeys map[string][]string, err error) {
	var top map[string]interface{}
	if err := toml.Unmarshal(raw, &top); err != nil {
		return nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
	}
	if len(top) == 0 {
		return map[string][]string{}, map[string][]string{}, map[string][]string{}, nil
	}
	for k := range top {
		if k != "action_keys" && k != "jobs_action_keys" && k != "commands_action_keys" {
			return nil, nil, nil, fmt.Errorf("parse keybindings %q: unknown field %q (allowed: action_keys, jobs_action_keys, commands_action_keys)", label, k)
		}
	}

	actionKeys = map[string][]string{}
	if rawAK, ok := top["action_keys"]; ok {
		table, ok := rawAK.(map[string]interface{})
		if !ok {
			return nil, nil, nil, fmt.Errorf("parse keybindings %q: [action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", actionKeys); err != nil {
			return nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range actionKeys {
			if len(keys) == 0 {
				return nil, nil, nil, fmt.Errorf("parse keybindings %q: action %q has empty key list", label, action)
			}
		}
	}

	jobsKeys = map[string][]string{}
	if rawJK, ok := top["jobs_action_keys"]; ok {
		table, ok := rawJK.(map[string]interface{})
		if !ok {
			return nil, nil, nil, fmt.Errorf("parse keybindings %q: [jobs_action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", jobsKeys); err != nil {
			return nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range jobsKeys {
			if len(keys) == 0 {
				return nil, nil, nil, fmt.Errorf("parse keybindings %q: [jobs_action_keys] action %q has empty key list", label, action)
			}
			if !AllowedInJobsOverlay(action) {
				return nil, nil, nil, fmt.Errorf("parse keybindings %q: [jobs_action_keys] action %q is not allowed (jobs.* only)", label, action)
			}
		}
	}

	commandsKeys = map[string][]string{}
	if rawCK, ok := top["commands_action_keys"]; ok {
		table, ok := rawCK.(map[string]interface{})
		if !ok {
			return nil, nil, nil, fmt.Errorf("parse keybindings %q: [commands_action_keys] must be a table", label)
		}
		if err := collectActionKeys(table, "", commandsKeys); err != nil {
			return nil, nil, nil, fmt.Errorf("parse keybindings %q: %w", label, err)
		}
		for action, keys := range commandsKeys {
			if len(keys) == 0 {
				return nil, nil, nil, fmt.Errorf("parse keybindings %q: [commands_action_keys] action %q has empty key list", label, action)
			}
			if !AllowedInCommandsOverlay(action) {
				return nil, nil, nil, fmt.Errorf("parse keybindings %q: [commands_action_keys] action %q is not allowed (commands.* only)", label, action)
			}
		}
	}

	return actionKeys, jobsKeys, commandsKeys, nil
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
		"# while the Commands view is focused. Only commands.* action IDs are accepted.\n\n"
	if _, err := io.WriteString(w, header); err != nil {
		return fmt.Errorf("encode keybindings stub header: %w", err)
	}
	payload := struct {
		ActionKeys          map[string][]string `toml:"action_keys"`
		JobsActionKeys      map[string][]string `toml:"jobs_action_keys"`
		CommandsActionKeys  map[string][]string `toml:"commands_action_keys"`
	}{
		ActionKeys:         DefaultActionKeys(),
		JobsActionKeys:       DefaultJobsOverlayKeys(),
		CommandsActionKeys:   DefaultCommandsOverlayKeys(),
	}
	if err := toml.NewEncoder(w).Encode(payload); err != nil {
		return fmt.Errorf("encode keybindings stub: %w", err)
	}
	return nil
}