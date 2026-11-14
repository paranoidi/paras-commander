package dialog

import (
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/rsync"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Text input indexes of the Rsync dialog (FileDialogState.Fields).
const (
	RsyncFieldDest      = 0
	RsyncFieldExclude   = 1
	RsyncFieldBandwidth = 2
	RsyncFieldExtra     = 3
)

// Widget IDs of the Rsync dialog's radios and checkboxes. The text inputs are FileFieldID(i); the
// buttons are dlg.OK and dlg.Cancel.
const (
	RsyncModeUpdate  = dlg.ID("rsync-mode-update")
	RsyncModeMirror  = dlg.ID("rsync-mode-mirror")
	RsyncModeMove    = dlg.ID("rsync-mode-move")
	RsyncArchive     = dlg.ID("rsync-archive")
	RsyncChecksum    = dlg.ID("rsync-checksum")
	RsyncOnlyNewer   = dlg.ID("rsync-only-newer")
	RsyncSkipExist   = dlg.ID("rsync-skip-existing")
	RsyncContentsDir = dlg.ID("rsync-contents")
	RsyncCompress    = dlg.ID("rsync-compress")
	RsyncDryRun      = dlg.ID("rsync-dry-run")
)

// RsyncPreviewList is the ScrollOnly list that shows the wrapped command preview.
const RsyncPreviewList = dlg.ID("rsync-preview")

// rsyncErrorMaxRows caps the rows a long error message takes; the rest ends in an ellipsis.
const rsyncErrorMaxRows = 3

// rsyncColumnGap is the blank cells between the two checkbox columns.
const rsyncColumnGap = 2

// rsyncBox is one Rsync dialog checkbox: the widget and the option it toggles. dirsOnly greys it
// out unless every source is a directory.
type rsyncBox struct {
	id       dlg.ID
	label    string
	mnemonic rune
	field    func(*rsync.Options) *bool
	dirsOnly bool
	tooltip  string
}

// rsyncBoxColumns are the checkbox grid's two columns, in display order: the view lays them out
// and applyRsyncEvent toggles from the same table.
var rsyncBoxColumns = [2][]rsyncBox{
	{
		{RsyncArchive, "Archive (-a)", 'a', func(o *rsync.Options) *bool { return &o.Archive }, false, "Recurse and keep permissions, times, symlinks, owner, group and device files. Off: recurse only (-r), nothing preserved."},
		{RsyncOnlyNewer, "Only newer (-u)", 'n', func(o *rsync.Options) *bool { return &o.OnlyNewer }, false, ""},
		{RsyncContentsDir, "Copy dir contents", 'd', func(o *rsync.Options) *bool { return &o.ContentsOnly }, true, "Copy what is inside each source directory, not the directory itself (adds a trailing slash: src/ instead of src)."},
		{RsyncDryRun, "Dry run (-n)", 'r', func(o *rsync.Options) *bool { return &o.DryRun }, false, ""},
	},
	{
		{RsyncChecksum, "Checksum compare (-c)", 'h', func(o *rsync.Options) *bool { return &o.Checksum }, false, "Decide what changed by comparing file contents (checksums) instead of size and modification time. Catches same-size edits but reads every file on both sides, so it is much slower."},
		{RsyncSkipExist, "Skip existing", 's', func(o *rsync.Options) *bool { return &o.SkipExisting }, false, "Never touch a file that already exists at the destination, even if it differs (--ignore-existing)."},
		{RsyncCompress, "Compress (-z)", 'z', func(o *rsync.Options) *bool { return &o.Compress }, false, "Compress data in transit. Helps only over a slow network; on local copies it just costs CPU."},
	},
}

// rsyncView is the widget spec of the Rsync dialog: the Mode label and its radios on a row of their
// own (one Tab segment), a blank row, the Destination input, the checkbox grid (one Tab segment;
// Copy dir contents is greyed out unless every source is a directory), the Exclude, Bandwidth limit and Extra arguments inputs (a segment each) and the
// command preview row (the error of the options when they do not build). Mirror makes OK
// destructive: it deletes destination files.
func rsyncView(st FileDialogState, ctx FileDialogViewContext) dlg.Spec {
	markerW := dlg.MarkerWidth(ctx.Styles)
	spec := dlg.Spec{
		Title: fileDialogOuterTitle(st), Width: fileDialogWidth(ctx.Layout.Width, st, ctx.IconLead), Unclamped: true, MarkerWidth: markerW,
		Buttons: []dlg.Button{
			{ID: dlg.OK, Label: "OK", Mnemonic: 'O', Destructive: st.RsyncOptions.Mode == rsync.ModeMirror},
			dlg.CancelButton,
		},
	}
	o := st.RsyncOptions
	input := func(i int) []dlg.Item {
		f := st.Fields[i]
		return []dlg.Item{{Kind: dlg.KindLabel, Label: f.Label + ":"}, fileFieldInput(FileFieldID(i), f, f.InputInvalid)}
	}

	// Mode label, then the radios on a row of their own.
	mode := func(id dlg.ID, label string, mnemonic rune, on bool) dlg.Item {
		return dlg.Item{ID: id, Kind: dlg.KindRadio, Label: label, Mnemonic: mnemonic, On: on, Gap: 1}
	}
	modeRow := dlg.Item{Kind: dlg.KindRow, TabToSelected: true, Cells: []dlg.Item{
		mode(RsyncModeUpdate, "Update", 'u', o.Mode == rsync.ModeUpdate),
		mode(RsyncModeMirror, "Mirror (delete extra)", 'm', o.Mode == rsync.ModeMirror),
		mode(RsyncModeMove, "Move", 'v', o.Mode == rsync.ModeMove),
	}}

	// Two columns of checkboxes, as wide as their widest label so the columns line up.
	left, right := rsyncBoxColumns[0], rsyncBoxColumns[1]
	leftW := 0
	for _, b := range left {
		leftW = max(leftW, markerW+utf8.RuneCountInString(b.label))
	}
	cell := func(b rsyncBox, width, gap int) dlg.Item {
		return dlg.Item{ID: b.id, Kind: dlg.KindCheckbox, Label: b.label, Mnemonic: b.mnemonic, On: *b.field(&o), Disabled: b.dirsOnly && !st.RsyncAllDirs, Tooltip: b.tooltip, Width: width, Gap: gap}
	}
	var options []dlg.Item
	for i, l := range left {
		cells := []dlg.Item{cell(l, leftW, rsyncColumnGap)}
		if i < len(right) {
			cells = append(cells, cell(right[i], 0, 0))
		} else {
			cells[0].Gap = 0
		}
		options = append(options, dlg.Item{Kind: dlg.KindRow, Cells: cells})
	}

	// The preview wraps to the row width, so its height is known only with the dialog's width: the
	// list is a placeholder of the rows it wants at the frame's row width, which fillList below
	// shortens to the screen and rebuilds at the width the list is placed at.
	rowW := dlg.RowWidth(ctx.Layout.Width, ctx.Layout.Height, dlg.Spec{Width: spec.Width, Unclamped: true, Buttons: spec.Buttons})
	preview := []dlg.Item{{ID: RsyncPreviewList, Kind: dlg.KindList, ScrollOnly: true, Height: len(textutil.WrapTokens(st.RsyncPreview, rowW))}}
	if st.RsyncError != "" {
		preview = nil
		for _, line := range textutil.ClampLines(primitive.WrapText(st.RsyncError, rowW), rsyncErrorMaxRows, rowW) {
			preview = append(preview, dlg.Item{Kind: dlg.KindLabel, Label: line, Style: dlg.TextError})
		}
	}

	spec.Groups = []dlg.Group{
		{Columns: [][]dlg.Item{{{Kind: dlg.KindLabel, Label: "Mode:"}, modeRow, {Kind: dlg.KindLabel}}}},
		{Columns: [][]dlg.Item{append(input(RsyncFieldDest), dlg.Item{Kind: dlg.KindSeparator})}},
		{Columns: [][]dlg.Item{append(options, dlg.Item{Kind: dlg.KindSeparator})}},
		{Columns: [][]dlg.Item{append(append(input(RsyncFieldExclude), input(RsyncFieldBandwidth)...), input(RsyncFieldExtra)...)}},
		{Columns: [][]dlg.Item{append([]dlg.Item{{Kind: dlg.KindSeparator}}, preview...)}},
	}
	spec, _ = fillList(spec, ctx.Layout, RsyncPreviewList, func(height, width int) dlg.Item {
		lines := textutil.ClampLines(textutil.WrapTokens(st.RsyncPreview, width), height, width)
		rows := make([]dlg.ListRow, len(lines))
		for i, l := range lines {
			rows[i] = dlg.ListRow{Text: l}
		}
		return dlg.Item{ID: RsyncPreviewList, Kind: dlg.KindList, ScrollOnly: true, Fit: dlg.FitNone, Height: height, RowCount: len(rows), Rows: rows}
	})
	return spec
}

// applyRsyncEvent folds an activation of the Rsync dialog's options into the state: a radio selects
// its mode, a checkbox toggles.
func (st *FileDialogState) applyRsyncEvent(ev dlg.Event) {
	o := &st.RsyncOptions
	switch ev.ID {
	case RsyncModeUpdate:
		o.Mode = rsync.ModeUpdate
	case RsyncModeMirror:
		o.Mode = rsync.ModeMirror
	case RsyncModeMove:
		o.Mode = rsync.ModeMove
	}
	for _, col := range rsyncBoxColumns {
		for _, b := range col {
			if b.id == ev.ID {
				f := b.field(o)
				*f = !*f
			}
		}
	}
}
