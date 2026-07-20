package usermenu

import (
	"fmt"
	"os"
	"strconv"
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
	"dialog":         {},
	"dialog_width":   {},
	"dialog_height":  {},
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
	Dialog        bool     `toml:"dialog"`
	DialogWidth   string   `toml:"dialog_width"`
	DialogHeight  string   `toml:"dialog_height"`
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

// dimField decodes dialog_width / dialog_height as either a quoted string ("80%", "100")
// or a bare TOML integer (80, 100) for user convenience.
type dimField struct {
	Set   bool
	Value string
}

func (d *dimField) UnmarshalTOML(data any) error {
	d.Set = true
	switch v := data.(type) {
	case string:
		d.Value = strings.TrimSpace(v)
	case int64:
		d.Value = fmt.Sprintf("%d", v)
	case uint64:
		d.Value = fmt.Sprintf("%d", v)
	default:
		return fmt.Errorf("expected integer or string, got %T", data)
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
	Dialog        *boolField `toml:"dialog"`
	DialogWidth   *dimField  `toml:"dialog_width"`
	DialogHeight  *dimField  `toml:"dialog_height"`
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
		entry, err := decodeEntry(i, e, out.ShellPatterns, pinnedKeys, &defaultCount)
		if err != nil {
			return nil, err
		}
		out.Entries = append(out.Entries, entry)
	}
	return out, nil
}

// entryModes holds one entry's resolved interactive/detach/background/dialog/shell booleans,
// as decoded by validateEntryModes.
type entryModes struct {
	interactive bool
	detach      bool
	background  bool
	dialog      bool
	shell       bool
}

// validateEntryModes resolves e's mode booleans (MC-style 0/1 or TOML bool, defaulting to
// false when unset) and enforces that interactive/detach/background/dialog are mutually
// exclusive and that pool cannot be combined with interactive or detach.
func validateEntryModes(i int, title string, e menuEntry, pool string) (entryModes, error) {
	m := entryModes{}
	if e.Interactive != nil && e.Interactive.Set {
		m.interactive = e.Interactive.Value
	}
	if e.Detach != nil && e.Detach.Set {
		m.detach = e.Detach.Value
	}
	if e.Background != nil && e.Background.Set {
		m.background = e.Background.Value
	}
	if e.Shell != nil && e.Shell.Set {
		m.shell = e.Shell.Value
	}
	if e.Dialog != nil && e.Dialog.Set {
		m.dialog = e.Dialog.Value
	}
	modeCount := 0
	if m.interactive {
		modeCount++
	}
	if m.detach {
		modeCount++
	}
	if m.background {
		modeCount++
	}
	if m.dialog {
		modeCount++
	}
	if modeCount > 1 {
		return entryModes{}, entryError(i, title, "interactive, detach, background, and dialog are mutually exclusive")
	}
	if pool != "" && (m.interactive || m.detach) {
		return entryModes{}, entryError(i, title, "pool cannot be combined with interactive or detach")
	}
	return m, nil
}

// decodeRunForEach lowercases/dedupes rawValues into the files/dirs run_for_each list and
// enforces its combination rules with interactive/detach/dialog and the required %f macro.
func decodeRunForEach(i int, title string, rawValues []string, interactive, detach, dialog bool, cmd string) ([]string, error) {
	runForEach := make([]string, 0, len(rawValues))
	seen := map[string]bool{}
	for _, raw := range rawValues {
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
	if len(runForEach) > 0 && dialog {
		return nil, entryError(i, title, "run_for_each cannot be combined with dialog")
	}
	if len(runForEach) > 0 && !cmdmacro.CommandRequiresMacro(cmd, 'f') {
		return nil, entryError(i, title, "run_for_each requires %f in command")
	}
	return runForEach, nil
}

// decodeEntry validates and converts one raw [[entry]] table (index i) into a MenuEntry.
// defaultCount is shared across all entries in the file to enforce "only one default = true".
func decodeEntry(i int, e menuEntry, shellPatternsDefault bool, pinnedKeys map[rune]int, defaultCount *int) (MenuEntry, error) {
	title := strings.TrimSpace(e.Title)
	if title == "" {
		return MenuEntry{}, fmt.Errorf("menu.toml: entry %d: title is required", i)
	}
	if strings.TrimSpace(e.Command) == "" {
		return MenuEntry{}, entryError(i, title, "command is required")
	}
	if err := validateEntryKey(i, title, e.Key, pinnedKeys); err != nil {
		return MenuEntry{}, err
	}
	if e.Default {
		(*defaultCount)++
		if *defaultCount > 1 {
			return MenuEntry{}, entryError(i, title, "only one entry may set default = true")
		}
	}

	pool := strings.TrimSpace(e.Pool)
	modes, err := validateEntryModes(i, title, e, pool)
	if err != nil {
		return MenuEntry{}, err
	}

	dialogWidth := ""
	if e.DialogWidth != nil && e.DialogWidth.Set {
		dialogWidth = e.DialogWidth.Value
		if err := validateDimSpec(dialogWidth); err != nil {
			return MenuEntry{}, entryError(i, title, fmt.Sprintf("dialog_width: %v", err))
		}
	}
	dialogHeight := ""
	if e.DialogHeight != nil && e.DialogHeight.Set {
		dialogHeight = e.DialogHeight.Value
		if err := validateDimSpec(dialogHeight); err != nil {
			return MenuEntry{}, entryError(i, title, fmt.Sprintf("dialog_height: %v", err))
		}
	}

	cmd := strings.TrimSpace(e.Command)
	runForEach, err := decodeRunForEach(i, title, e.RunForEach, modes.interactive, modes.detach, modes.dialog, cmd)
	if err != nil {
		return MenuEntry{}, err
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
	entryShellPatterns := resolveShellPatterns(shellPatternsDefault, e.ShellPatterns)
	if err := entrymatch.ValidateWhenExprs(whenList, entryShellPatterns); err != nil {
		return MenuEntry{}, entryError(i, title, fmt.Sprintf("when: %v", err))
	}

	return MenuEntry{
		Key:           strings.TrimSpace(e.Key),
		Title:         title,
		Command:       cmd,
		Toast:         strings.TrimSpace(e.Toast),
		When:          whenList,
		RunForEach:    runForEach,
		Default:       e.Default,
		Interactive:   modes.interactive,
		Detach:        modes.detach,
		Background:    modes.background,
		Pool:          pool,
		Shell:         modes.shell,
		ShellPatterns: entryShellPatterns,
		Dialog:        modes.dialog,
		DialogWidth:   dialogWidth,
		DialogHeight:  dialogHeight,
	}, nil
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
	if prev, dup := pinned[lr]; dup {
		return entryError(i, title, fmt.Sprintf("duplicate key %q (also used by entry %d)", key, prev))
	}
	pinned[lr] = i
	return nil
}

// validateDimSpec checks that a dialog_width or dialog_height value is a positive integer
// or a percentage string like "80%" (1–100).
func validateDimSpec(spec string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	if pct, ok := strings.CutSuffix(spec, "%"); ok {
		f, err := strconv.ParseFloat(pct, 64)
		if err != nil || f <= 0 || f > 100 {
			return fmt.Errorf("invalid percentage %q (want 1–100%%)", spec)
		}
		return nil
	}
	n, err := strconv.Atoi(spec)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid dimension %q (want positive integer or e.g. \"80%%\")", spec)
	}
	return nil
}

func resolveShellPatterns(fileDefault bool, entry *boolField) bool {
	if entry != nil && entry.Set {
		return entry.Value
	}
	return fileDefault
}
