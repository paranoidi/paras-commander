package panel

// NewSideListing returns a fresh read-only listing State (quick-view directory overlay, dedup
// browse panel) that follows driver's sort/filter/view settings and follower's per-directory
// probes and history. Callers set ScheduleAsyncLoad, ScheduleGitStatus and FileListViewportRows.
func NewSideListing(driver, follower *State) State {
	return State{
		Sort:                       driver.Sort,
		Filter:                     driver.Filter,
		ShowHidden:                 driver.ShowHidden,
		Columns:                    driver.Columns,
		ScrollMode:                 driver.ScrollMode,
		ScrollEdgeMargin:           driver.ScrollEdgeMargin,
		Gitignore:                  follower.Gitignore,
		DiskSorter:                 follower.DiskSorter,
		DiskExcluded:               follower.DiskExcluded,
		SuppressHeavyPathProbes:    follower.SuppressHeavyPathProbes,
		IdleDiskTotalsSort:         driver.IdleDiskTotalsSort,
		DiskUsageIdleSortEligible:  follower.DiskUsageIdleSortEligible,
		DiskUsageIdleSortActivated: follower.DiskUsageIdleSortActivated,
		HistoryCursorByPath:        MergeHistoryCursorByPath(follower.HistoryCursorByPath, driver.HistoryCursorByPath),
	}
}
