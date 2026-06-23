package ui

// panelEndEdgeOnTopRow reports whether sync/quick-view/other-panel indicators paint on the top frame row.
func panelEndEdgeOnTopRow(panelID int, orientation SplitOrientation) bool {
	return orientation == SplitVertical && panelID == SecondaryPanel
}

// panelEndEdgeOnBottomRow reports whether end-edge indicators paint on the bottom frame row.
func panelEndEdgeOnBottomRow(panelID int, orientation SplitOrientation) bool {
	return !panelEndEdgeOnTopRow(panelID, orientation)
}

func panelSyncIndicatorLabel(panelID int, orientation SplitOrientation) string {
	if panelID == SecondaryPanel {
		if orientation == SplitVertical {
			return " ↑ Sync "
		}
		return " ← Sync "
	}
	if orientation == SplitVertical {
		return " Sync ↓ "
	}
	return " Sync → "
}

func panelQuickViewIndicatorLabel(panelID int, orientation SplitOrientation) string {
	if panelID == SecondaryPanel {
		if orientation == SplitVertical {
			return " ↑ Quick view "
		}
		return " ← Quick view "
	}
	if orientation == SplitVertical {
		return " Quick view ↓ "
	}
	return " Quick view → "
}

func panelOtherPanelIndicatorLabel(panelID int, ctx PanelBottomIndicatorContext) string {
	max := ctx.EndEdgePathMaxRunes
	if max <= 0 {
		max = 12
	}
	path := PanelTitlePath(ctx.OtherPanelPath, ctx.UserHomeDir, max)
	if panelID == SecondaryPanel {
		if ctx.SplitOrientation == SplitVertical {
			return " ↑ " + path + " "
		}
		return " ← " + path + " "
	}
	if ctx.SplitOrientation == SplitVertical {
		return " ↓ " + path + " "
	}
	return " → " + path + " "
}

// SyncFollowToastParts returns arrow and pane labels for the sync-enable status message.
func SyncFollowToastParts(driverPanel int, orientation SplitOrientation) (arrow, driver, follower string) {
	driver = "Primary"
	follower = "Secondary"
	if driverPanel == SecondaryPanel {
		driver, follower = follower, driver
	}
	if orientation == SplitVertical {
		if driverPanel == SecondaryPanel {
			arrow = "↑"
		} else {
			arrow = "↓"
		}
		return arrow, driver, follower
	}
	if driverPanel == SecondaryPanel {
		arrow = "←"
	} else {
		arrow = "→"
	}
	return arrow, driver, follower
}
