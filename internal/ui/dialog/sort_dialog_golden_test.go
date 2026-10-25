package dialog

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

var updateGolden = flag.Bool("update", false, "rewrite sort dialog golden files")

// sortGoldenDir holds the sort dialog render goldens: every rune and style of the dialog rect.
const sortGoldenDir = "testdata/sort_dialog"

// sortPos names a focus position independently of how SortDialogState encodes focus.
type sortPos struct {
	kind string // "radio", "meta", "cb", "ok", "cancel"
	idx  int
}

func (p sortPos) String() string {
	switch p.kind {
	case "ok", "cancel":
		return p.kind
	}
	return fmt.Sprintf("%s%d", p.kind, p.idx)
}

func sortMetaRadios(n int) []SortDialogMetaRadio {
	all := []SortDialogMetaRadio{
		{Title: "duration", Name: "duration"},
		{Title: "bitrate", Name: "bitrate"},
		{Title: "codec", Name: "codec"},
		{Title: "resolution", Name: "resolution"},
	}
	return all[:n]
}

// sortGoldenState builds the dialog state for a scenario: metas==0 selects the Name radio,
// otherwise the third meta column ("codec") is the selected sort.
func sortGoldenState(metas int, checked bool, pos sortPos) SortDialogState {
	st := SortDialogState{
		Open:                  true,
		SortMode:              panel.SortName,
		MetaRadios:            sortMetaRadios(metas),
		SortReverse:           checked,
		DirectoriesFirst:      checked,
		DiskUsageIdleSizeSort: checked,
	}
	if metas > 0 {
		st.SortMode = panel.SortMeta
		st.MetaColumn = "codec"
	}
	setSortGoldenFocus(&st, pos)
	return st
}

func renderSortGolden(t *testing.T, w, h int, st SortDialogState) string {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(w, h)
	layout := Layout{Width: w, Height: h}
	DrawSortDialog(screen, layout, st, asciiDialogStyles())
	screen.Show()
	rect, ok := draw.ClampCenteredDialogRect(layout, 56, 11, 30, 11)
	if !ok {
		rect = draw.Rect{Width: w, Height: h}
	}
	return tcelltest.DumpRect(screen, rect.X, rect.Y, rect.Width, rect.Height)
}

func checkSortGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(sortGoldenDir, name+".golden")
	if *updateGolden {
		if err := os.MkdirAll(sortGoldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden %s\n--- got ---\n%s\n--- want ---\n%s", name, path, got, want)
	}
}

func TestSortDialogGoldenRender(t *testing.T) {
	var cases []struct {
		name    string
		w, h    int
		metas   int
		checked bool
		pos     sortPos
	}
	add := func(w, h, metas int, checked bool, pos sortPos) {
		on := "off"
		if checked {
			on = "on"
		}
		cases = append(cases, struct {
			name    string
			w, h    int
			metas   int
			checked bool
			pos     sortPos
		}{fmt.Sprintf("%dx%d_meta%d_cb-%s_focus-%s", w, h, metas, on, pos), w, h, metas, checked, pos})
	}
	for _, metas := range []int{0, 4} {
		positions := []sortPos{{"radio", 0}, {"ok", 0}}
		if metas > 0 {
			positions = append(positions, sortPos{"meta", 0})
		}
		for _, checked := range []bool{false, true} {
			for _, pos := range positions {
				add(80, 24, metas, checked, pos)
			}
		}
	}
	// Clamped width (dialog narrower than its preferred 56), then too small to draw at all.
	add(40, 24, 0, false, sortPos{"radio", 0})
	add(40, 24, 0, true, sortPos{"cb", 1})
	add(40, 24, 4, false, sortPos{"meta", 1})
	add(40, 24, 4, true, sortPos{"cancel", 0})
	add(30, 24, 0, false, sortPos{"radio", 0})

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkSortGolden(t, c.name, renderSortGolden(t, c.w, c.h, sortGoldenState(c.metas, c.checked, c.pos)))
		})
	}
}
