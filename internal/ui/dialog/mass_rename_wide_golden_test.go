package dialog

import (
	"fmt"
	"testing"
)

// The mass rename main screen on a terminal wide enough for all four radio / checkbox columns
// (120 columns): every mode and focus position, hints and error states.
func TestMassRenameWideGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(mode, data, opts, focus string, scroll int) {
		addMassRenameGolden(&cases, 120, 50, massRenameGoldenState(mode, data, opts, focus, scroll),
			fmt.Sprintf("wide_%s_%s_opts-%s_scroll%d_focus-%s", mode, data, opts, scroll, focus))
	}
	for _, focus := range []string{"r0", "r1", "r2", "r3", "o0", "o1", "o2", "o3", "find", "repl", "ok", "apply", "cancel"} {
		add("simple", "few", "-", focus, 0)
	}
	for _, focus := range []string{"r1", "o2", "find", "repl", "ok"} {
		add("regex", "few", "-", focus, 0)
	}
	for _, focus := range []string{"r2", "o0", "o1", "ok", "apply", "cancel"} {
		add("external", "few", "-", focus, 0)
	}
	for _, focus := range []string{"r3", "o0", "o1", "o2", "cw", "cp", "ok", "apply", "cancel"} {
		add("cap", "few", "-", focus, 0)
	}
	add("simple", "few", "mtix", "o0", 0)
	add("regex", "few", "ti", "o2", 0)
	add("cap", "few", "wp", "cp", 0)
	add("cap", "few", "mx", "o2", 0)
	add("external", "few", "m", "o0", 0)
	for _, mode := range []string{"simple", "regex", "external", "cap"} {
		add(mode, "none", "-", "find", 0)
		add(mode, "many", "-", "o0", 0)
		add(mode, "err", "-", "o0", 0)
		add(mode, "long", "-", "o0", 0)
	}
	add("simple", "many", "-", "find", 6)
	add("external", "many", "-", "r2", 20)
	add("simple", "same", "m", "find", 0)
	add("simple", "few", "r", "ok", 0)
	add("simple", "few", "r", "apply", 0)
	add("external", "few", "e", "ok", 0)
	add("external", "few", "er", "apply", 0)
	add("simple", "few", "v", "find", 0)
	add("regex", "few", "h", "find", 0)
	add("regex", "few", "h", "repl", 0)
	add("regex", "few", "v", "find", 0)
	runSimpleGoldens(t, "mass_rename", cases)
}
