package dialog

import (
	"fmt"
	"strings"

	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// elevateGoldenState builds an elevate prompt. variant: "empty", "typed", "long" (a password
// longer than the field), "badpw" (typed, previous password rejected), "nopw" (no password
// field), "noop" (typed, no operation name) or "longpath". focus: input, elevate, skip, cancel.
func elevateGoldenState(variant, focus string) ElevateDialogState {
	st := ElevateDialogState{Open: true, Op: "rename", Path: "/srv/harbor/orchard/meadow.txt"}
	switch variant {
	case "typed", "badpw", "noop":
		st.Password = FileDialogField{Value: "willow42", Cursor: 8}
	case "long":
		v := strings.Repeat("lantern", 12)
		st.Password = FileDialogField{Value: v, Cursor: len(v)}
	case "nopw":
		st.NoPassword = true
	case "longpath":
		st.Op = "delete"
		st.Path = "/srv/" + strings.Repeat("harbor-orchard-meadow/", 5) + "lantern.txt"
	}
	st.BadPassword = variant == "badpw"
	if variant == "noop" {
		st.Op = ""
	}
	setElevateGoldenFocus(&st, focus)
	return st
}

func TestElevateGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, variant, focus string) {
		st := elevateGoldenState(variant, focus)
		cases = append(cases, simpleGolden{name: fmt.Sprintf("%dx%d_%s_focus-%s", w, h, variant, focus), w: w, h: h,
			draw: func(s tcell.Screen, l Layout) { DrawElevateDialog(s, l, st, asciiDialogStyles()) }})
	}
	for _, focus := range []string{"input"} {
		add(80, 24, "empty", focus)
	}
	add(80, 24, "typed", "input")
	add(80, 24, "typed", "elevate")
	add(80, 24, "long", "input")
	add(80, 24, "long", "cancel")
	add(80, 24, "badpw", "input")
	add(80, 24, "badpw", "skip")
	add(80, 24, "noop", "input")
	add(80, 24, "longpath", "input")
	for _, focus := range []string{"elevate"} {
		add(80, 24, "nopw", focus)
	}
	add(80, 24, "longpath", "elevate") // unused password text, long path
	add(50, 24, "typed", "input")      // narrow: width = W-4
	add(46, 24, "badpw", "elevate")    // below the 44 floor
	add(40, 24, "nopw", "cancel")      // narrower than the floor: W-2 at most
	runSimpleGoldens(t, "elevate", cases)
}

// archivePasswordGoldenState builds an archive password prompt. variant: "typed" or "bad"
// (typed, previous password rejected). focus: input, ok, cancel.
func archivePasswordGoldenState(variant, focus string) ArchivePasswordDialogState {
	st := ArchivePasswordDialogState{
		Open: true, Path: "/srv/harbor/orchard.7z", BadPassword: variant == "bad",
		Password: FileDialogField{Value: "willow42", Cursor: 8},
	}
	st.Focus = map[string]dlg.ID{"input": ArchivePasswordInput, "ok": dlg.OK, "cancel": dlg.Cancel}[focus]
	return st
}

func TestArchivePasswordGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, variant, focus string) {
		st := archivePasswordGoldenState(variant, focus)
		cases = append(cases, simpleGolden{name: fmt.Sprintf("%dx%d_%s_focus-%s", w, h, variant, focus), w: w, h: h,
			draw: func(s tcell.Screen, l Layout) { DrawArchivePasswordDialog(s, l, st, asciiDialogStyles()) }})
	}
	add(80, 24, "typed", "input")
	add(80, 24, "bad", "ok")
	add(80, 24, "bad", "cancel")
	add(46, 24, "typed", "input") // below the 44 floor
	runSimpleGoldens(t, "archive_password", cases)
}
