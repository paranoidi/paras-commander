package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the quick job-blocker dialog: the three screens (File exists, Disk space,
// Conflict rules), every focus position, the overwrite and directory-mismatch button layouts,
// short and overlong content, narrow widths and clamped heights. Focus is named; conflictGolden*
// are the only code that knows how the state encodes it.

const conflictGoldenHome = "/home/kestrel"

// conflictFocusNames are the buttons of the File exists screen by name.
func conflictFileState(c jobs.ConflictEvent, focus string) ConflictDialogState {
	st := ConflictDialogState{Open: true, JobID: "job-harbor", Blocker: jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &c}}
	st.Focus = conflictGoldenFileFocus(focus)
	return st
}

func conflictDiskState(d jobs.DiskSpaceBlockerDetails, focus string) ConflictDialogState {
	st := ConflictDialogState{Open: true, JobID: "job-harbor", Blocker: jobs.BlockerDetails{Kind: jobs.BlockerKindDiskSpace, DiskSpace: &d}}
	st.Focus = conflictGoldenDiskFocus(focus)
	return st
}

func conflictAdvState(c jobs.ConflictEvent, focus string, rules jobs.ConflictRules, all bool) ConflictDialogState {
	st := ConflictDialogState{Open: true, JobID: "job-harbor", Blocker: jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &c}}
	st.StartAdvanced()
	st.Rules, st.AdvAll = rules, all
	conflictGoldenAdvFocus(&st, focus)
	return st
}

func conflictGoldenFileFocus(name string) dlg.ID {
	switch name {
	case "overwrite":
		return ConflictOverwrite
	case "rename":
		return ConflictRename
	case "overwrite-all":
		return ConflictOverwriteAll
	case "rename-all":
		return ConflictRenameAll
	case "advanced":
		return ConflictAdvanced
	case "skip":
		return ConflictSkip
	case "skip-all":
		return ConflictSkipAll
	case "cancel":
		return ConflictCancelJob
	case "postpone":
		return ConflictPostpone
	}
	panic("unknown file focus " + name)
}

func conflictGoldenDiskFocus(name string) dlg.ID {
	switch name {
	case "retry":
		return ConflictRetry
	case "abort":
		return ConflictAbort
	case "postpone":
		return ConflictPostpone
	}
	panic("unknown disk focus " + name)
}

// conflictGoldenAdvFocus sets the advanced-mode focus: "r<row>c<col>" is a rule radio, or
// skip-identical / all / ok / cancel.
func conflictGoldenAdvFocus(st *ConflictDialogState, name string) {
	var row, col int
	if n, _ := fmt.Sscanf(name, "r%dc%d", &row, &col); n == 2 {
		st.AdvFocus = ConflictRuleID(row, col)
		return
	}
	switch name {
	case "skip-identical":
		st.AdvFocus = ConflictSkipIdentical
	case "all":
		st.AdvFocus = ConflictApplyAll
	case "ok":
		st.AdvFocus = dlg.OK
	case "cancel":
		st.AdvFocus = dlg.Cancel
	default:
		panic("unknown advanced focus " + name)
	}
}

func conflictGoldenEvent() jobs.ConflictEvent {
	return jobs.ConflictEvent{
		Source: "/home/kestrel/meadow/orchard.dat", Destination: "/srv/willow/amber/orchard.dat",
		SourceSize: "1.5 MiB", SourceTime: "2024-03-01 10:15", DestSize: "900 KiB", DestTime: "2023-11-20 08:00",
	}
}

func conflictGoldenLong(c jobs.ConflictEvent) jobs.ConflictEvent {
	c.Source = "/home/kestrel/" + strings.Repeat("glacier-harbor/", 7) + "orchard.dat"
	c.Destination = "/srv/" + strings.Repeat("quartz-pebble/", 7) + "orchard.dat"
	return c
}

func TestConflictDialogGoldenRender(t *testing.T) {
	goldenBandRows["conflict_dialog"] = 24
	var cases []simpleGolden
	add := func(name string, w, h int, st ConflictDialogState) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawConflictDialog(s, l, st, asciiDialogStyles(), conflictGoldenHome)
		}})
	}
	base := conflictGoldenEvent()

	// File exists, overwrite layout: every focus position.
	for _, f := range []string{"overwrite", "overwrite-all", "advanced", "skip", "skip-all", "cancel", "postpone"} {
		add("file_focus-"+f, 80, 30, conflictFileState(base, f))
	}
	// Directory mismatch (rename layout), both directions.
	dirDest := base
	dirDest.Mismatch, dirDest.DestIsDir = true, true
	fileDest := base
	fileDest.Mismatch = true
	for _, f := range []string{"rename", "rename-all", "skip", "skip-all", "cancel", "postpone"} {
		add("mismatch-dir_focus-"+f, 80, 30, conflictFileState(dirDest, f))
	}
	add("mismatch-file_focus-rename", 80, 30, conflictFileState(fileDest, "rename"))
	// Content variants.
	reprompt := base
	reprompt.Reprompt = "No rule matched."
	add("file_reprompt", 80, 30, conflictFileState(reprompt, "overwrite"))
	bare := jobs.ConflictEvent{Source: "/home/kestrel/a.txt", Destination: "/srv/b.txt"}
	add("file_no_meta", 80, 30, conflictFileState(bare, "skip"))
	add("file_long_paths", 80, 30, conflictFileState(conflictGoldenLong(base), "overwrite"))
	add("file_nil_conflict", 80, 30, ConflictDialogState{Open: true, Blocker: jobs.BlockerDetails{Kind: jobs.BlockerKindConflict}})
	// Sizes: wide, narrow (the 48 minimum), narrower than the minimum, clamped heights.
	add("file_wide120", 120, 30, conflictFileState(base, "overwrite-all"))
	add("file_narrow60", 60, 30, conflictFileState(conflictGoldenLong(base), "advanced"))
	add("file_narrow50", 50, 30, conflictFileState(base, "postpone"))
	add("file_narrow40", 40, 30, conflictFileState(base, "overwrite"))
	add("file_short12", 80, 12, conflictFileState(base, "overwrite"))
	add("file_short8_cancel", 80, 8, conflictFileState(base, "cancel"))

	// Disk space.
	disk := jobs.DiskSpaceBlockerDetails{Destination: "/srv/willow/amber", RequiredBytes: 5 << 20, AvailableBytes: 900 << 10, AvailableKnown: true, NextSource: "/home/kestrel/meadow/orchard.dat"}
	for _, f := range []string{"retry", "abort", "postpone"} {
		add("disk_focus-"+f, 80, 30, conflictDiskState(disk, f))
	}
	noNext := disk
	noNext.NextSource = ""
	add("disk_no_next", 80, 30, conflictDiskState(noNext, "retry"))
	unknown := disk
	unknown.AvailableKnown = false
	add("disk_unknown_available", 80, 30, conflictDiskState(unknown, "abort"))
	tiny := disk
	tiny.RequiredBytes, tiny.AvailableBytes = 12, 1500
	add("disk_small_sizes", 80, 30, conflictDiskState(tiny, "retry"))
	longDisk := disk
	longDisk.Destination = "/srv/" + strings.Repeat("quartz-pebble/", 8) + "amber"
	longDisk.NextSource = "/home/kestrel/" + strings.Repeat("glacier-harbor/", 8) + "orchard.dat"
	add("disk_long_paths", 80, 30, conflictDiskState(longDisk, "retry"))
	add("disk_nil_details", 80, 30, ConflictDialogState{Open: true, Blocker: jobs.BlockerDetails{Kind: jobs.BlockerKindDiskSpace}})
	add("disk_narrow50", 50, 30, conflictDiskState(disk, "postpone"))
	add("disk_narrow40", 40, 30, conflictDiskState(disk, "retry"))
	add("disk_short8", 80, 8, conflictDiskState(disk, "abort"))

	// Conflict rules.
	var none jobs.ConflictRules
	for row := 0; row < ConflictRuleRows; row++ {
		for col := 0; col < len(ConflictRuleChoices); col++ {
			add(fmt.Sprintf("adv_focus-r%dc%d", row, col), 80, 30, conflictAdvState(base, fmt.Sprintf("r%dc%d", row, col), none, true))
		}
	}
	for _, f := range []string{"skip-identical", "all", "ok", "cancel"} {
		add("adv_focus-"+f, 80, 30, conflictAdvState(base, f, none, true))
	}
	picked := jobs.ConflictRules{Time: [3]jobs.RuleAction{jobs.RuleOverwrite, jobs.RuleAsk, jobs.RuleSkip}, Size: [3]jobs.RuleAction{jobs.RuleAsk, jobs.RuleOverwrite, jobs.RuleAsk}, SkipIdentical: true}
	picked.Normalize()
	add("adv_rules_conflicting", 80, 30, conflictAdvState(base, "r3c0", picked, false))
	oneKind := jobs.ConflictRules{Time: [3]jobs.RuleAction{jobs.RuleSkip, jobs.RuleAsk, jobs.RuleAsk}, SkipIdentical: true}
	add("adv_rules_one_action", 80, 30, conflictAdvState(base, "r4c2", oneKind, true))
	add("adv_rules_one_action_ok", 80, 30, conflictAdvState(base, "ok", oneKind, false))
	noCompare := base
	noCompare.NoCompare = true
	for _, f := range []string{"r0c0", "all", "ok", "cancel"} {
		add("adv_nocompare_focus-"+f, 80, 30, conflictAdvState(noCompare, f, oneKind, true))
	}
	add("adv_wide120", 120, 30, conflictAdvState(base, "r1c1", oneKind, true))
	add("adv_narrow56", 56, 30, conflictAdvState(base, "r1c1", oneKind, true))
	add("adv_narrow50", 50, 30, conflictAdvState(base, "skip-identical", oneKind, true))
	add("adv_narrow40", 40, 30, conflictAdvState(base, "r0c0", none, true))
	add("adv_short14", 80, 14, conflictAdvState(base, "ok", oneKind, true))
	add("adv_short10_cancel", 80, 10, conflictAdvState(base, "cancel", none, true))
	runSimpleGoldens(t, "conflict_dialog", cases)
}
