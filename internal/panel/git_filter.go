package panel

import (
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// GitStagedFilter shows only entries with a staged (index) change.
func GitStagedFilter() *EntryFilter {
	return &EntryFilter{ID: "git-staged", Label: "Filter: staged", Match: func(e localfs.Entry, s *State) bool {
		return s.GitByPath[e.Path].Staged != gitstatus.NotModified
	}, Applicable: func(s *State) bool {
		return s.GitColumnActive
	}}
}

// GitUnstagedFilter shows only entries with an unstaged (work tree) modification, excluding
// untracked and ignored entries.
func GitUnstagedFilter() *EntryFilter {
	return &EntryFilter{ID: "git-unstaged", Label: "Filter: unstaged", Match: func(e localfs.Entry, s *State) bool {
		c := s.GitByPath[e.Path]
		return c.Unstaged != gitstatus.NotModified && c.Unstaged != gitstatus.New && c.Unstaged != gitstatus.Ignored
	}, Applicable: func(s *State) bool {
		return s.GitColumnActive
	}}
}

// GitUntrackedFilter shows only entries not tracked by git.
func GitUntrackedFilter() *EntryFilter {
	return &EntryFilter{ID: "git-untracked", Label: "Filter: untracked", Match: func(e localfs.Entry, s *State) bool {
		c := s.GitByPath[e.Path]
		return c.Staged == gitstatus.NotModified && c.Unstaged == gitstatus.New
	}, Applicable: func(s *State) bool {
		return s.GitColumnActive
	}}
}

// GitTrackedFilter shows only entries tracked by git (excludes untracked and ignored entries).
func GitTrackedFilter() *EntryFilter {
	return &EntryFilter{ID: "git-tracked", Label: "Filter: tracked", Match: func(e localfs.Entry, s *State) bool {
		c := s.GitByPath[e.Path]
		untracked := c.Staged == gitstatus.NotModified && c.Unstaged == gitstatus.New
		return !untracked && c.Unstaged != gitstatus.Ignored
	}, Applicable: func(s *State) bool {
		return s.GitColumnActive
	}}
}
