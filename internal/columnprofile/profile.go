// Package columnprofile holds named column profiles: which file and meta columns a panel shows.
// Built-in profiles live in code; saved ones in column-profiles.toml.
package columnprofile

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
	"github.com/paranoidi/paras-commander/internal/panel"
)

// Profile is one saved set of columns.
type Profile struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	// Columns are file column names (panel.Columns).
	Columns []string `toml:"columns"`
	// Meta are meta column names (meta.toml entries or fileindex columns).
	Meta []string `toml:"meta"`
	// Builtin marks a profile that lives in code and cannot be overwritten or deleted.
	Builtin bool `toml:"-"`
	// SkipCycle leaves the profile out of the profile cycle.
	SkipCycle bool `toml:"-"`
}

// Set returns the profile's file columns; unknown names are returned for a warning.
func (p Profile) Set() (panel.ColumnSet, []string) { return panel.ParseColumnNames(p.Columns) }

// Builtins returns the built-in profiles in cycle order. The index profile is what
// [fileindex].default_profile names by default; it is left out of the cycle because it only
// differs from brief where the index exists.
func Builtins() []Profile {
	return []Profile{
		{Name: config.ColumnProfileModified, Description: "size and modified time", Columns: panel.ColumnsModified.Names(), Builtin: true},
		{Name: config.ColumnProfilePermissions, Description: "size and permissions", Columns: panel.ColumnsPermissions.Names(), Builtin: true},
		{Name: config.ColumnProfileBrief, Description: "size only", Columns: panel.ColumnsBrief.Names(), Builtin: true},
		{Name: config.ColumnProfileIndex, Description: "size and fileindex tags", Columns: panel.ColumnsBrief.Names(), Meta: []string{config.FileIndexTagsColumn}, Builtin: true, SkipCycle: true},
	}
}

// All returns the built-in profiles followed by saved. A saved profile named like a built-in (a
// hand-edited file) is dropped: built-in names always mean the built-in.
func All(saved []Profile) []Profile {
	out := Builtins()
	for _, p := range saved {
		if !IsBuiltinName(p.Name) {
			out = append(out, p)
		}
	}
	return out
}

// Cycle returns the profiles of the profile cycle, in order.
func Cycle(saved []Profile) []Profile {
	all := All(saved)
	out := make([]Profile, 0, len(all))
	for _, p := range all {
		if !p.SkipCycle {
			out = append(out, p)
		}
	}
	return out
}

// Next returns the profile after current in the cycle, or the first when current is not in it.
func Next(saved []Profile, current string) Profile {
	cycle := Cycle(saved)
	for i, p := range cycle {
		if p.Name == current {
			return cycle[(i+1)%len(cycle)]
		}
	}
	return cycle[0]
}

// Find returns the profile named name among the built-in and saved profiles.
func Find(saved []Profile, name string) (Profile, bool) {
	for _, p := range All(saved) {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// IsBuiltinName reports whether name belongs to a built-in profile.
func IsBuiltinName(name string) bool {
	return slices.ContainsFunc(Builtins(), func(p Profile) bool { return p.Name == name })
}

type profilesFile struct {
	Profiles []Profile `toml:"profiles"`
}

// ResolveFile returns the path to column-profiles.toml: cfgFile when non-empty, otherwise the
// default name in configDir; "" when neither is known (nothing is read, saving fails).
func ResolveFile(cfgFile, configDir string) string {
	if f := strings.TrimSpace(cfgFile); f != "" {
		return f
	}
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, config.DefaultColumnProfilesFileName)
}

// Load reads the saved profiles from path; a missing file holds none.
func Load(path string) ([]Profile, error) {
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
	var f profilesFile
	if err := toml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	return f.Profiles, nil
}

// Save atomically writes the saved profiles to path.
func Save(path string, profiles []Profile) error {
	if path == "" {
		return errors.New("config location is unknown; cannot save")
	}
	return ops.WriteTOMLAtomic(path, profilesFile{Profiles: profiles})
}

// EncodeStub writes an empty profiles file (used by pc --config-stub).
func EncodeStub(w io.Writer) error {
	_, err := io.WriteString(w, "# Saved column profiles. Save them from the Columns dialog.\nprofiles = []\n")
	return err
}

// Upsert replaces the saved profile with p's name, or appends p. A built-in name is refused.
func Upsert(path string, p Profile) error {
	if IsBuiltinName(p.Name) {
		return fmt.Errorf("%q is a built-in profile", p.Name)
	}
	profiles, err := Load(path)
	if err != nil {
		return err
	}
	for i := range profiles {
		if profiles[i].Name == p.Name {
			profiles[i] = p
			return Save(path, profiles)
		}
	}
	return Save(path, append(profiles, p))
}

// Remove deletes the saved profile named name, if present. A built-in name is refused.
func Remove(path, name string) error {
	if IsBuiltinName(name) {
		return fmt.Errorf("%q is a built-in profile", name)
	}
	profiles, err := Load(path)
	if err != nil {
		return err
	}
	return Save(path, slices.DeleteFunc(profiles, func(p Profile) bool { return p.Name == name }))
}
