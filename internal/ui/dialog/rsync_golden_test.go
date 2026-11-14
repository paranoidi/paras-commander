package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/rsync"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the Rsync dialog: every focus position, the modes, the checkbox states, the
// greyed-out Copy dir contents, the command preview row and its error replacement, and edge
// sizes. The cases are named <w>x<h>_<value>_opts-<letters>_focus-<name>; setRsyncGoldenFocus is
// the only code that knows how the state encodes focus.

func init() { goldenBandRows["rsync"] = 40 }

func addRsyncGolden(cases *[]simpleGolden, w, h int, st FileDialogState, label string) {
	name := fmt.Sprintf("%dx%d_%s", w, h, label)
	*cases = append(*cases, simpleGolden{name, w, h, func(s tcell.Screen, l Layout) {
		DrawFileDialog(s, l, st, fileGoldenCtx(), nil)
	}})
}

// setRsyncGoldenFocus names a focus position: f0 (Destination), f1 (Exclude), f2 (Bandwidth limit),
// f3 (Extra arguments), mode (the selected mode radio), cb0.. (the checkboxes in reading order),
// ok, cancel.
func setRsyncGoldenFocus(st *FileDialogState, focus string) {
	boxes := []dlg.ID{RsyncArchive, RsyncChecksum, RsyncOnlyNewer, RsyncSkipExist, RsyncContentsDir, RsyncCompress, RsyncDryRun}
	switch focus {
	case "f0", "f1", "f2", "f3":
		st.Focus = FileFieldID(goldenIndex(focus))
	case "mode":
		st.Focus = map[rsync.Mode]dlg.ID{rsync.ModeUpdate: RsyncModeUpdate, rsync.ModeMirror: RsyncModeMirror, rsync.ModeMove: RsyncModeMove}[st.RsyncOptions.Mode]
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		st.Focus = boxes[goldenIndex(focus)]
	}
}

// rsyncGoldenState builds the Rsync dialog. value: empty, typed (a destination path), long (one
// wider than the field), filled (every input has text). opts: m Mirror, v Move, d every source is
// a directory, c Copy dir contents on (needs d), a Archive off, x Checksum, u Only newer, s Skip
// existing, z Compress, n Dry run, e an error in place of the preview, k the preview of a long
// command, l a very long one that wraps over several rows, i invalid destination, t a two-item selection.
func rsyncGoldenState(value, opts, focus string) FileDialogState {
	has := func(c string) bool { return strings.Contains(opts, c) }
	st := FileDialogState{
		Open:       true,
		DialogType: FileDialogRsync,
		Fields: []FileDialogField{
			{Label: "Destination", PathPicker: true},
			{Label: "Exclude"},
			{Label: "Bandwidth limit (KiB/s)"},
			{Label: "Extra arguments"},
		},
		RsyncSources: pathloc.PathsForTest("/srv/harbor/orchard"),
		RsyncAllDirs: has("d"),
		RsyncOptions: rsync.Options{
			Archive: !has("a"), Checksum: has("x"), OnlyNewer: has("u"), SkipExisting: has("s"),
			ContentsOnly: has("c"), Compress: has("z"), DryRun: has("n"),
		},
		RsyncPreview: strings.Fields("rsync -a /srv/harbor/orchard /srv/meadow/"),
	}
	switch {
	case has("m"):
		st.RsyncOptions.Mode = rsync.ModeMirror
	case has("v"):
		st.RsyncOptions.Mode = rsync.ModeMove
	}
	if has("t") {
		st.RsyncSources = pathloc.PathsForTest("/srv/harbor/orchard", "/srv/harbor/willow")
	}
	switch value {
	case "typed":
		st.Fields[0].Value, st.Fields[0].Cursor = "/srv/meadow/", 12
	case "long":
		v := "/srv/" + strings.Repeat("harbor-orchard-meadow/", 5) + "lantern"
		st.Fields[0].Value, st.Fields[0].Cursor = v, len(v)
	case "filled":
		st.Fields[0].Value, st.Fields[0].Cursor = "/srv/meadow/", 12
		st.Fields[1].Value, st.Fields[1].Cursor = "*.tmp node_modules", 18
		st.Fields[2].Value, st.Fields[2].Cursor = "512", 3
		st.Fields[3].Value, st.Fields[3].Cursor = "--hard-links --acls", 19
	}
	if has("k") {
		st.RsyncPreview = strings.Fields("rsync -a -c -z --delete --bwlimit=512 '--exclude=*.tmp' --exclude=node_modules --hard-links --acls /srv/harbor/orchard /srv/meadow/")
	}
	if has("l") {
		var argv []string
		for _, w := range []string{"harbor-orchard-meadow", "lantern-willow-compass", "meadow-thistle-ember", "granite-sparrow-ribbon"} {
			argv = append(argv, "/srv/"+w+"/"+strings.Repeat("quiet-pebble-canyon/", 2)+w+".dat")
		}
		argv = append([]string{"rsync", "-a", "--exclude=*.tmp pending", "--exclude=node_modules"}, argv...)
		st.RsyncPreview = rsync.DisplayArgs(append(argv, "/srv/meadow/"))
	}
	if has("e") {
		st.RsyncPreview, st.RsyncError = nil, "Bandwidth limit must be a whole number of KiB/s"
		st.Fields[2].Value, st.Fields[2].Cursor = "fast", 4
	}
	st.Fields[0].InputInvalid = has("i")
	setRsyncGoldenFocus(&st, focus)
	return st
}

func TestRsyncGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, value, opts, focus string) {
		addRsyncGolden(&cases, w, h, rsyncGoldenState(value, opts, focus), fmt.Sprintf("%s_opts-%s_focus-%s", value, opts, focus))
	}
	for _, focus := range []string{"f0", "f1", "f2", "f3", "mode", "cb0", "cb1", "cb4", "cb6", "ok", "cancel"} {
		add(80, 30, "filled", "-", focus)
	}
	for _, value := range []string{"empty", "typed", "long"} {
		add(80, 30, value, "-", "f0")
	}
	add(80, 30, "typed", "-", "cb5") // Compress tooltip (Checksum compare: the filled cb1 case)
	add(80, 30, "typed", "d", "cb4") // Copy dir contents enabled
	add(80, 30, "typed", "dc", "cb4")
	add(80, 30, "typed", "m", "mode") // Mirror: destructive OK
	add(80, 30, "typed", "m", "ok")
	add(80, 30, "typed", "v", "mode")
	add(80, 30, "typed", "axusz", "cb0")
	add(80, 30, "typed", "n", "cb6")
	add(80, 30, "typed", "t", "f0") // a multi-item selection names its count
	add(80, 30, "typed", "k", "f0")
	add(80, 30, "typed", "e", "f2")
	add(80, 30, "typed", "i", "f0") // invalid destination
	add(100, 24, "filled", "mk", "f0")
	add(60, 30, "typed", "k", "f0") // narrow: the preview is cut in the middle
	add(60, 30, "typed", "e", "f2")
	add(80, 30, "typed", "l", "f0") // a long command wraps between arguments
	add(80, 24, "typed", "l", "f0") // a short screen: fewer rows, the last ends in an ellipsis
	add(80, 26, "typed", "l", "f0")
	add(36, 30, "typed", "l", "f0") // narrow: a path wider than the row is hard-broken
	add(36, 30, "typed", "-", "f0")
	add(80, 24, "typed", "-", "f0") // short screens
	add(80, 20, "typed", "-", "ok")
	add(80, 12, "typed", "-", "ok")
	runSimpleGoldens(t, "rsync", cases)
}
