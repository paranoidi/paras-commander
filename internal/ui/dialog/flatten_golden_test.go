package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// flattenGoldenState builds a flatten dialog. value: empty, typed, prefill (unedited suggestion),
// long (wider than the field, caret at the end), longmid (caret mid-text, scrolled), ghost (single
// completion candidate), dropdown (3 candidates), scrollbar (8 candidates). opts: letters r
// (Recursive), e (RemoveEmpty), i (DestPathInvalid), p (DestPathCheckPending).
func flattenGoldenState(value, opts, focus string) FlattenDialogState {
	st := FlattenDialogState{
		Open:        true,
		Recursive:   strings.Contains(opts, "r"),
		RemoveEmpty: strings.Contains(opts, "e"),
	}
	st.DestPathInvalid = strings.Contains(opts, "i")
	st.DestPathCheckPending = strings.Contains(opts, "p")
	names := func(n ...string) []pathpick.Candidate {
		var items []pathpick.Candidate
		for _, name := range n {
			items = append(items, pathpick.Candidate{Name: name, IsDir: true})
		}
		return items
	}
	set := func(v string, cursor int) {
		st.Destination.Value, st.Destination.Cursor = v, cursor
	}
	switch value {
	case "typed":
		set("/srv/harbor/orchard", 19)
	case "prefill":
		set("/srv/harbor/orchard/", 20)
		st.Destination.Prefill, st.Destination.PrefillPending = "/srv/harbor/orchard/", true
	case "long":
		v := "/srv/" + strings.Repeat("harbor-orchard-meadow/", 4) + "lantern"
		set(v, len(v))
	case "longmid":
		v := "/srv/" + strings.Repeat("harbor-orchard-meadow/", 4) + "lantern"
		set(v, 40)
		st.Destination.Scroll = 30
	case "ghost":
		set("/srv/harbor/or", 14)
		st.Destination.Completion.Set("/srv/harbor/or", 12, names("orchard"))
	case "dropdown":
		set("/srv/harbor/", 12)
		st.Destination.Completion.Set("/srv/harbor/o", 12, names("orchard", "otter", "oak"))
		st.Destination.Completion.Open = true
	case "scrollbar":
		set("/srv/harbor/", 12)
		st.Destination.Completion.Set("/srv/harbor/m", 12, names("meadow", "marble", "mast", "moss", "mill", "maple", "mint", "mule"))
		st.Destination.Completion.Open = true
		st.Destination.Completion.Selected = 6
		st.Destination.Completion.Scroll = 2
	}
	setFlattenGoldenFocus(&st, focus)
	return st
}

func TestFlattenGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, value, opts, focus string) {
		st := flattenGoldenState(value, opts, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s_opts-%s_focus-%s", w, h, value, opts, focus), w, h, func(s tcell.Screen, l Layout) {
			DrawFlattenDialog(s, l, st, asciiDialogStyles(), uiscrollbar.StyleThumb)
		}})
	}
	for _, focus := range []string{"dest", "ok"} {
		add(80, 24, "typed", "r", focus)
	}
	for _, opts := range []string{"-", "re", "e"} {
		add(80, 24, "typed", opts, "dest")
		add(80, 24, "typed", opts, "rec")
		add(80, 24, "typed", opts, "empty")
	}
	for _, value := range []string{"empty", "prefill", "long", "longmid", "ghost", "dropdown", "scrollbar"} {
		add(80, 24, value, "r", "dest")
	}
	add(80, 24, "long", "r", "ok")
	add(80, 24, "typed", "ri", "dest") // invalid destination
	add(80, 24, "typed", "ri", "ok")
	add(80, 24, "typed", "rip", "dest") // validation still pending: not painted invalid
	add(80, 24, "dropdown", "r", "rec") // an open dropdown stays painted off the input
	add(60, 24, "long", "r", "dest")    // narrow: width = W-4
	add(60, 24, "dropdown", "re", "ok")
	add(36, 24, "typed", "r", "cancel") // narrower still
	add(80, 12, "typed", "r", "dest")   // short screen: dropdown flips above the input
	add(80, 12, "scrollbar", "r", "dest")
	runSimpleGoldens(t, "flatten", cases)
}
