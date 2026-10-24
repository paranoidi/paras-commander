package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// Render goldens for the Select / Unselect group dialog (harness: simple_dialog_golden_test.go).
// The screen is dumped in full. groupSelectGoldenFocus is the only code that knows how the state
// encodes focus.

func init() { goldenBandRows["group_select"] = 26 }

var groupSelectGoldenFocusNames = []string{
	"shell", "regex", "simple",
	"videos", "pictures", "archives", "audio", "documents",
	"pattern", "filesonly", "dirsonly", "case", "fullpath", "includemeta", "onlymeta",
	"ok", "cancel",
}

func groupSelectGoldenFocus(t *testing.T, st *GroupSelectState, name string) {
	t.Helper()
	st.Focus = patternGoldenFocus(t, name)
}

func groupSelectGoldenState(t *testing.T, variant, focus string) GroupSelectState {
	t.Helper()
	st := GroupSelectState{Open: true, Mode: "select", Context: "panel", PatternMode: panel.GroupPatternShell}
	typed := func(text string) {
		st.Text, st.TextCursor = text, len([]rune(text))
	}
	switch variant {
	case "empty":
	case "unselect":
		st.Mode = "unselect"
		typed("*.mp4")
		st.PreviewShow, st.PreviewFiles, st.PreviewFolders = true, 3, 2
	case "typed":
		typed("*.mp4")
		st.PreviewShow, st.PreviewFiles, st.PreviewFolders = true, 3, 2
	case "many":
		typed("*")
		st.PreviewShow, st.PreviewFiles, st.PreviewFolders = true, 12345, 678
	case "zero":
		typed("zzz")
		st.PreviewShow = true
	case "bad-regex":
		typed("(harbor")
		st.PatternMode = panel.GroupPatternRegex
	case "compile-hint":
		typed("orchard")
		st.PatternCompileHint = "nothing compiled"
	case "scrolled":
		typed(strings.Repeat("meadow-", 12))
		st.TextScroll = 30
		st.PreviewShow, st.PreviewFiles = true, 1
	case "options":
		typed("*.zip")
		st.Types[FilterTypeVideos], st.Types[FilterTypeArchives] = true, true
		st.CaseSensitive, st.FilesOnly = true, true
		st.PatternMode = panel.GroupPatternRegex
		st.PreviewShow, st.PreviewFiles, st.PreviewFolders = true, 9, 1
	case "dirs-only":
		typed("anchor")
		st.Types[FilterTypePictures], st.Types[FilterTypeDocuments] = true, true
		st.DirsOnly = true
		st.PreviewShow, st.PreviewFolders = true, 4
	case "find":
		st.Context = "find"
		typed("lantern")
		st.FullPath = true
		st.PreviewShow, st.PreviewFiles = true, 5
	case "find-empty":
		st.Context = "find"
	case "output":
		st.Context = "output"
		st.Mode = "unselect"
		typed("quartz")
		st.PreviewShow, st.PreviewFiles = true, 2
	case "meta":
		st.MetaColumnCount = 2
		st.IncludeMetaColumns = true
		typed("lantern")
		st.PreviewShow, st.PreviewFiles = true, 5
	case "meta-only":
		st.MetaColumnCount = 2
		st.IncludeMetaColumns, st.OnlyMetaColumns = true, true
		st.Types[FilterTypeAudio] = true
	default:
		t.Fatalf("unknown group select variant %q", variant)
	}
	groupSelectGoldenFocus(t, &st, focus)
	return st
}

func TestGroupSelectGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, variant, focus string) {
		st := groupSelectGoldenState(t, variant, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s_focus-%s", w, h, variant, focus), w, h, func(s tcell.Screen, l Layout) {
			DrawGroupSelectDialog(s, l, st, asciiDialogStyles())
		}})
	}
	for _, pos := range groupSelectGoldenFocusNames {
		switch pos {
		case "includemeta", "onlymeta", "fullpath":
			continue
		}
		add(80, 24, "empty", pos)
	}
	for _, pos := range groupSelectGoldenFocusNames {
		if pos != "fullpath" {
			add(80, 25, "meta", pos)
		}
	}
	for _, pos := range groupSelectGoldenFocusNames {
		switch pos {
		case "includemeta", "onlymeta":
			continue
		}
		add(80, 24, "find", pos)
	}
	for _, v := range []string{"typed", "unselect", "many", "zero", "bad-regex", "compile-hint", "scrolled", "options", "dirs-only", "output", "find-empty"} {
		for _, pos := range []string{"pattern", "shell", "ok"} {
			add(80, 24, v, pos)
		}
	}
	add(80, 24, "options", "case")
	add(80, 24, "options", "videos")
	add(80, 24, "dirs-only", "videos")
	add(80, 24, "dirs-only", "dirsonly")
	add(80, 25, "meta-only", "onlymeta")
	add(80, 25, "meta-only", "audio")
	add(34, 24, "typed", "pattern")
	add(34, 24, "find", "fullpath")
	add(34, 25, "meta", "ok")
	add(70, 24, "scrolled", "pattern")
	add(80, 12, "typed", "pattern")
	runSimpleGoldens(t, "group_select", cases)
}
