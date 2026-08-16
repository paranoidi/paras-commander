package usermenu

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/entrymatch"
)

// reservedShellPatternsKey is the one scalar root key; every other root key is an entry table.
const reservedShellPatternsKey = "shell_patterns"

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
	"pty":            {},
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

// MenuEntry is one named menu.toml table. A non-nil Entries makes it a submenu container
// (see IsSubmenu); otherwise it is a runnable leaf and Command is required.
type MenuEntry struct {
	Key           string
	Title         string
	Command       string
	Toast         string
	When          []string
	RunForEach    []string
	Default       bool
	Interactive   bool
	Detach        bool
	Background    bool
	PTY           bool
	Pool          string
	Shell         bool
	ShellPatterns bool
	Dialog        bool
	DialogWidth   string
	DialogHeight  string
	Entries       []MenuEntry
}

// IsSubmenu reports whether e is a submenu container (has nested child entries) rather
// than a runnable leaf.
func (e MenuEntry) IsSubmenu() bool {
	return len(e.Entries) > 0
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

// menuEntry is the raw per-table decode target, shared by both leaf entries and submenu
// containers (a submenu container only uses Key/Title/When/Default/ShellPatterns).
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
	PTY           *boolField `toml:"pty"`
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

// ValidatePoolRefs reports entries (including ones nested in submenus) that reference
// unknown pool names.
func (f *MenuFile) ValidatePoolRefs(known map[string]struct{}) error {
	if f == nil {
		return nil
	}
	return validatePoolRefs(f.Entries, known)
}

func validatePoolRefs(entries []MenuEntry, known map[string]struct{}) error {
	for _, e := range entries {
		if e.IsSubmenu() {
			if err := validatePoolRefs(e.Entries, known); err != nil {
				return err
			}
			continue
		}
		pool := strings.TrimSpace(e.Pool)
		if pool == "" {
			continue
		}
		if _, ok := known[pool]; !ok {
			return fmt.Errorf("menu.toml: %q: unknown pool %q", strings.TrimSpace(e.Title), pool)
		}
	}
	return nil
}

// tableHeaderRe matches a bare `[name]` or `[a.b.c]` table header line (an optional
// trailing comment is allowed).
var tableHeaderRe = regexp.MustCompile(`^\[([A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*)\]\s*(#.*)?$`)

// scanTableHeaders reads the raw source once and returns every `[name]` / `[a.b.c]` table
// header's dotted path, in file order. It fails fast with a migration-guiding error if a
// `[[...]]` array-of-tables header is found anywhere in the file.
func scanTableHeaders(data []byte) ([]string, error) {
	var order []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[[") {
			return nil, fmt.Errorf("menu.toml: [[entry]] array syntax is no longer supported — entries are now named tables, e.g. [tools] / [tools.disk_use]")
		}
		if !strings.HasPrefix(line, "[") {
			continue
		}
		if m := tableHeaderRe.FindStringSubmatch(line); m != nil {
			order = append(order, m[1])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("menu.toml: %w", err)
	}
	return order, nil
}

// sortByOrder sorts names (direct children of parentPath, "" for the root level) into file
// header order using orderIndex (dotted path -> header index). Names with no matching
// header (shouldn't happen for well-formed files) sort last.
func sortByOrder(names []string, orderIndex map[string]int, parentPath string) {
	pos := func(name string) int {
		path := name
		if parentPath != "" {
			path = parentPath + "." + name
		}
		if idx, ok := orderIndex[path]; ok {
			return idx
		}
		return len(orderIndex)
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && pos(names[j-1]) > pos(names[j]); j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
}

// Decode parses menu TOML from bytes.
func Decode(data []byte) (*MenuFile, error) {
	order, err := scanTableHeaders(data)
	if err != nil {
		return nil, err
	}
	orderIndex := make(map[string]int, len(order))
	for i, p := range order {
		orderIndex[p] = i
	}

	var top map[string]toml.Primitive
	meta, err := toml.Decode(string(data), &top)
	if err != nil {
		return nil, fmt.Errorf("menu.toml: %w", err)
	}

	out := &MenuFile{ShellPatterns: true}
	if sp, ok := top[reservedShellPatternsKey]; ok {
		var bf boolField
		if err := meta.PrimitiveDecode(sp, &bf); err != nil {
			return nil, entryError(reservedShellPatternsKey, err.Error())
		}
		if bf.Set {
			out.ShellPatterns = bf.Value
		}
	}

	var rootNames []string
	for name := range top {
		if name == reservedShellPatternsKey {
			continue
		}
		rootNames = append(rootNames, name)
	}
	sortByOrder(rootNames, orderIndex, "")

	pinnedKeys := make(map[rune]string)
	defaultCount := 0
	for _, name := range rootNames {
		entry, err := decodeEntryAtPath(meta, top[name], name, orderIndex, out.ShellPatterns, pinnedKeys, &defaultCount)
		if err != nil {
			return nil, err
		}
		out.Entries = append(out.Entries, entry)
	}
	return out, nil
}

// decodeEntryAtPath decodes the table at the dotted path into a MenuEntry: a submenu
// container when it has nested child tables, otherwise a runnable leaf. pinnedKeys and
// defaultCount enforce "unique key=" / "only one default=true" among siblings at this level.
func decodeEntryAtPath(meta toml.MetaData, prim toml.Primitive, path string, orderIndex map[string]int, shellPatternsDefault bool, pinnedKeys map[rune]string, defaultCount *int) (MenuEntry, error) {
	// Discover child tables and validate field names from the untyped map first: decoding
	// straight into the typed menuEntry struct would fail with a confusing type-mismatch
	// error if a child table happens to share a name with a scalar field (e.g. [tools.command]).
	var flat map[string]interface{}
	if err := meta.PrimitiveDecode(prim, &flat); err != nil {
		return MenuEntry{}, fmt.Errorf("menu.toml: [%s]: %w", path, err)
	}
	var childNames []string
	for k, v := range flat {
		if _, isTable := v.(map[string]interface{}); isTable {
			if _, reserved := menuEntryKeys[k]; reserved {
				return MenuEntry{}, fmt.Errorf("menu.toml: [%s.%s]: %q is a reserved field name, cannot be used as a submenu table", path, k, k)
			}
			childNames = append(childNames, k)
			continue
		}
		if _, known := menuEntryKeys[k]; !known {
			return MenuEntry{}, entryFieldError(path, k)
		}
	}

	var raw menuEntry
	if err := meta.PrimitiveDecode(prim, &raw); err != nil {
		return MenuEntry{}, fmt.Errorf("menu.toml: [%s]: %w", path, err)
	}
	title := strings.TrimSpace(raw.Title)
	if title == "" {
		return MenuEntry{}, fmt.Errorf("menu.toml: [%s]: title is required", path)
	}

	if err := validateEntryKey(path, raw.Key, pinnedKeys); err != nil {
		return MenuEntry{}, err
	}
	if raw.Default {
		(*defaultCount)++
		if *defaultCount > 1 {
			return MenuEntry{}, entryError(path, "only one entry may set default = true")
		}
	}

	entryShellPatterns := resolveShellPatterns(shellPatternsDefault, raw.ShellPatterns)

	if len(childNames) > 0 {
		return decodeSubmenuEntry(meta, prim, path, title, raw, childNames, orderIndex, entryShellPatterns)
	}
	return decodeLeafEntry(path, title, raw, entryShellPatterns)
}

// decodeSubmenuEntry decodes a table with nested child tables into a submenu container,
// recursing into each child in file-header order with fresh pinnedKeys/defaultCount (sibling
// key=/default= uniqueness is scoped per menu level).
func decodeSubmenuEntry(meta toml.MetaData, prim toml.Primitive, path, title string, raw menuEntry, childNames []string, orderIndex map[string]int, entryShellPatterns bool) (MenuEntry, error) {
	if err := validateSubmenuMutualExclusion(path, raw); err != nil {
		return MenuEntry{}, err
	}
	whenList, err := decodeWhenList(path, raw, entryShellPatterns)
	if err != nil {
		return MenuEntry{}, err
	}

	var primMap map[string]toml.Primitive
	if err := meta.PrimitiveDecode(prim, &primMap); err != nil {
		return MenuEntry{}, fmt.Errorf("menu.toml: [%s]: %w", path, err)
	}

	sortByOrder(childNames, orderIndex, path)
	childPinned := make(map[rune]string)
	childDefault := 0
	children := make([]MenuEntry, 0, len(childNames))
	for _, name := range childNames {
		childPath := path + "." + name
		child, err := decodeEntryAtPath(meta, primMap[name], childPath, orderIndex, entryShellPatterns, childPinned, &childDefault)
		if err != nil {
			return MenuEntry{}, err
		}
		children = append(children, child)
	}

	return MenuEntry{
		Key:           strings.TrimSpace(raw.Key),
		Title:         title,
		When:          whenList,
		Default:       raw.Default,
		ShellPatterns: entryShellPatterns,
		Entries:       children,
	}, nil
}

// validateSubmenuMutualExclusion enforces that a table with nested child tables does not
// also set any leaf-only field (mutually exclusive with being a submenu container).
func validateSubmenuMutualExclusion(path string, raw menuEntry) error {
	const suffix = "cannot be combined with a submenu (this table has nested entries)"
	if strings.TrimSpace(raw.Command) != "" {
		return entryError(path, "command "+suffix)
	}
	if len(raw.RunForEach) > 0 {
		return entryError(path, "run_for_each "+suffix)
	}
	if strings.TrimSpace(raw.Pool) != "" {
		return entryError(path, "pool "+suffix)
	}
	if strings.TrimSpace(raw.Toast) != "" {
		return entryError(path, "toast "+suffix)
	}
	if raw.Interactive != nil && raw.Interactive.Set && raw.Interactive.Value {
		return entryError(path, "interactive "+suffix)
	}
	if raw.Detach != nil && raw.Detach.Set && raw.Detach.Value {
		return entryError(path, "detach "+suffix)
	}
	if raw.Background != nil && raw.Background.Set && raw.Background.Value {
		return entryError(path, "background "+suffix)
	}
	if raw.Dialog != nil && raw.Dialog.Set && raw.Dialog.Value {
		return entryError(path, "dialog "+suffix)
	}
	if raw.PTY != nil && raw.PTY.Set && raw.PTY.Value {
		return entryError(path, "pty "+suffix)
	}
	return nil
}

// decodeWhenList resolves and validates the `when` visibility expressions shared by leaf
// entries and submenu containers.
func decodeWhenList(path string, raw menuEntry, entryShellPatterns bool) ([]string, error) {
	var whenList []string
	if raw.When != nil && raw.When.Set {
		for _, w := range raw.When.Value {
			w = strings.TrimSpace(w)
			if w == "" {
				continue
			}
			whenList = append(whenList, w)
		}
	}
	if err := entrymatch.ValidateWhenExprs(whenList, entryShellPatterns); err != nil {
		return nil, entryError(path, fmt.Sprintf("when: %v", err))
	}
	return whenList, nil
}

// entryModes holds one entry's resolved interactive/detach/background/dialog/shell booleans,
// as decoded by validateEntryModes.
type entryModes struct {
	interactive bool
	detach      bool
	background  bool
	dialog      bool
	shell       bool
	pty         bool
}

// validateEntryModes resolves raw's mode booleans (MC-style 0/1 or TOML bool, defaulting to
// false when unset) and enforces that interactive/detach/background/dialog are mutually
// exclusive and that pool cannot be combined with interactive or detach.
func validateEntryModes(path string, raw menuEntry, pool string) (entryModes, error) {
	m := entryModes{}
	if raw.Interactive != nil && raw.Interactive.Set {
		m.interactive = raw.Interactive.Value
	}
	if raw.Detach != nil && raw.Detach.Set {
		m.detach = raw.Detach.Value
	}
	if raw.Background != nil && raw.Background.Set {
		m.background = raw.Background.Value
	}
	if raw.Shell != nil && raw.Shell.Set {
		m.shell = raw.Shell.Value
	}
	if raw.Dialog != nil && raw.Dialog.Set {
		m.dialog = raw.Dialog.Value
	}
	if raw.PTY != nil && raw.PTY.Set {
		m.pty = raw.PTY.Value
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
		return entryModes{}, entryError(path, "interactive, detach, background, and dialog are mutually exclusive")
	}
	if pool != "" && (m.interactive || m.detach) {
		return entryModes{}, entryError(path, "pool cannot be combined with interactive or detach")
	}
	return m, nil
}

// decodeRunForEach lowercases/dedupes rawValues into the files/dirs run_for_each list and
// enforces its combination rules with interactive/detach/dialog, pty requiring run_for_each,
// and the required %f macro.
func decodeRunForEach(path string, rawValues []string, interactive, detach, dialog, pty bool, cmd string) ([]string, error) {
	runForEach := make([]string, 0, len(rawValues))
	seen := map[string]bool{}
	for _, raw := range rawValues {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" {
			continue
		}
		if v != "files" && v != "dirs" {
			return nil, entryError(path, fmt.Sprintf("run_for_each: invalid value %q (want files/dirs)", raw))
		}
		if !seen[v] {
			seen[v] = true
			runForEach = append(runForEach, v)
		}
	}
	if len(runForEach) > 0 && (interactive || detach) {
		return nil, entryError(path, "run_for_each cannot be combined with interactive or detach")
	}
	if len(runForEach) > 0 && dialog {
		return nil, entryError(path, "run_for_each cannot be combined with dialog")
	}
	if len(runForEach) > 0 && !cmdmacro.CommandRequiresMacro(cmd, 'f') {
		return nil, entryError(path, "run_for_each requires %f in command")
	}
	if pty && len(runForEach) == 0 {
		return nil, entryError(path, "pty requires run_for_each")
	}
	return runForEach, nil
}

// decodeLeafEntry validates and converts a raw table with no child tables (path, for error
// messages) into a runnable leaf MenuEntry.
func decodeLeafEntry(path, title string, raw menuEntry, entryShellPatterns bool) (MenuEntry, error) {
	if strings.TrimSpace(raw.Command) == "" {
		return MenuEntry{}, entryError(path, "command is required")
	}

	pool := strings.TrimSpace(raw.Pool)
	modes, err := validateEntryModes(path, raw, pool)
	if err != nil {
		return MenuEntry{}, err
	}

	dialogWidth := ""
	if raw.DialogWidth != nil && raw.DialogWidth.Set {
		dialogWidth = raw.DialogWidth.Value
		if err := validateDimSpec(dialogWidth); err != nil {
			return MenuEntry{}, entryError(path, fmt.Sprintf("dialog_width: %v", err))
		}
	}
	dialogHeight := ""
	if raw.DialogHeight != nil && raw.DialogHeight.Set {
		dialogHeight = raw.DialogHeight.Value
		if err := validateDimSpec(dialogHeight); err != nil {
			return MenuEntry{}, entryError(path, fmt.Sprintf("dialog_height: %v", err))
		}
	}

	cmd := strings.TrimSpace(raw.Command)
	runForEach, err := decodeRunForEach(path, raw.RunForEach, modes.interactive, modes.detach, modes.dialog, modes.pty, cmd)
	if err != nil {
		return MenuEntry{}, err
	}

	whenList, err := decodeWhenList(path, raw, entryShellPatterns)
	if err != nil {
		return MenuEntry{}, err
	}

	return MenuEntry{
		Key:           strings.TrimSpace(raw.Key),
		Title:         title,
		Command:       cmd,
		Toast:         strings.TrimSpace(raw.Toast),
		When:          whenList,
		RunForEach:    runForEach,
		Default:       raw.Default,
		Interactive:   modes.interactive,
		Detach:        modes.detach,
		Background:    modes.background,
		PTY:           modes.pty,
		Pool:          pool,
		Shell:         modes.shell,
		ShellPatterns: entryShellPatterns,
		Dialog:        modes.dialog,
		DialogWidth:   dialogWidth,
		DialogHeight:  dialogHeight,
	}, nil
}

func entryFieldError(path, field string) error {
	return fmt.Errorf("menu.toml: [%s]: unknown field %q", path, field)
}

func entryError(path, msg string) error {
	return fmt.Errorf("menu.toml: [%s]: %s", path, msg)
}

func validateEntryKey(path, key string, pinned map[rune]string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	runes := []rune(key)
	if len(runes) != 1 || !unicode.IsLetter(runes[0]) {
		return entryError(path, fmt.Sprintf("key must be a single letter, got %q", key))
	}
	lr := unicode.ToLower(runes[0])
	if prev, dup := pinned[lr]; dup {
		return entryError(path, fmt.Sprintf("duplicate key %q (also used by [%s])", key, prev))
	}
	pinned[lr] = path
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
