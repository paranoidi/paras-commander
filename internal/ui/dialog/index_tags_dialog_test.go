package dialog

import (
	"slices"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func indexTagsState(names ...string) IndexTagsDialogState {
	return NewIndexTagsDialog([]string{"/orchard"}, names, nil, 1)
}

func TestIndexTagsInitialStates(t *testing.T) {
	st := NewIndexTagsDialog([]string{"/a", "/b"}, []string{"amber", "birch", "cedar"}, map[string]int{"amber": 2, "birch": 1}, 2)
	got := []dlg.CheckState{st.Tags[0].State, st.Tags[1].State, st.Tags[2].State}
	if !slices.Equal(got, []dlg.CheckState{dlg.CheckOn, dlg.CheckMixed, dlg.CheckOff}) {
		t.Fatalf("states = %v", got)
	}
}

func TestIndexTagsToggleCycles(t *testing.T) {
	st := NewIndexTagsDialog([]string{"/a", "/b"}, []string{"amber", "birch"}, map[string]int{"amber": 1}, 2)
	var got []dlg.CheckState
	for range 3 {
		st.ToggleCell(0, 0)
		got = append(got, st.Tags[0].State)
	}
	if !slices.Equal(got, []dlg.CheckState{dlg.CheckOn, dlg.CheckOff, dlg.CheckMixed}) {
		t.Fatalf("mixed cycle = %v", got)
	}
	st.ToggleCell(0, 1)
	if st.Tags[1].State != dlg.CheckOn {
		t.Fatalf("state = %v", st.Tags[1].State)
	}
	st.ToggleCell(0, 1)
	if st.Tags[1].State != dlg.CheckOff {
		t.Fatalf("state = %v", st.Tags[1].State)
	}
}

func TestIndexTagsEdits(t *testing.T) {
	st := NewIndexTagsDialog([]string{"/a", "/b"}, []string{"amber", "birch", "cedar", "dune"}, map[string]int{"amber": 2, "birch": 1, "dune": 2}, 2)
	st.ToggleCell(0, 0)           // amber off
	st.ToggleCell(0, 1)           // birch mixed -> on
	st.ToggleCell(1, 1)           // dune off ...
	st.ToggleCell(1, 1)           // ... and back: no edit
	st.AddTags([]string{"azure"}) // new
	st.AddTags([]string{"cedar"}) // existing
	add, remove := st.Edits()
	if !slices.Equal(add, []string{"azure", "birch", "cedar"}) || !slices.Equal(remove, []string{"amber"}) {
		t.Fatalf("add %v remove %v", add, remove)
	}
}

func TestIndexTagsAddKeepsOrderAndMovesCursor(t *testing.T) {
	st := indexTagsState("amber", "cedar", "dune")
	st.AddTags([]string{"birch", "ember"})
	names := []string{}
	for _, tag := range st.Tags {
		names = append(names, tag.Name)
	}
	if !slices.Equal(names, []string{"amber", "birch", "cedar", "dune", "ember"}) {
		t.Fatalf("names = %v", names)
	}
	if st.Cursor != 2 || st.Col != 0 {
		t.Fatalf("cursor = %d/%d, want ember's cell", st.Cursor, st.Col)
	}
}

func TestIndexTagsJump(t *testing.T) {
	st := indexTagsState("coral", "birch", "bronze", "cedar")
	for _, r := range "br" {
		st.TypeJump(r)
	}
	if st.JumpQuery != "br" || st.jumpLabel() != "br 1/2" {
		t.Fatalf("query %q label %q", st.JumpQuery, st.jumpLabel())
	}
	st.TypeJump('z') // birch drops out, bronze stays
	if st.cursorTag() != 2 || st.jumpLabel() != "brz" {
		t.Fatalf("tag %d label %q", st.cursorTag(), st.jumpLabel())
	}
	st.TypeJump('q') // no match: ignored
	if st.JumpQuery != "brz" {
		t.Fatalf("query = %q", st.JumpQuery)
	}
	st.BackspaceJump()
	first := st.cursorTag()
	st.CycleJump(1)
	second := st.cursorTag()
	st.CycleJump(1)
	if first == second || st.cursorTag() != first {
		t.Fatalf("cycle: %d, %d, %d", first, second, st.cursorTag())
	}
	st.BackspaceJump()
	st.BackspaceJump()
	if st.JumpQuery != "" {
		t.Fatalf("query = %q", st.JumpQuery)
	}
}

func renderIndexTags(t *testing.T, st IndexTagsDialogState, w, h int) string {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(w, h)
	DrawIndexTagsDialog(screen, Layout{Width: w, Height: h}, st, theme.Default())
	screen.Show()
	return tcelltest.DumpRect(screen, 0, 0, w, h)
}

func TestIndexTagsJumpTextInSeparator(t *testing.T) {
	st := indexTagsState("coral", "birch")
	st.Focus = IndexTagsGrid
	st.TypeJump('b')
	if got := renderIndexTags(t, st, 80, 24); !strings.Contains(got, "─ b ─") {
		t.Fatalf("separator has no padded jump text:\n%s", got)
	}
}

func TestIndexTagsGridComesFirst(t *testing.T) {
	st := indexTagsState("coral", "birch")
	if st.Focus != IndexTagsGrid {
		t.Fatalf("focus = %q, want the grid", st.Focus)
	}
	got := renderIndexTags(t, st, 80, 24)
	if strings.Index(got, "coral") > strings.Index(got, "New tags") {
		t.Fatalf("the grid should sit above the new-tags input:\n%s", got)
	}
}

func TestIndexTagsEmptyDatabase(t *testing.T) {
	st := indexTagsState()
	if st.Focus != IndexTagsInput {
		t.Fatalf("focus = %q, want the input", st.Focus)
	}
	spec := IndexTagsDialogView(&st, Layout{Width: 80, Height: 24}, theme.Default())
	if _, ok := spec.Item(IndexTagsGrid); ok {
		t.Fatal("no grid expected without tags")
	}
	if got := renderIndexTags(t, st, 80, 24); !strings.Contains(got, "No tags yet") {
		t.Fatalf("missing the empty note:\n%s", got)
	}
}

func TestIndexTagsGridScrollsOnShortScreen(t *testing.T) {
	var names []string
	for _, a := range "abcdefgh" {
		for _, b := range "abcd" {
			names = append(names, string(a)+string(b))
		}
	}
	st := indexTagsState(names...)
	layout := Layout{Width: 80, Height: 20}
	spec := IndexTagsDialogView(&st, layout, theme.Default())
	grid, ok := spec.Item(IndexTagsGrid)
	if !ok || grid.Height >= len(names)/2 || grid.Height < indexTagsMinListRows {
		t.Fatalf("grid height = %d, ok %v", grid.Height, ok)
	}
}

func TestIndexTagsGoldenRender(t *testing.T) {
	names := []string{"amber", "birch", "bronze", "cedar", "dune"}
	counts := map[string]int{"amber": 2, "bronze": 1}
	state := func() IndexTagsDialogState { return NewIndexTagsDialog([]string{"/a", "/b"}, names, counts, 2) }
	var many []string
	for _, a := range "abcdefgh" {
		for _, b := range "abcd" {
			many = append(many, "tag-"+string(a)+string(b))
		}
	}
	add := func(cases *[]simpleGolden, name string, w, h int, st IndexTagsDialogState) {
		*cases = append(*cases, simpleGolden{name, w, h, func(s tcell.Screen, l Layout) {
			DrawIndexTagsDialog(s, l, st, asciiDialogStyles())
		}})
	}
	var cases []simpleGolden
	add(&cases, "80x24_default", 80, 24, state())
	grid := state()
	grid.Focus, grid.Cursor, grid.Col = IndexTagsGrid, 1, 1
	add(&cases, "80x24_grid", 80, 24, grid)
	input := state()
	input.Focus, input.Input, input.InputCursor = IndexTagsInput, "clover", 6
	add(&cases, "80x24_input", 80, 24, input)
	jump := state()
	jump.Focus = IndexTagsGrid
	jump.TypeJump('b')
	jump.TypeJump('r')
	add(&cases, "80x24_jump", 80, 24, jump)
	overflow := NewIndexTagsDialog([]string{"/a"}, many, nil, 1)
	overflow.Focus, overflow.Cursor = IndexTagsGrid, 5
	add(&cases, "80x20_overflow", 80, 20, overflow)
	add(&cases, "80x24_empty", 80, 24, NewIndexTagsDialog([]string{"/a"}, nil, nil, 1))
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkGoldenFile(t, simpleGoldenRoot+"/index_tags", c.name, dumpDialogBand(t, c, c.h))
		})
	}
}
