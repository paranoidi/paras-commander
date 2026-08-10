package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// MassRenamePattern is one saved mass-rename find/replace (or capitalize) configuration.
type MassRenamePattern struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Mode        string `toml:"mode"` // "simple" | "regex" | "capitalize"
	Find        string `toml:"find"`
	Replace     string `toml:"replace"`
	CaseFold    bool   `toml:"case_fold"`
	StripSpaces bool   `toml:"strip_spaces"`
	CapEachWord bool   `toml:"cap_each_word"`
	CapPunctSep bool   `toml:"cap_punct_sep"`
}

type massRenamePatternsFile struct {
	Patterns []MassRenamePattern `toml:"patterns"`
}

// MassRenamePatternsResolveFile returns the path to patterns.toml: cfgFile when non-empty
// (an absolute or already-resolved path), otherwise filepath.Join(configDir, "patterns.toml").
func MassRenamePatternsResolveFile(cfgFile, configDir string) string {
	if f := strings.TrimSpace(cfgFile); f != "" {
		return f
	}
	return filepath.Join(configDir, "patterns.toml")
}

// LoadMassRenamePatterns reads path. A missing file returns (nil, nil).
func LoadMassRenamePatterns(path string) ([]MassRenamePattern, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var f massRenamePatternsFile
	if err := toml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	return f.Patterns, nil
}

// SaveMassRenamePatterns atomically writes patterns to path (temp file in the same directory,
// synced and chmod 0o644, then renamed into place).
func SaveMassRenamePatterns(path string, patterns []MassRenamePattern) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %q: %w", dir, err)
	}
	tmpPath := tmp.Name()
	encErr := toml.NewEncoder(tmp).Encode(massRenamePatternsFile{Patterns: patterns})
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if encErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		if encErr != nil {
			return fmt.Errorf("encode %q: %w", path, encErr)
		}
		return fmt.Errorf("write %q: %w", path, syncErr)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %q to %q: %w", tmpPath, path, err)
	}
	return nil
}

// UpsertMassRenamePattern replaces the entry with a matching Name (case-sensitive), or appends
// p when no entry matches.
func UpsertMassRenamePattern(path string, p MassRenamePattern) error {
	patterns, err := LoadMassRenamePatterns(path)
	if err != nil {
		return err
	}
	for i := range patterns {
		if patterns[i].Name == p.Name {
			patterns[i] = p
			return SaveMassRenamePatterns(path, patterns)
		}
	}
	patterns = append(patterns, p)
	return SaveMassRenamePatterns(path, patterns)
}

// RemoveMassRenamePattern removes the entry named name, if present.
func RemoveMassRenamePattern(path string, name string) error {
	patterns, err := LoadMassRenamePatterns(path)
	if err != nil {
		return err
	}
	out := make([]MassRenamePattern, 0, len(patterns))
	for _, p := range patterns {
		if p.Name == name {
			continue
		}
		out = append(out, p)
	}
	return SaveMassRenamePatterns(path, out)
}
