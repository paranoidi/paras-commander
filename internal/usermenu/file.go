package usermenu

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

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
	Key         string `toml:"key"`
	Title       string `toml:"title"`
	Command     string `toml:"command"`
	When        string `toml:"when"`
	Default     bool   `toml:"default"`
	Interactive bool   `toml:"interactive"`
	Detach      bool   `toml:"detach"`
	Background  bool   `toml:"background"`
}

type menuFileRaw struct {
	ShellPatterns *boolField  `toml:"shell_patterns"`
	Entry         []menuEntry `toml:"entry"`
}

type menuEntry struct {
	Key         string     `toml:"key"`
	Title       string     `toml:"title"`
	Command     string     `toml:"command"`
	When        string     `toml:"when"`
	Default     bool       `toml:"default"`
	Interactive *boolField `toml:"interactive"`
	Detach      *boolField `toml:"detach"`
	Background  *boolField `toml:"background"`
}

// LoadFile reads and validates menu.toml from path.
func LoadFile(path string) (*MenuFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Decode(b)
}

// Decode parses menu TOML from bytes.
func Decode(data []byte) (*MenuFile, error) {
	var raw menuFileRaw
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("menu.toml: %w", err)
	}
	out := &MenuFile{ShellPatterns: true}
	if raw.ShellPatterns != nil && raw.ShellPatterns.Set {
		out.ShellPatterns = raw.ShellPatterns.Value
	}
	for i, e := range raw.Entry {
		if strings.TrimSpace(e.Title) == "" {
			return nil, fmt.Errorf("menu.toml: entry %d: title is required", i)
		}
		if strings.TrimSpace(e.Command) == "" {
			return nil, fmt.Errorf("menu.toml: entry %d: command is required", i)
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
			return nil, fmt.Errorf("menu.toml: entry %d: interactive, detach, and background are mutually exclusive", i)
		}
		out.Entries = append(out.Entries, MenuEntry{
			Key:         strings.TrimSpace(e.Key),
			Title:       strings.TrimSpace(e.Title),
			Command:     strings.TrimSpace(e.Command),
			When:        strings.TrimSpace(e.When),
			Default:     e.Default,
			Interactive: interactive,
			Detach:      detach,
			Background:  background,
		})
	}
	return out, nil
}
