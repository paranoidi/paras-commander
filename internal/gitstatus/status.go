package gitstatus

// Status is one letter in the Git column (eza GitStatus).
type Status int

const (
	NotModified Status = iota
	New
	Modified
	Deleted
	Renamed
	TypeChange
	Ignored
	Conflicted
)

// Cell holds staged (index) and unstaged (work tree) status for one path.
type Cell struct {
	Staged   Status
	Unstaged Status
}

// Effective returns the more significant of Staged/Unstaged for single-status display.
func (c Cell) Effective() Status {
	return combineStatus(c.Staged, c.Unstaged)
}

// Rune returns the display glyph for s (eza notation).
func (s Status) Rune() rune {
	switch s {
	case New:
		return 'N'
	case Modified:
		return 'M'
	case Deleted:
		return 'D'
	case Renamed:
		return 'R'
	case TypeChange:
		return 'T'
	case Ignored:
		return 'I'
	case Conflicted:
		return 'U'
	default:
		return '-'
	}
}

// Label returns a short human-readable word for this status, shown in preview titles.
func (s Status) Label() string {
	switch s {
	case New:
		return "untracked"
	case Modified:
		return "modified"
	case Deleted:
		return "deleted"
	case Renamed:
		return "renamed"
	case TypeChange:
		return "type changed"
	case Ignored:
		return "ignored"
	case Conflicted:
		return "conflicted"
	default:
		return "no changes"
	}
}

// ThemeKey returns the panel.git.* theme key suffix for this status.
func (s Status) ThemeKey() string {
	switch s {
	case New:
		return "panel.git.new"
	case Modified:
		return "panel.git.modified"
	case Deleted:
		return "panel.git.deleted"
	case Renamed:
		return "panel.git.renamed"
	case TypeChange:
		return "panel.git.typechange"
	case Ignored:
		return "panel.git.ignored"
	case Conflicted:
		return "panel.git.conflicted"
	default:
		return "panel.git.not_modified"
	}
}

func combineStatus(a, b Status) Status {
	if statusPriority(a) >= statusPriority(b) {
		return a
	}
	return b
}

func statusPriority(s Status) int {
	switch s {
	case Conflicted:
		return 7
	case Deleted:
		return 6
	case Renamed:
		return 5
	case TypeChange:
		return 4
	case Modified:
		return 3
	case New:
		return 2
	case Ignored:
		return 1
	default:
		return 0
	}
}

func mapIndexRune(c rune) Status {
	switch c {
	case 'M':
		return Modified
	case 'A':
		return New
	case 'D':
		return Deleted
	case 'R':
		return Renamed
	case 'T':
		return TypeChange
	case 'U':
		return Conflicted
	case '!':
		return Ignored
	default:
		return NotModified
	}
}

func mapWorkTreeRune(c rune) Status {
	switch c {
	case 'M':
		return Modified
	case 'D':
		return Deleted
	case 'R':
		return Renamed
	case 'T':
		return TypeChange
	case 'U':
		return Conflicted
	case '!':
		return Ignored
	default:
		return NotModified
	}
}
