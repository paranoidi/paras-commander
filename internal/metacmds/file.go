package metacmds

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/paranoidi/paras-commander/internal/entrymatch"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// MetaFile is the decoded meta command definition file (meta.toml).
type MetaFile struct {
	ShellPatterns bool
	Entries       []MetaEntry
}

// MetaEntry is one [[entry]] block in meta.toml.
// File runs for regular files; Dirs runs for directories; %f = absolute row path in shell scripts.
//
// Stdout is shown in the panel Meta column. If the trimmed output contains no tab or newline,
// it is one cell (legacy). Cells that exceed the panel meta budget are clipped with a trailing ….
// Otherwise fields are split on tab and line feed (after \r\n/\r normalization to \n, then \n→\t),
// up to 8 fields; empty fields are preserved.
type MetaEntry struct {
	Name          string
	Description   string
	Column        string // panel header; empty means use Name
	Order         int    // left-to-right sort key (lower = closer to name)
	File          string
	Dirs          string
	When          []string
	Cache         bool
	ShellPatterns bool
	// Workers is the number of concurrent background goroutines for this entry.
	// 0 means use the global default from [meta] default_entry_workers in the main config.
	Workers int
}

type metaFileRaw struct {
	ShellPatterns *boolField     `toml:"shell_patterns"`
	Entry         []metaEntryRaw `toml:"entry"`
}

type metaEntryRaw struct {
	Name          string     `toml:"name"`
	Description   string     `toml:"description"`
	Column        string     `toml:"column"`
	Order         int        `toml:"order"`
	File          string     `toml:"file"`
	Dirs          string     `toml:"dirs"`
	When          *whenField `toml:"when"`
	Cache         bool       `toml:"cache"`
	ShellPatterns *boolField `toml:"shell_patterns"`
	Workers       int        `toml:"workers"`
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

// LoadFile reads and validates meta.toml from path.
func LoadFile(path string) (*MetaFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Decode(b)
}

// Decode parses meta TOML from bytes.
func Decode(data []byte) (*MetaFile, error) {
	var raw metaFileRaw
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("meta.toml: %w", err)
	}
	out := &MetaFile{ShellPatterns: true}
	if raw.ShellPatterns != nil && raw.ShellPatterns.Set {
		out.ShellPatterns = raw.ShellPatterns.Value
	}
	for i, e := range raw.Entry {
		if strings.TrimSpace(e.Name) == "" {
			return nil, fmt.Errorf("meta.toml: entry %d: name is required", i)
		}
		if strings.TrimSpace(e.Description) == "" {
			return nil, fmt.Errorf("meta.toml: entry %d: description is required", i)
		}
		if strings.TrimSpace(e.File) == "" && strings.TrimSpace(e.Dirs) == "" {
			return nil, fmt.Errorf("meta.toml: entry %d: at least one of file or dirs command is required", i)
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
		if e.Order < 0 {
			return nil, fmt.Errorf("meta.toml: entry %d: order must be >= 0", i)
		}
		// Validate workers.
		if e.Workers < 0 {
			return nil, fmt.Errorf("meta.toml: entry %d: workers must be >= 0", i)
		}
		const metaEntryWorkersMax = 64
		workers := e.Workers
		if workers > metaEntryWorkersMax {
			workers = metaEntryWorkersMax
		}
		name := strings.TrimSpace(e.Name)
		column := strings.TrimSpace(e.Column)
		if column == "" {
			column = name
		}
		out.Entries = append(out.Entries, MetaEntry{
			Name:          name,
			Description:   strings.TrimSpace(e.Description),
			Column:        column,
			Order:         e.Order,
			File:          strings.TrimSpace(e.File),
			Dirs:          strings.TrimSpace(e.Dirs),
			When:          whenList,
			Cache:         e.Cache,
			ShellPatterns: resolveShellPatterns(out.ShellPatterns, e.ShellPatterns),
			Workers:       workers,
		})
	}
	return out, nil
}

// EntryByName returns the MetaEntry with the given name, or (MetaEntry{}, false) if not found.
func (f *MetaFile) EntryByName(name string) (MetaEntry, bool) {
	for _, e := range f.Entries {
		if e.Name == name {
			return e, true
		}
	}
	return MetaEntry{}, false
}

func resolveShellPatterns(fileDefault bool, entry *boolField) bool {
	if entry != nil && entry.Set {
		return entry.Value
	}
	return fileDefault
}

// MatchesRow reports whether ent in panelDir passes this entry's when filter.
func (e *MetaEntry) MatchesRow(ent localfs.Entry, panelDir string) (bool, error) {
	if len(e.When) == 0 {
		return true, nil
	}
	row := ent
	ctx := &entrymatch.Context{
		ShellPatterns: e.ShellPatterns,
		Row:           &row,
		PanelDir:      panelDir,
	}
	return entrymatch.EvalWhenAny(e.When, ctx)
}
