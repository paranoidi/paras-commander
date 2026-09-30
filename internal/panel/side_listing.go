package panel

// NewSideListing returns a fresh read-only listing State (quick-view directory overlay, dedup
// browse panel) that follows driver's sort/filter/view settings and follower's per-directory
// probes and history. Callers set ScheduleAsyncLoad, ScheduleGitStatus and FileListViewportRows.
func NewSideListing(driver, follower *State) State {
	return State{
		Sort:                       driver.Sort,
		Filter:                     driver.Filter,
		ShowHidden:                 driver.ShowHidden,
		ListFormat:                 driver.ListFormat,
		ScrollMode:                 driver.ScrollMode,
		ScrollEdgeMargin:           driver.ScrollEdgeMargin,
		Gitignore:                  follower.Gitignore,
		DiskSorter:                 follower.DiskSorter,
		SuppressHeavyPathProbes:    follower.SuppressHeavyPathProbes,
		IdleDiskTotalsSort:         follower.IdleDiskTotalsSort,
		DiskUsageIdleSortEligible:  follower.DiskUsageIdleSortEligible,
		DiskUsageIdleSortActivated: follower.DiskUsageIdleSortActivated,
		HistoryCursorByPath:        MergeHistoryCursorByPath(follower.HistoryCursorByPath, driver.HistoryCursorByPath),
	}
}
