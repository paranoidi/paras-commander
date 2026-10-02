package panel

import (
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// GitStagedFilter shows only entries with a staged (index) change.
func GitStagedFilter() *EntryFilter {
	return &EntryFilter{FiltersDirs: true, ID: "git-staged", Label: "Filter: staged", Match: func(e localfs.Entry, s *State) bool {
		c := s.GitByPath[e.Path]
		if e.IsDir() {
			return c.HasStaged
		}
		return c.Staged != gitstatus.NotModified
	}, Applicable: func(s *State) bool {
		return s.GitColumnActive
	}}
}

// GitUnstagedFilter shows only entries with an unstaged (work tree) modification, excluding
// untracked and ignored entries.
func GitUnstagedFilter() *EntryFilter {
	return &EntryFilter{FiltersDirs: true, ID: "git-unstaged", Label: "Filter: unstaged", Match: func(e localfs.Entry, s *State) bool {
		c := s.GitByPath[e.Path]
		if e.IsDir() {
			return c.HasUnstaged
		}
		return c.Unstaged != gitstatus.NotModified && c.Unstaged != gitstatus.New && c.Unstaged != gitstatus.Ignored
	}, Applicable: func(s *State) bool {
		return s.GitColumnActive
	}}
}

// GitUntrackedFilter shows only entries not tracked by git.
func GitUntrackedFilter() *EntryFilter {
	return &EntryFilter{FiltersDirs: true, ID: "git-untracked", Label: "Filter: untracked", Match: func(e localfs.Entry, s *State) bool {
		c := s.GitByPath[e.Path]
		if e.IsDir() {
			return c.HasUntracked
		}
		return c.Staged == gitstatus.NotModified && c.Unstaged == gitstatus.New
	}, Applicable: func(s *State) bool {
		return s.GitColumnActive
	}}
}

// GitTrackedFilter shows only entries tracked by git (excludes untracked and ignored entries).
func GitTrackedFilter() *EntryFilter {
	return &EntryFilter{FiltersDirs: true, ID: "git-tracked", Label: "Filter: tracked", Match: func(e localfs.Entry, s *State) bool {
		c := s.GitByPath[e.Path]
		if e.IsDir() {
			return !c.NoTracked
		}
		untracked := c.Staged == gitstatus.NotModified && c.Unstaged == gitstatus.New
		return !untracked && c.Unstaged != gitstatus.Ignored
	}, Applicable: func(s *State) bool {
		return s.GitColumnActive
	}}
}
