// Package findcrit holds the Find dialog's advanced criteria (age, size, kind, owner, file-type
// groups) and its query match modes (fuzzy, glob, regex). It is pure: the scan coordinator
// evaluates Criteria and Matcher off the UI thread, the find handler parses the dialog's text
// fields into them, and the dialog view reads the same parsers for its live validation.
package findcrit

import (
	"io/fs"
	"regexp"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// StatLevel is how much per-entry metadata a find walk collects. Levels are ordered: an index
// built at a level serves every criterion that needs that level or less.
type StatLevel int

const (
	// StatNone collects nothing beyond the directory listing (name, type).
	StatNone StatLevel = iota
	// StatMeta lstats every entry (times, size, mode, owner).
	StatMeta
	// StatMetaEmptyDirs also opens every directory to learn whether it has entries.
	StatMetaEmptyDirs
)

// Meta is one entry's lstat metadata, collected by the walk when StatLevel >= StatMeta.
type Meta struct {
	ModTime, ChangeTime, AccessTime int64 // unix nanoseconds
	Mode                            fs.FileMode
	UID                             uint32
	// EmptyDir is set on a directory without entries (StatMetaEmptyDirs only).
	EmptyDir bool
}

// TimeBound limits one timestamp: Newer keeps t >= Newer, Older keeps t < Older (unix
// nanoseconds, 0 = no bound).
type TimeBound struct {
	Newer, Older int64
}

func (b TimeBound) active() bool { return b.Newer != 0 || b.Older != 0 }

func (b TimeBound) match(t int64) bool {
	return (b.Newer == 0 || t >= b.Newer) && (b.Older == 0 || t < b.Older)
}

// Criteria are the advanced find filters; every set field must hold (AND). The kind flags
// (Symlinks, BrokenSymlinks, Executables) are alternatives: an entry passes when it is any of
// the checked kinds. The zero value filters nothing.
type Criteria struct {
	Modified, Changed, Accessed TimeBound
	// MinSize and MaxSize (with HasMaxSize) bound a file's byte size; directories never match a
	// size bound.
	MinSize    int64
	MaxSize    int64
	HasMaxSize bool
	// Empty keeps zero-byte regular files and directories without entries.
	Empty                                 bool
	Symlinks, BrokenSymlinks, Executables bool
	// Types (the checked [filter.types] groups, OR-joined) must match a file's basename;
	// directories never match. nil = any.
	Types *regexp.Regexp
	// Owner (with HasOwner) is the uid an entry must belong to.
	Owner    uint32
	HasOwner bool
	// NotMine keeps entries whose owner is not MyUID.
	NotMine bool
	MyUID   uint32
}

// Active reports whether any criterion is set.
func (c Criteria) Active() bool {
	return c.Modified.active() || c.Changed.active() || c.Accessed.active() ||
		c.MinSize > 0 || c.HasMaxSize || c.Empty ||
		c.Symlinks || c.BrokenSymlinks || c.Executables ||
		c.Types != nil || c.HasOwner || c.NotMine
}

func (c Criteria) sizeBound() bool { return c.MinSize > 0 || c.HasMaxSize }

// NeedsStat is the walk metadata level the criteria read.
func (c Criteria) NeedsStat() StatLevel {
	switch {
	case c.Empty:
		return StatMetaEmptyDirs
	case c.Modified.active() || c.Changed.active() || c.Accessed.active() ||
		c.sizeBound() || c.Executables || c.HasOwner || c.NotMine:
		return StatMeta
	}
	return StatNone
}

// Subject is what Match reads of one indexed entry.
type Subject struct {
	Name    string // basename
	IsDir   bool
	Symlink bool
	Broken  bool  // a symlink whose target does not resolve
	Size    int64 // file byte size (lstat); meaningful when Meta is set
	Meta    *Meta // nil when the walk did not stat
}

// Match reports whether s passes every criterion. An entry without metadata never passes a
// criterion that needs it.
func (c Criteria) Match(s Subject) bool {
	if c.Types != nil && (s.IsDir || !c.Types.MatchString(s.Name)) {
		return false
	}
	if c.Symlinks || c.BrokenSymlinks || c.Executables {
		exec := s.Meta != nil && localfs.ModeIsExecutable(s.Meta.Mode)
		kind := (c.Symlinks && s.Symlink) || (c.BrokenSymlinks && s.Broken) || (c.Executables && exec)
		if !kind {
			return false
		}
	}
	if c.NeedsStat() == StatNone {
		return true
	}
	m := s.Meta
	if m == nil {
		return false
	}
	if !c.Modified.match(m.ModTime) || !c.Changed.match(m.ChangeTime) || !c.Accessed.match(m.AccessTime) {
		return false
	}
	if c.sizeBound() && (s.IsDir || s.Size < c.MinSize || (c.HasMaxSize && s.Size > c.MaxSize)) {
		return false
	}
	if c.Empty {
		if s.IsDir {
			if !m.EmptyDir {
				return false
			}
		} else if !m.Mode.IsRegular() || s.Size != 0 {
			return false
		}
	}
	if c.HasOwner && m.UID != c.Owner {
		return false
	}
	if c.NotMine && m.UID == c.MyUID {
		return false
	}
	return true
}
