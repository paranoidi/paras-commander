// Package statuscmds loads status.toml: the shell commands whose output is drawn as
// segments at the top-left of the menu bar, and which of them are shown.
package statuscmds

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/paranoidi/paras-commander/internal/config"
)

// Def is one status command (a built-in or a [[command]] table in status.toml).
type Def struct {
	// Name is the stable identifier listed in show.
	Name string
	// Label is the checkbox text in the status-commands dialog.
	Label string
	// Command is a shell command line run via sh -c (no macro expansion).
	Command string
	// IntervalMS is how often Command runs.
	IntervalMS int
	// MaxWidth caps the displayed text's column width (longer output is ellipsized).
	MaxWidth int
	// Color is the text's foreground as a theme color value (palette name, "#RRGGBB" or
	// "default"), resolved against the active theme; empty uses the menu.status style.
	Color string
}

// File is the decoded status.toml.
type File struct {
	// Show lists the names of the commands drawn in the menu bar, in order.
	Show     []string
	Commands []Def
}

type fileRaw struct {
	Show    []string     `toml:"show"`
	Command []commandRaw `toml:"command"`
}

type commandRaw struct {
	Name       string `toml:"name"`
	Label      string `toml:"label"`
	Command    string `toml:"command"`
	IntervalMS int    `toml:"interval_ms"`
	MaxWidth   int    `toml:"max_width"`
	Color      string `toml:"color"`
}

// Path returns status.toml under configDir ("" when configDir is unknown).
func Path(configDir string) string {
	if strings.TrimSpace(configDir) == "" {
		return ""
	}
	return filepath.Join(configDir, config.DefaultStatusCommandsFileName)
}

// Decode parses status.toml from bytes, filling defaults and clamping numeric fields.
func Decode(data []byte) (*File, error) {
	var raw fileRaw
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("status.toml: %w", err)
	}
	out := &File{Show: raw.Show}
	seen := make(map[string]struct{}, len(raw.Command))
	for i, c := range raw.Command {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			return nil, fmt.Errorf("status.toml: command %d: name is required", i)
		}
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("status.toml: duplicate command name %q", name)
		}
		seen[name] = struct{}{}
		if strings.TrimSpace(c.Command) == "" {
			return nil, fmt.Errorf("status.toml: command %q: command is required", name)
		}
		out.Commands = append(out.Commands, normalize(Def{
			Name:       name,
			Label:      strings.TrimSpace(c.Label),
			Command:    c.Command,
			IntervalMS: c.IntervalMS,
			MaxWidth:   c.MaxWidth,
			Color:      strings.TrimSpace(c.Color),
		}))
	}
	return out, nil
}

// normalize fills Label and clamps IntervalMS / MaxWidth to the config bounds.
func normalize(d Def) Def {
	if d.Label == "" {
		d.Label = d.Name
	}
	if d.IntervalMS < config.StatusCommandIntervalMinMS {
		d.IntervalMS = config.DefaultStatusCommandIntervalMS
	}
	if d.MaxWidth < config.StatusCommandMaxWidthMin {
		d.MaxWidth = config.DefaultStatusCommandMaxWidth
	}
	d.MaxWidth = min(d.MaxWidth, config.StatusCommandMaxWidthMax)
	return d
}

// LoadFile reads status.toml from path. A missing file decodes as an empty File.
func LoadFile(path string) (*File, error) {
	if path == "" {
		return &File{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &File{}, nil
		}
		return nil, err
	}
	return Decode(b)
}

// Set is the resolved list of status commands and the shown subset.
type Set struct {
	// Defs is every available command: built-ins first (user entries with the same name
	// replace them in place), then user-only entries in file order.
	Defs []Def
	// Show is the shown names in order, limited to names in Defs and deduplicated.
	Show []string
}

// Resolve merges Builtins with f's [[command]] entries and filters f.Show.
func Resolve(f *File) Set {
	var s Set
	index := map[string]int{}
	for _, d := range Builtins() {
		index[d.Name] = len(s.Defs)
		s.Defs = append(s.Defs, d)
	}
	if f == nil {
		return s
	}
	for _, d := range f.Commands {
		if i, ok := index[d.Name]; ok {
			s.Defs[i] = d
			continue
		}
		index[d.Name] = len(s.Defs)
		s.Defs = append(s.Defs, d)
	}
	shown := map[string]struct{}{}
	for _, name := range f.Show {
		name = strings.TrimSpace(name)
		if _, ok := index[name]; !ok {
			continue
		}
		if _, dup := shown[name]; dup {
			continue
		}
		shown[name] = struct{}{}
		s.Show = append(s.Show, name)
	}
	return s
}

// Load reads and resolves status.toml at path.
func Load(path string) (Set, error) {
	f, err := LoadFile(path)
	if err != nil {
		return Resolve(nil), err
	}
	return Resolve(f), nil
}

// Shown returns the Defs listed in Show, in Show order.
func (s Set) Shown() []Def {
	out := make([]Def, 0, len(s.Show))
	for _, name := range s.Show {
		for _, d := range s.Defs {
			if d.Name == name {
				out = append(out, d)
				break
			}
		}
	}
	return out
}
