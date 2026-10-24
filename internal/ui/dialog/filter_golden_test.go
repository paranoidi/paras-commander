package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the Filter dialog (harness: simple_dialog_golden_test.go). The screen is
// dumped in full. filterGoldenFocus is the only code that knows how the state encodes focus.

func init() { goldenBandRows["filter"] = 26 }

var filterGoldenFocusNames = []string{
	"shell", "regex", "simple",
	"videos", "pictures", "archives", "audio", "documents",
	"pattern", "filesonly", "dirsonly", "invert", "case", "includemeta", "onlymeta",
	"ok", "cancel",
}

// patternGoldenFocus names a focus position of the Filter and Select group dialogs (shared
// widget IDs).
func patternGoldenFocus(t *testing.T, name string) dlg.ID {
	t.Helper()
	ids := map[string]dlg.ID{
		"shell": PatternShellRadio, "regex": PatternRegexRadio, "simple": PatternSimpleRadio,
		"pattern": PatternInput, "filesonly": PatternFilesOnly, "dirsonly": PatternDirsOnly, "invert": PatternInvert,
		"case": PatternCase, "fullpath": PatternFullPath, "includemeta": PatternIncludeMeta, "onlymeta": PatternOnlyMeta,
		"ok": dlg.OK, "cancel": dlg.Cancel,
	}
	if id, ok := ids[name]; ok {
		return id
	}
	for i, l := range FilterTypeLabels {
		if strings.EqualFold(l, name) {
			return PatternTypeID(i)
		}
	}
	t.Fatalf("unknown pattern dialog focus %q", name)
	return ""
}

func filterGoldenFocus(t *testing.T, st *FilterDialogState, name string) {
	t.Helper()
	st.Focus = patternGoldenFocus(t, name)
}

// filterGoldenState builds a state for a named scenario.
func filterGoldenState(t *testing.T, variant, focus string) FilterDialogState {
	t.Helper()
	st := FilterDialogState{Open: true, PatternMode: panel.GroupPatternShell, Target: FilterTargetPanel}
	typed := func(text string) {
		st.Text, st.TextCursor = text, len([]rune(text))
	}
	switch variant {
	case "empty":
	case "typed":
		typed("*.mp4")
		st.PreviewShow, st.PreviewFiles, st.PreviewFolders = true, 3, 2
	case "files-only-preview":
		typed("willow")
		st.PatternMode = panel.GroupPatternSimple
		st.PreviewShow, st.PreviewFiles = true, 7
	case "many":
		typed("*")
		st.PreviewShow, st.PreviewFiles, st.PreviewFolders = true, 12345, 678
	case "zero":
		typed("zzz")
		st.PreviewShow = true
	case "bad-regex":
		typed("(harbor")
		st.PatternMode = panel.GroupPatternRegex
	case "bad-shell":
		typed("[orchard")
	case "scrolled":
		typed(strings.Repeat("meadow-", 12))
		st.TextScroll = 30
		st.PreviewShow, st.PreviewFiles = true, 1
	case "options":
		typed("*.zip")
		st.Types[FilterTypeVideos], st.Types[FilterTypeArchives] = true, true
		st.Invert, st.CaseSensitive, st.FilesOnly = true, true, true
		st.PatternMode = panel.GroupPatternRegex
		st.PreviewShow, st.PreviewFiles, st.PreviewFolders = true, 9, 1
	case "dirs-only":
		typed("anchor")
		st.Types[FilterTypePictures], st.Types[FilterTypeDocuments] = true, true
		st.DirsOnly = true
		st.PreviewShow, st.PreviewFolders = true, 4
	case "meta":
		st.MetaColumnCount = 2
		st.IncludeMetaColumns = true
		typed("lantern")
		st.PreviewShow, st.PreviewFiles = true, 5
	case "meta-only":
		st.MetaColumnCount = 2
		st.IncludeMetaColumns, st.OnlyMetaColumns = true, true
		st.Types[FilterTypeAudio] = true
	case "compare":
		st.Target = FilterTargetCompare
		typed("*.txt")
		st.PreviewShow, st.PreviewFiles = true, 6
	default:
		t.Fatalf("unknown filter variant %q", variant)
	}
	filterGoldenFocus(t, &st, focus)
	return st
}

func TestFilterGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, variant, focus string) {
		st := filterGoldenState(t, variant, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s_focus-%s", w, h, variant, focus), w, h, func(s tcell.Screen, l Layout) {
			DrawFilterDialog(s, l, st, asciiDialogStyles())
		}})
	}
	for _, pos := range filterGoldenFocusNames {
		switch pos {
		case "includemeta", "onlymeta":
			continue
		}
		add(80, 24, "empty", pos)
	}
	for _, pos := range filterGoldenFocusNames {
		add(80, 25, "meta", pos)
	}
	for _, v := range []string{"typed", "many", "zero", "bad-regex", "bad-shell", "scrolled", "options", "dirs-only", "files-only-preview"} {
		for _, pos := range []string{"pattern", "shell", "ok"} {
			add(80, 24, v, pos)
		}
	}
	add(80, 24, "options", "invert")
	add(80, 24, "options", "videos")
	add(80, 24, "dirs-only", "videos")
	add(80, 24, "dirs-only", "dirsonly")
	add(80, 24, "dirs-only", "filesonly")
	add(80, 25, "meta-only", "onlymeta")
	add(80, 25, "meta-only", "audio")
	for _, pos := range []string{"pattern", "shell", "invert", "case", "ok"} {
		add(80, 25, "compare", pos)
	}
	add(34, 24, "typed", "pattern")
	add(34, 24, "options", "documents")
	add(34, 25, "meta", "ok")
	add(70, 24, "scrolled", "pattern")
	add(80, 12, "typed", "pattern")
	runSimpleGoldens(t, "filter", cases)
}
