package dialog

import (
	"fmt"
	"strconv"
	"strings"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// The golden tests name focus positions ("yes", "item3", "ok"...). These helpers are the only
// code that knows how each dialog state encodes focus.

// goldenIndex parses names like "item3" or "radio1" into the index.
func goldenIndex(name string) int {
	n, err := strconv.Atoi(strings.TrimLeft(name, "abcdefghijklmnopqrstuvwxyz"))
	if err != nil {
		panic("bad golden focus name " + name)
	}
	return n
}

func undoGoldenState(focus string) UndoConfirmState {
	st := UndoConfirmState{
		Open:  true,
		JobID: "job-harbor",
		Line1: "Undo copy of 3 items to willow (5 steps)?",
		Line2: "Copied files are removed again.",
	}
	st.Focus = map[string]dlg.ID{"yes": UndoYes, "no": UndoNo}[focus]
	return st
}

func quitGoldenState(focus string, commands bool) QuitConfirmState {
	st := QuitConfirmState{Open: true}
	if commands {
		st.WarnLine1 = "Commands are still running."
		st.WarnLine2 = "Quitting will cancel running subprocesses."
	}
	st.Focus = map[string]dlg.ID{"stay": QuitStay, "quit": QuitQuit}[focus]
	return st
}

func listingFormatGoldenState(format panel.ListFormat, focus string) ListingFormatDialogState {
	st := ListingFormatDialogState{Open: true, ListFormat: format}
	switch focus {
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		st.Focus = dlg.ID(listingFormatPrefix + panel.ListFormatDialogRadios()[goldenIndex(focus)].Label)
	}
	return st
}

func compareFilterGoldenState(mode CompareFilterDialogMode, secondary bool, filter comparepkg.Filter, focus string) CompareFilterDialogState {
	st := CompareFilterDialogState{Open: true, Mode: mode, FocusSecondary: secondary, Filter: filter}
	switch focus {
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		st.Focus = compareFilterItemID(st.Radios()[goldenIndex(focus)].Filter)
	}
	return st
}

func jobsResultFilterGoldenState(filter JobResultFilter, focus string) JobsResultFilterDialogState {
	st := JobsResultFilterDialogState{Open: true, Filter: filter}
	switch focus {
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		st.Focus = JobsResultFilterView(st).Groups[0].Columns[0][goldenIndex(focus)].ID
	}
	return st
}

func hostKeyGoldenState(fingerprint, focus string) HostKeyDialogState {
	st := HostKeyDialogState{
		Open: true, Host: "harbor.example.net:22", KeyType: "ssh-ed25519", Fingerprint: fingerprint,
	}
	st.Focus = map[string]dlg.ID{"accept": HostKeyAccept, "save": HostKeySave, "reject": HostKeyReject}[focus]
	return st
}

func metaGoldenState(n int, checked, focus string) MetaDialogState {
	names := []string{"lantern", "orchard", "velvet", "timber", "ribbon"}
	descs := []string{"Shows the lantern count", "", "Velvet rating", "", "Ribbon colour"}
	st := MetaDialogState{Open: true}
	for i := range n {
		st.Entries = append(st.Entries, MetaEntry{Name: names[i], Description: descs[i]})
		on := checked == "all" || (checked == "mixed" && i%2 == 0)
		st.Checked = append(st.Checked, on)
	}
	switch focus {
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		st.Focus = MetaEntryID(goldenIndex(focus))
	}
	return st
}

func compareSelectionGoldenState(selected int, focus string) CompareSelectionDialogState {
	ids := map[string]dlg.ID{"drop": CompareSelectionDrop, "stash": CompareSelectionStash, "cancel": dlg.Cancel}
	return CompareSelectionDialogState{Open: true, Focus: ids[focus], Selected: selected}
}

func stashRestoreGoldenState(focus string) StashRestoreDialogState {
	ids := map[string]dlg.ID{"replace": StashReplace, "merge": StashMerge, "drop-stash": StashDropStash, "drop-all": StashDropAll, "cancel": dlg.Cancel}
	return StashRestoreDialogState{Open: true, Focus: ids[focus]}
}

func dedupReturnGoldenState(scope, focus string) DedupReturnDialogState {
	st := DedupReturnDialogState{Open: true, Dir: "/home/user/meadow/orchard"}
	switch scope {
	case "one":
		st.ScopeDirs = []string{"/home/user/meadow/orchard/willow"}
	case "many":
		for i := range 3 {
			st.ScopeDirs = append(st.ScopeDirs, fmt.Sprintf("/home/user/meadow/orchard/dir%d", i))
		}
	}
	st.Focus = map[string]dlg.ID{"first": DedupReturnFirst, "second": DedupReturnSecond, "cancel": dlg.Cancel}[focus]
	return st
}

func setElevateGoldenFocus(st *ElevateDialogState, focus string) {
	st.Focus = map[string]dlg.ID{"input": ElevatePassword, "elevate": ElevateRetry, "skip": ElevateSkip, "cancel": dlg.Cancel}[focus]
}

func setDebounceGoldenFocus(st *DebounceCalibrateDialogState, focus string) {
	st.Focus = map[string]dlg.ID{"in0": DebounceDelay, "in1": DebouncePreview, "ok": dlg.OK, "calibrate": DebounceCalibrate, "cancel": dlg.Cancel}[focus]
}

func setFlattenGoldenFocus(st *FlattenDialogState, focus string) {
	st.Focus = map[string]dlg.ID{"dest": FlattenDest, "rec": FlattenRecursive, "empty": FlattenRemoveEmpty, "ok": dlg.OK, "cancel": dlg.Cancel}[focus]
}

func setCompareMergeGoldenFocus(st *CompareMergeDialogState, focus string) {
	st.Focus = map[string]dlg.ID{
		"left": CompareMergeLeft, "right": CompareMergeRight, "missing": CompareMergeMissing, "modified": CompareMergeModified,
		"copy": CompareMergeCopy, "move": CompareMergeMove, "ok": dlg.OK, "cancel": dlg.Cancel,
	}[focus]
}
