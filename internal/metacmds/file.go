package metacmds

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// MetaFile is the decoded meta command definition file (meta.toml).
type MetaFile struct {
	Entries []MetaEntry
}

// MetaEntry is one [[entry]] block in meta.toml.
// File runs for regular files; Dirs runs for directories; $1 = absolute path.
//
// Stdout is shown in the panel Meta column. If the trimmed output contains no tab or newline,
// it is one cell (legacy): display width over the panel meta budget is replaced with "too long".
// Otherwise fields are split on tab and line feed (after \r\n/\r normalization to \n, then \n→\t),
// up to 8 fields; empty fields are preserved.
type MetaEntry struct {
	Name        string
	Description string
	File        string
	Dirs        string
	// Extensions holds glob patterns (e.g. "*.py", "*.go") matched against the entry basename.
	// When non-empty, the command is only run for entries whose basename matches at least one pattern.
	// Empty Extensions means no filter: the command runs for every entry.
	Extensions []string
	// Cache, when true, stores each computed result in a session-scoped in-memory cache keyed by
	// absolute path. Re-entering a directory skips re-running the command for paths already cached.
	// The cache persists for the lifetime of the application and is never written to disk.
	Cache bool
}

type metaFileRaw struct {
	Entry []metaEntryRaw `toml:"entry"`
}

type metaEntryRaw struct {
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	File        string   `toml:"file"`
	Dirs        string   `toml:"dirs"`
	Extensions  []string `toml:"extensions"`
	Cache       bool     `toml:"cache"`
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
	out := &MetaFile{}
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
		// Validate glob patterns.
		var exts []string
		for _, pat := range e.Extensions {
			pat = strings.TrimSpace(pat)
			if pat == "" {
				continue
			}
			if _, err := filepath.Match(pat, ""); err != nil {
				return nil, fmt.Errorf("meta.toml: entry %d: extensions: invalid glob %q: %w", i, pat, err)
			}
			exts = append(exts, pat)
		}
		out.Entries = append(out.Entries, MetaEntry{
			Name:        strings.TrimSpace(e.Name),
			Description: strings.TrimSpace(e.Description),
			File:        strings.TrimSpace(e.File),
			Dirs:        strings.TrimSpace(e.Dirs),
			Extensions:  exts,
			Cache:       e.Cache,
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

// MatchesPath reports whether path should be processed by this entry.
// When Extensions is empty every path matches. Otherwise the basename of path
// must match at least one glob pattern (using filepath.Match semantics).
func (e *MetaEntry) MatchesPath(path string) bool {
	if len(e.Extensions) == 0 {
		return true
	}
	base := filepath.Base(path)
	for _, pat := range e.Extensions {
		if matched, _ := filepath.Match(pat, base); matched {
			return true
		}
	}
	return false
}
