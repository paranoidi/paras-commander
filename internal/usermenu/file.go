package usermenu

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/entrymatch"
)

var menuTopLevelKeys = map[string]struct{}{
	"shell_patterns": {},
	"entry":          {},
}

var menuEntryKeys = map[string]struct{}{
	"key":            {},
	"title":          {},
	"command":        {},
	"toast":          {},
	"when":           {},
	"run_for_each":   {},
	"default":        {},
	"interactive":    {},
	"detach":         {},
	"background":     {},
	"pool":           {},
	"shell":          {},
	"shell_patterns": {},
}

// boolField decodes MC-style 0/1 or a TOML boolean.
type boolField struct {
	Set   bool
	Value bool
}

func (s *boolField) UnmarshalTOML(data interface{}) error {
	s.Set = true
	switch v := data.(type) {
	case bool:
		s.Value = v
	case int64:
		s.Value = v != 0
	case uint64:
		s.Value = v != 0
	case float64:
		s.Value = v != 0
	default:
		return fmt.Errorf("expected bool or numeric 0/1, got %T", data)
	}
	return nil
}

// MenuFile is the decoded user menu definition (menu.toml).
type MenuFile struct {
	ShellPatterns bool
	Entries       []MenuEntry
}

// MenuEntry is one [[entry]] block.
type MenuEntry struct {
	Key           string   `toml:"key"`
	Title         string   `toml:"title"`
	Command       string   `toml:"command"`
	Toast         string   `toml:"toast"`
	When          []string `toml:"when"`
	RunForEach    []string `toml:"run_for_each"`
	Default       bool     `toml:"default"`
	Interactive   bool     `toml:"interactive"`
	Detach        bool     `toml:"detach"`
	Background    bool     `toml:"background"`
	Pool          string   `toml:"pool"`
	Shell         bool     `toml:"shell"`
	ShellPatterns bool     `toml:"shell_patterns"`
}

type menuFileRaw struct {
	ShellPatterns *boolField  `toml:"shell_patterns"`
	Entry         []menuEntry `toml:"entry"`
}

// whenField decodes `when` as either a string or a TOML array of strings.
// A missing value stays unset (empty slice).
type whenField struct {
	Set   bool
	Value []string
}

func (w *whenField) UnmarshalTOML(data interface{}) error {
	w.Set = true
	switch v := data.(type) {
	case string:
		w.Value = []string{v}
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, it := range v {
			s, ok := it.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", it)
			}
			out = append(out, s)
		}
		w.Value = out
	default:
		return fmt.Errorf("expected string or array of strings, got %T", data)
	}
	return nil
}

type menuEntry struct {
	Key           string     `toml:"key"`
	Title         string     `toml:"title"`
	Command       string     `toml:"command"`
	Toast         string     `toml:"toast"`
	When          *whenField `toml:"when"`
	RunForEach    []string   `toml:"run_for_each"`
	Default       bool       `toml:"default"`
	Interactive   *boolField `toml:"interactive"`
	Detach        *boolField `toml:"detach"`
	Background    *boolField `toml:"background"`
	Pool          string     `toml:"pool"`
	Shell         *boolField `toml:"shell"`
	ShellPatterns *boolField `toml:"shell_patterns"`
}

// LoadFile reads and validates menu.toml from path.
func LoadFile(path string) (*MenuFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Decode(b)
}

// PoolNameSet builds a lookup set from configured pool names.
func PoolNameSet(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}

// ValidatePoolRefs reports entries that reference unknown pool names.
func (f *MenuFile) ValidatePoolRefs(known map[string]struct{}) error {
	if f == nil {
		return nil
	}
	for i, e := range f.Entries {
		pool := strings.TrimSpace(e.Pool)
		if pool == "" {
			continue
		}
		if _, ok := known[pool]; !ok {
			return entryError(i, e.Title, fmt.Sprintf("unknown pool %q", pool))
		}
	}
	return nil
}

// Decode parses menu TOML from bytes.
func Decode(data []byte) (*MenuFile, error) {
	var top map[string]interface{}
	if err := toml.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("menu.toml: %w", err)
	}
	if err := validateMenuStructure(top); err != nil {
		return nil, err
	}

	var raw menuFileRaw
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("menu.toml: %w", err)
	}
	out := &MenuFile{ShellPatterns: true}
	if raw.ShellPatterns != nil && raw.ShellPatterns.Set {
		out.ShellPatterns = raw.ShellPatterns.Value
	}

	pinnedKeys := make(map[rune]int)
	defaultCount := 0
	for i, e := range raw.Entry {
		title := strings.TrimSpace(e.Title)
		if title == "" {
			return nil, fmt.Errorf("menu.toml: entry %d: title is required", i)
		}
		if strings.TrimSpace(e.Command) == "" {
			return nil, entryError(i, title, "command is required")
		}
		if err := validateEntryKey(i, title, e.Key, pinnedKeys); err != nil {
			return nil, err
		}
		if e.Default {
			defaultCount++
			if defaultCount > 1 {
				return nil, entryError(i, title, "only one entry may set default = true")
			}
		}

		interactive := false
		if e.Interactive != nil && e.Interactive.Set {
			interactive = e.Interactive.Value
		}
		detach := false
		if e.Detach != nil && e.Detach.Set {
			detach = e.Detach.Value
		}
		background := false
		if e.Background != nil && e.Background.Set {
			background = e.Background.Value
		}
		shell := false
		if e.Shell != nil && e.Shell.Set {
			shell = e.Shell.Value
		}
		modeCount := 0
		if interactive {
			modeCount++
		}
		if detach {
			modeCount++
		}
		if background {
			modeCount++
		}
		if modeCount > 1 {
			return nil, entryError(i, title, "interactive, detach, and background are mutually exclusive")
		}
		pool := strings.TrimSpace(e.Pool)
		if pool != "" && (interactive || detach) {
			return nil, entryError(i, title, "pool cannot be combined with interactive or detach")
		}

		runForEach := make([]string, 0, len(e.RunForEach))
		seen := map[string]bool{}
		for _, raw := range e.RunForEach {
			v := strings.ToLower(strings.TrimSpace(raw))
			if v == "" {
				continue
			}
			if v != "files" && v != "dirs" {
				return nil, entryError(i, title, fmt.Sprintf("run_for_each: invalid value %q (want files/dirs)", raw))
			}
			if !seen[v] {
				seen[v] = true
				runForEach = append(runForEach, v)
			}
		}
		if len(runForEach) > 0 && (interactive || detach) {
			return nil, entryError(i, title, "run_for_each cannot be combined with interactive or detach")
		}
		cmd := strings.TrimSpace(e.Command)
		if len(runForEach) > 0 && !cmdmacro.CommandRequiresMacro(cmd, 'f') {
			return nil, entryError(i, title, "run_for_each requires %f in command")
		}

		var whenList []string
		if e.When != nil && e.When.Set {
			for _, w := range e.When.Value {
				w = strings.TrimSpace(w)
				if w == "" {
					continue
				}
				whenList = append(whenList, w)
			}
		}
		entryShellPatterns := resolveShellPatterns(out.ShellPatterns, e.ShellPatterns)
		if err := entrymatch.ValidateWhenExprs(whenList, entryShellPatterns); err != nil {
			return nil, entryError(i, title, fmt.Sprintf("when: %v", err))
		}

		out.Entries = append(out.Entries, MenuEntry{
			Key:           strings.TrimSpace(e.Key),
			Title:         title,
			Command:       cmd,
			Toast:         strings.TrimSpace(e.Toast),
			When:          whenList,
			RunForEach:    runForEach,
			Default:       e.Default,
			Interactive:   interactive,
			Detach:        detach,
			Background:    background,
			Pool:          pool,
			Shell:         shell,
			ShellPatterns: entryShellPatterns,
		})
	}
	return out, nil
}

func validateMenuStructure(top map[string]interface{}) error {
	for k := range top {
		if _, ok := menuTopLevelKeys[k]; !ok {
			return fmt.Errorf("menu.toml: unknown top-level field %q", k)
		}
	}
	rawEntry, ok := top["entry"]
	if !ok {
		return nil
	}
	entries, err := asEntryTables(rawEntry)
	if err != nil {
		return err
	}
	for i, table := range entries {
		title := entryTitleFromMap(table)
		for k := range table {
			if _, ok := menuEntryKeys[k]; !ok {
				return entryFieldError(i, title, k)
			}
		}
	}
	return nil
}

func asEntryTables(raw interface{}) ([]map[string]interface{}, error) {
	switch v := raw.(type) {
	case []map[string]interface{}:
		return v, nil
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for i, item := range v {
			table, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("menu.toml: entry %d: must be a table", i)
			}
			out = append(out, table)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("menu.toml: entry must be an array of tables")
	}
}

func entryTitleFromMap(table map[string]interface{}) string {
	if v, ok := table["title"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func entryFieldError(i int, title, field string) error {
	if title != "" {
		return fmt.Errorf(`menu.toml: entry %d ("%s"): unknown field %q`, i, title, field)
	}
	return fmt.Errorf(`menu.toml: entry %d: unknown field %q`, i, field)
}

func entryError(i int, title, msg string) error {
	title = strings.TrimSpace(title)
	if title != "" {
		return fmt.Errorf(`menu.toml: entry %d ("%s"): %s`, i, title, msg)
	}
	return fmt.Errorf("menu.toml: entry %d: %s", i, msg)
}

func validateEntryKey(i int, title, key string, pinned map[rune]int) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	runes := []rune(key)
	if len(runes) != 1 || !unicode.IsLetter(runes[0]) {
		return entryError(i, title, fmt.Sprintf("key must be a single letter, got %q", key))
	}
	lr := unicode.ToLower(runes[0])
	if lr == 'c' || lr == 'o' {
		return entryError(i, title, fmt.Sprintf("key %q is reserved (Alt+C/Cancel, Alt+O/OK)", key))
	}
	if prev, dup := pinned[lr]; dup {
		return entryError(i, title, fmt.Sprintf("duplicate key %q (also used by entry %d)", key, prev))
	}
	pinned[lr] = i
	return nil
}

func resolveShellPatterns(fileDefault bool, entry *boolField) bool {
	if entry != nil && entry.Set {
		return entry.Value
	}
	return fileDefault
}
