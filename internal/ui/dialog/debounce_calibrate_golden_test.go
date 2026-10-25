package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// debounceGoldenState builds a calibrate dialog. variant: "edit", "empty", "status" (validation
// message), "long" (values wider than the fields), "await" / "collect" / "released" / "full"
// (measuring phase). focus: in0, in1, ok, calibrate, cancel.
func debounceGoldenState(variant, focus string) DebounceCalibrateDialogState {
	st := DebounceCalibrateDialogState{Open: true, Value: "40", Cursor: 2, PreviewValue: "150", PreviewCursor: 1}
	switch variant {
	case "empty":
		st.Value, st.Cursor, st.PreviewValue, st.PreviewCursor = "", 0, "", 0
	case "status":
		st.Status = "Enter 0–1000"
	case "long":
		v := strings.Repeat("1234567890", 7)
		st.Value, st.Cursor, st.PreviewValue, st.PreviewCursor = v, len(v), v, 3
	case "await", "collect", "released", "full":
		st.Phase = DebounceCalibrateMeasuring
		need := MeasureMinRepeatSamples()
		switch variant {
		case "collect":
			st.MeasureStep, st.HoldIndex = MeasureCollecting, 1
			st.Samples = make([]int64, 1+need/2)
			st.HoldSamples = 1
		case "released":
			st.Status = "Released too soon; hold until 0/8 on the bar."
		case "full":
			st.MeasureStep, st.HoldIndex = MeasureCollecting, MeasureHolds()-1
			st.Samples = make([]int64, need+4)
		}
	}
	setDebounceGoldenFocus(&st, focus)
	return st
}

func TestDebounceCalibrateGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, variant, focus string) {
		st := debounceGoldenState(variant, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s_focus-%s", w, h, variant, focus), w, h, func(s tcell.Screen, l Layout) {
			DrawDebounceCalibrateDialog(s, l, st, asciiDialogStyles())
		}})
	}
	for _, focus := range []string{"in0", "ok"} {
		add(80, 24, "edit", focus)
	}
	add(80, 24, "empty", "in0")
	add(80, 24, "empty", "in1")
	add(80, 24, "status", "in0")
	add(80, 24, "status", "ok")
	add(80, 24, "long", "in0")
	add(80, 24, "long", "in1")
	add(80, 24, "long", "cancel")
	for _, variant := range []string{"await", "collect", "released", "full"} {
		add(80, 24, variant, "in0")
	}
	add(60, 24, "edit", "in1")      // width = W-4
	add(48, 24, "status", "in0")    // exactly the 44 floor
	add(47, 24, "edit", "in0")      // too narrow to draw
	add(80, 12, "edit", "in0")      // too short to draw
	add(48, 24, "collect", "in0")   // measuring at the floor
	add(100, 40, "released", "in0") // large screen
	runSimpleGoldens(t, "debounce_calibrate", cases)
}
