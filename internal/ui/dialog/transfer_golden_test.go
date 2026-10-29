package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func init() { goldenBandRows["transfer"] = 40 }

// transferGoldenEntries is the multi-location preview list: n entries under /srv/harbor, a mix of
// nested files and directories.
func transferGoldenEntries(n int) []DeleteListEntry {
	names := []string{
		"lantern/", "orchard/willow.txt", "meadow/", "timber/ribbon.txt", "velvet.txt", "anchor/",
		"marble/otter.txt", "thicket.txt", "pebble/", "quartz/lichen.txt", "ember.txt", "harvest/",
		"juniper/bramble.txt", "saddle.txt",
	}
	var out []DeleteListEntry
	for i := 0; i < n; i++ {
		name := names[i%len(names)]
		e := DeleteListEntry{Name: strings.TrimSuffix(name, "/"), Path: "/srv/harbor/" + strings.TrimSuffix(name, "/"), Type: localfs.EntryFile}
		if strings.HasSuffix(name, "/") {
			e.Type = localfs.EntryDirectory
		}
		out = append(out, e)
	}
	return out
}

// transferGoldenState builds a transfer dialog. phase: dest or self. multi adds the common-root
// preview with entries entries. value: empty, typed, prefill, long, longmid, ghost, dropdown,
// scrollbar (the destination in the dest phase, the new name in the self phase for the first
// four). opts: p preserve permissions, t timestamps, d dereference, f flatten into destination,
// i invalid destination, n validation pending, r remote source, s scrolled entries.
func transferGoldenState(kind TransferKind, phase string, entries int, value, opts, focus string) TransferDialogState {
	st := TransferDialogState{
		Open:                 true,
		Kind:                 kind,
		PreservePermissions:  strings.Contains(opts, "p"),
		PreserveTimestamps:   strings.Contains(opts, "t"),
		DereferenceSymlinks:  strings.Contains(opts, "d"),
		FlattenIntoDest:      strings.Contains(opts, "f"),
		DestPathInvalid:      strings.Contains(opts, "i"),
		DestPathCheckPending: strings.Contains(opts, "n"),
		SourceIsRemote:       strings.Contains(opts, "r"),
	}
	if phase == "self" {
		st.Phase = TransferPhaseSelfCopyRename
		st.SelfCopyOrigBasename = "lantern.txt"
	}
	if entries > 0 {
		st.CommonRoot = "/home/alice/projects/harbor"
		st.Entries = transferGoldenEntries(entries)
		if strings.Contains(opts, "s") {
			st.EntriesScroll = 3
		}
	}
	names := func(n ...string) []pathpick.Candidate {
		var items []pathpick.Candidate
		for _, name := range n {
			items = append(items, pathpick.Candidate{Name: name, IsDir: true})
		}
		return items
	}
	f := &st.Destination
	if phase == "self" {
		f = &st.SelfCopyNewName
	}
	set := func(v string, cursor int) { f.Value, f.Cursor = v, cursor }
	switch value {
	case "typed":
		set("/srv/harbor/orchard", 19)
	case "prefill":
		set("/srv/harbor/orchard/", 20)
		f.Prefill, f.PrefillPending = "/srv/harbor/orchard/", true
	case "long":
		v := "/srv/" + strings.Repeat("harbor-orchard-meadow/", 4) + "lantern"
		set(v, len(v))
	case "longmid":
		v := "/srv/" + strings.Repeat("harbor-orchard-meadow/", 4) + "lantern"
		set(v, 40)
		f.Scroll = 30
	case "ghost":
		set("/srv/harbor/or", 14)
		f.Completion.Set("/srv/harbor/or", 12, names("orchard"))
	case "dropdown":
		set("/srv/harbor/", 12)
		f.Completion.Set("/srv/harbor/o", 12, names("orchard", "otter", "oak"))
		f.Completion.Open = true
	case "scrollbar":
		set("/srv/harbor/", 12)
		f.Completion.Set("/srv/harbor/m", 12, names("meadow", "marble", "mast", "moss", "mill", "maple", "mint", "mule"))
		f.Completion.Open = true
		f.Completion.Selected = 6
		f.Completion.Scroll = 2
	}
	setTransferGoldenFocus(&st, focus)
	return st
}

// setTransferGoldenFocus names a focus position: dest (the destination, or the new name in the
// self phase), perm, ts, deref, flat, ok, advanced and cancel. It is the only code that knows how
// the state encodes focus.
func setTransferGoldenFocus(st *TransferDialogState, focus string) {
	st.Focus = map[string]dlg.ID{
		"dest": TransferDest, "perm": TransferPerm, "ts": TransferTimes, "deref": TransferDeref, "flat": TransferFlatten,
		"ok": dlg.OK, "advanced": TransferAdvanced, "cancel": dlg.Cancel,
	}[focus]
	if focus == "dest" && st.Phase == TransferPhaseSelfCopyRename {
		st.Focus = TransferNewName
	}
}

// transferGoldenIconPainter stands in for the devicon painter: one marker glyph per row.
func transferGoldenIconPainter(screen tcell.Screen, x, y int, e DeleteListEntry, styles theme.Theme) {
	glyph := '#'
	if e.Type == localfs.EntryDirectory {
		glyph = '+'
	}
	screen.SetContent(x, y, glyph, nil, styles.DialogText)
}

func TestTransferGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, kind TransferKind, phase string, entries int, value, opts, focus string) {
		st := transferGoldenState(kind, phase, entries, value, opts, focus)
		kindName := map[TransferKind]string{TransferKindCopy: "copy", TransferKindMove: "move"}[kind]
		name := fmt.Sprintf("%dx%d_%s_%s_n%d_%s_opts-%s_focus-%s", w, h, kindName, phase, entries, value, opts, focus)
		cases = append(cases, simpleGolden{name, w, h, func(s tcell.Screen, l Layout) {
			ctx := DialogRenderContext{Styles: asciiDialogStyles(), UserHomeDir: "/home/alice", ShowIcons: true, IconLead: 2, ScrollbarStyle: uiscrollbar.StyleThumb}
			DrawTransferDialog(s, l, st, ctx, transferGoldenIconPainter)
		}})
	}
	copyK, moveK := TransferKindCopy, TransferKindMove

	// Single location, copy: every focus position, then the option and value variants.
	for _, focus := range []string{"dest", "ok"} {
		add(80, 24, copyK, "dest", 0, "typed", "pt", focus)
	}
	for _, opts := range []string{"-", "ptd", "d"} {
		add(80, 24, copyK, "dest", 0, "typed", opts, "perm")
		add(80, 24, copyK, "dest", 0, "typed", opts, "deref")
	}
	for _, value := range []string{"empty", "prefill", "long", "longmid", "ghost", "dropdown", "scrollbar"} {
		add(80, 24, copyK, "dest", 0, value, "p", "dest")
	}
	add(80, 24, copyK, "dest", 0, "long", "p", "ok")
	add(80, 24, copyK, "dest", 0, "typed", "pi", "dest") // invalid destination
	add(80, 24, copyK, "dest", 0, "typed", "pi", "ok")
	add(80, 24, copyK, "dest", 0, "typed", "pin", "dest") // validation pending: not painted invalid
	add(80, 24, copyK, "dest", 0, "typed", "pr", "deref") // dereference greyed out (remote source)
	add(80, 24, copyK, "dest", 0, "typed", "pdr", "ok")
	add(80, 24, copyK, "dest", 0, "dropdown", "p", "perm") // an open dropdown stays painted off the input
	add(80, 24, copyK, "dest", 0, "dropdown", "pt", "advanced")
	add(60, 24, copyK, "dest", 0, "long", "p", "dest") // narrow
	add(60, 24, copyK, "dest", 0, "dropdown", "pt", "ok")
	add(36, 24, copyK, "dest", 0, "typed", "p", "cancel") // narrower still
	add(80, 12, copyK, "dest", 0, "typed", "p", "dest")   // clamped height
	add(80, 12, copyK, "dest", 0, "scrollbar", "p", "dest")
	add(80, 9, copyK, "dest", 0, "typed", "p", "ok")

	// Single location, move.
	for _, focus := range []string{"dest", "ok"} {
		add(80, 24, moveK, "dest", 0, "typed", "-", focus)
	}
	for _, value := range []string{"empty", "prefill", "long", "ghost", "dropdown", "scrollbar"} {
		add(80, 24, moveK, "dest", 0, value, "-", "dest")
	}
	add(80, 24, moveK, "dest", 0, "typed", "i", "dest")
	add(80, 24, moveK, "dest", 0, "typed", "i", "advanced")
	add(80, 24, moveK, "dest", 0, "dropdown", "-", "ok")
	add(60, 24, moveK, "dest", 0, "long", "-", "dest")
	add(36, 24, moveK, "dest", 0, "typed", "-", "advanced")
	add(80, 8, moveK, "dest", 0, "dropdown", "-", "dest")
	add(80, 6, moveK, "dest", 0, "typed", "-", "cancel")

	// Self-copy / self-move rename phase.
	for _, k := range []TransferKind{copyK, moveK} {
		for _, focus := range []string{"dest", "ok"} {
			add(80, 24, k, "self", 0, "typed", "pt", focus)
		}
		for _, value := range []string{"empty", "prefill", "long", "longmid"} {
			add(80, 24, k, "self", 0, value, "-", "dest")
		}
	}
	add(60, 24, copyK, "self", 0, "long", "-", "dest")
	add(36, 24, copyK, "self", 0, "typed", "-", "ok")
	add(80, 8, moveK, "self", 0, "typed", "-", "dest")

	// Multi-location: Source header, Result preview.
	for _, focus := range []string{"dest", "ok"} {
		add(80, 30, copyK, "dest", 14, "typed", "pt", focus)
	}
	for _, focus := range []string{"dest", "ok"} {
		add(80, 30, moveK, "dest", 14, "typed", "-", focus)
	}
	add(80, 30, copyK, "dest", 14, "typed", "ptf", "flat") // flatten on: basename labels
	add(80, 30, moveK, "dest", 14, "typed", "f", "flat")
	add(80, 30, copyK, "dest", 14, "typed", "pts", "dest") // scrolled preview
	add(80, 30, copyK, "dest", 4, "typed", "p", "dest")    // short list: padded viewport
	add(80, 30, copyK, "dest", 1, "typed", "p", "dest")
	add(80, 30, moveK, "dest", 2, "typed", "-", "ok")
	add(80, 30, copyK, "dest", 0+14, "empty", "p", "dest")
	add(80, 30, copyK, "dest", 14, "prefill", "p", "dest")
	add(80, 30, copyK, "dest", 14, "long", "p", "dest")
	add(80, 30, copyK, "dest", 14, "ghost", "p", "dest")
	add(80, 30, copyK, "dest", 14, "dropdown", "p", "dest")
	add(80, 30, copyK, "dest", 14, "dropdown", "p", "flat")
	add(80, 30, copyK, "dest", 14, "scrollbar", "p", "dest")
	add(80, 30, copyK, "dest", 14, "typed", "pi", "dest")
	add(80, 30, copyK, "dest", 14, "typed", "pin", "ok")
	add(80, 30, copyK, "dest", 14, "typed", "pr", "deref")
	add(80, 30, copyK, "dest", 14, "typed", "pdr", "deref")
	add(80, 24, copyK, "dest", 14, "typed", "p", "dest") // viewport shrinks with the screen
	add(80, 24, copyK, "dest", 14, "dropdown", "pt", "ok")
	add(80, 24, moveK, "dest", 14, "typed", "-", "dest")
	add(80, 20, copyK, "dest", 14, "typed", "p", "dest") // clamped height
	add(80, 16, copyK, "dest", 14, "typed", "pt", "ok")
	add(80, 12, moveK, "dest", 14, "typed", "-", "ok")
	add(60, 30, copyK, "dest", 14, "typed", "p", "dest") // narrow
	add(60, 30, copyK, "dest", 14, "dropdown", "pt", "perm")
	add(36, 30, moveK, "dest", 14, "typed", "-", "dest") // narrower still
	runSimpleGoldens(t, "transfer", cases)
}

// TestTransferGoldenRenderWide covers a wide screen where long preview labels and the common root
// widen the dialog beyond the preferred width.
func TestTransferGoldenRenderWide(t *testing.T) {
	var cases []simpleGolden
	st := transferGoldenState(TransferKindCopy, "dest", 14, "typed", "pt", "dest")
	st.CommonRoot = "/home/alice/projects/" + strings.Repeat("harbor-orchard-meadow/", 3) + "lantern"
	st.Entries[0].Name = strings.Repeat("willow-ribbon-marble/", 4) + "velvet"
	cases = append(cases, simpleGolden{"140x30_copy_wide", 140, 30, func(s tcell.Screen, l Layout) {
		ctx := DialogRenderContext{Styles: asciiDialogStyles(), UserHomeDir: "/home/alice", ShowIcons: true, IconLead: 2, ScrollbarStyle: uiscrollbar.StyleThumb}
		DrawTransferDialog(s, l, st, ctx, transferGoldenIconPainter)
	}})
	cases = append(cases, simpleGolden{"100x30_copy_wide", 100, 30, func(s tcell.Screen, l Layout) {
		ctx := DialogRenderContext{Styles: asciiDialogStyles(), UserHomeDir: "/home/alice", ShowIcons: true, IconLead: 2, ScrollbarStyle: uiscrollbar.StyleThumb}
		DrawTransferDialog(s, l, st, ctx, transferGoldenIconPainter)
	}})
	noIcons := transferGoldenState(TransferKindMove, "dest", 6, "typed", "-", "ok")
	cases = append(cases, simpleGolden{"80x30_move_noicons", 80, 30, func(s tcell.Screen, l Layout) {
		ctx := DialogRenderContext{Styles: asciiDialogStyles(), UserHomeDir: "/home/alice"}
		DrawTransferDialog(s, l, noIcons, ctx, nil)
	}})
	runSimpleGoldens(t, "transfer", cases)
}
