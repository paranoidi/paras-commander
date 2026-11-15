// Package filterpattern holds the saved patterns of the Filter and Select group dialogs: a pattern
// with its mode, file types and options, kept in filters.toml.
package filterpattern

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ops"
)

// Pattern modes as stored in filters.toml.
const (
	ModeSimple = "simple"
	ModeShell  = "shell"
	ModeRegex  = "regex"
)

// Pattern scopes as stored in filters.toml: which entries the pattern applies to.
const (
	ScopeFiles = "files"
	ScopeDirs  = "dirs"
	ScopeBoth  = "both"
)

// Pattern is one saved Filter dialog configuration. History entries have no Name.
type Pattern struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Pattern     string `toml:"pattern"`
	// Mode is ModeSimple, ModeShell or ModeRegex.
	Mode string `toml:"mode"`
	// Types are the checked file types, lowercase ("videos", "pictures", ...).
	Types []string `toml:"types"`
	// Scope is ScopeFiles, ScopeDirs or ScopeBoth; empty or unknown loads as ScopeFiles.
	Scope         string `toml:"scope"`
	CaseSensitive bool   `toml:"case_sensitive"`
	// Invert is a Filter dialog option; Select group ignores it.
	Invert bool `toml:"invert"`
	// FullPath is a Select group option (where it shows Match whole path); Filter ignores it.
	FullPath    bool `toml:"full_path"`
	IncludeMeta bool `toml:"include_meta"`
	OnlyMeta    bool `toml:"only_meta"`
}

// Equal reports whether p and o hold the same name, description, pattern and options.
func (p Pattern) Equal(o Pattern) bool {
	return p.scalars() == o.scalars() && slices.Equal(p.Types, o.Types)
}

// patternScalars is Pattern without its slice, so it compares with ==.
type patternScalars struct {
	name, description, pattern, mode, scope                string
	caseSensitive, invert, fullPath, includeMeta, onlyMeta bool
}

func (p Pattern) scalars() patternScalars {
	return patternScalars{p.Name, p.Description, p.Pattern, p.Mode, p.Scope,
		p.CaseSensitive, p.Invert, p.FullPath, p.IncludeMeta, p.OnlyMeta}
}

type patternsFile struct {
	Filters []Pattern `toml:"filters"`
}

// ResolveFile returns the path to filters.toml: cfgFile when non-empty, otherwise the default
// name in configDir; "" when neither is known (nothing is read, saving fails).
func ResolveFile(cfgFile, configDir string) string {
	if f := strings.TrimSpace(cfgFile); f != "" {
		return f
	}
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, config.DefaultFilterPatternsFileName)
}

// Load reads the saved patterns from path; a missing file holds none.
func Load(path string) ([]Pattern, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var f patternsFile
	if err := toml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	return f.Filters, nil
}

// Save atomically writes the saved patterns to path.
func Save(path string, patterns []Pattern) error {
	if path == "" {
		return errors.New("config location is unknown; cannot save")
	}
	return ops.WriteTOMLAtomic(path, patternsFile{Filters: patterns})
}

// EncodeStub writes an empty patterns file (used by pc --config-stub).
func EncodeStub(w io.Writer) error {
	_, err := io.WriteString(w, "# Saved Filter dialog patterns. Save them from the Filter dialog.\nfilters = []\n")
	return err
}

// Upsert replaces the saved pattern with p's name, or appends p.
func Upsert(path string, p Pattern) error {
	patterns, err := Load(path)
	if err != nil {
		return err
	}
	for i := range patterns {
		if patterns[i].Name == p.Name {
			patterns[i] = p
			return Save(path, patterns)
		}
	}
	return Save(path, append(patterns, p))
}

// Remove deletes the saved pattern named name, if present.
func Remove(path, name string) error {
	patterns, err := Load(path)
	if err != nil {
		return err
	}
	return Save(path, slices.DeleteFunc(patterns, func(p Pattern) bool { return p.Name == name }))
}
