package commands

import (
	"errors"
	"slices"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestOutputDialogResultKeepsStderr(t *testing.T) {
	res := cmdrun.RunResult{Stdout: []byte("out\n"), Stderr: []byte("err\n"), ExitCode: 2}
	st := OutputDialogResult("id", "Show", res, "", "", "")
	if st.Title != "Show (exit 2)" || st.Running || !st.Open || st.RunID != "id" {
		t.Fatalf("state: %+v", st)
	}
	if want := []string{"out", "--- stderr ---", "err"}; !slices.Equal(st.Lines, want) {
		t.Fatalf("Lines = %#v, want %#v", st.Lines, want)
	}
	st = OutputDialogResult("id", "Show", cmdrun.RunResult{LaunchErr: errors.New("boom")}, "", "", "")
	if want := []string{"--- stderr ---", "boom"}; !slices.Equal(st.Lines, want) {
		t.Fatalf("launch err Lines = %#v", st.Lines)
	}
}

type bannerHost struct {
	Host
	banners int
	pnl     *panel.State
}

func (b *bannerHost) SetTransientMessageBanner(string, string, ui.MessageUrgency) { b.banners++ }
func (b *bannerHost) RefreshAfterBackgroundCommand()                              {}
func (b *bannerHost) ActivePanel() *panel.State                                   { return b.pnl }

func runningHandler(t *testing.T) (*Handler, *bannerHost, *bool) {
	h, _ := newOutputDialogHandler(t)
	st, err := panel.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bh := &bannerHost{pnl: &st}
	h.host = bh
	canceled := new(bool)
	h.OpenRunningOutputDialog("row1", " T ", true, func() { *canceled = true }, "", "")
	return h, bh, canceled
}

func TestApplyWakeStartedClearsQueued(t *testing.T) {
	h, _, _ := runningHandler(t)
	h.ApplyWake(WakePayload{OutputDialogStartedRunID: "other"})
	if !h.model.CommandOutputDialog.Queued {
		t.Fatal("non-matching start cleared Queued")
	}
	h.ApplyWake(WakePayload{OutputDialogStartedRunID: "row1"})
	if st := h.model.CommandOutputDialog; st.Queued || !st.Running {
		t.Fatalf("after start: %+v", st)
	}
}

func TestApplyWakeFillsMatchingDialogAndSuppressesBanner(t *testing.T) {
	h, bh, canceled := runningHandler(t)
	res := OutputDialogResult("row1", "T", cmdrun.RunResult{Stdout: []byte("x")}, "", "", "")
	h.ApplyWake(WakePayload{OpenOutputDialog: &res, NotifyLog: "l", NotifyBanner: "b", RefreshBrowserPanel: true})
	if got := h.model.CommandOutputDialog; got.Running || !slices.Equal(got.Lines, []string{"x"}) {
		t.Fatalf("after fill: %+v", got)
	}
	if bh.banners != 0 || !*canceled {
		t.Fatalf("banners=%d canceled=%v", bh.banners, *canceled)
	}
}

func TestApplyWakeBackgroundedAppliesBanner(t *testing.T) {
	h, bh, canceled := runningHandler(t)
	h.HandleOutputDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, 0))
	res := OutputDialogResult("row1", "T", cmdrun.RunResult{}, "", "", "")
	h.ApplyWake(WakePayload{OpenOutputDialog: &res, NotifyLog: "l", NotifyBanner: "b"})
	if h.model.CommandOutputDialog.Open || bh.banners != 1 || *canceled {
		t.Fatalf("open=%v banners=%d canceled=%v", h.model.CommandOutputDialog.Open, bh.banners, *canceled)
	}

	h, bh, _ = runningHandler(t)
	other := OutputDialogResult("other", "T", cmdrun.RunResult{}, "", "", "")
	h.ApplyWake(WakePayload{OpenOutputDialog: &other, NotifyLog: "l", NotifyBanner: "b"})
	if !h.model.CommandOutputDialog.Running || bh.banners != 1 {
		t.Fatalf("non-matching: %+v banners=%d", h.model.CommandOutputDialog, bh.banners)
	}
}

func TestOutputDialogRunningKeys(t *testing.T) {
	alt := func(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModAlt) }
	key := func(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, 0) }
	cases := []struct {
		name       string
		keys       []*tcell.EventKey
		wantCancel bool
	}{
		{"esc", []*tcell.EventKey{key(tcell.KeyEsc)}, false},
		{"alt+b", []*tcell.EventKey{alt('b')}, false},
		{"enter default", []*tcell.EventKey{key(tcell.KeyEnter)}, false},
		{"alt+c", []*tcell.EventKey{alt('c')}, true},
		{"right enter", []*tcell.EventKey{key(tcell.KeyRight), key(tcell.KeyEnter)}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _, canceled := runningHandler(t)
			for _, k := range tc.keys {
				h.HandleOutputDialogKey(k)
			}
			if h.model.CommandOutputDialog.Open {
				t.Fatal("dialog still open")
			}
			if *canceled != tc.wantCancel {
				t.Fatalf("canceled = %v, want %v", *canceled, tc.wantCancel)
			}
		})
	}
}

func newOutputDialogHandler(t *testing.T) (*Handler, tcell.SimulationScreen) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	return &Handler{
		screen: screen,
		model:  &ui.Model{},
	}, screen
}

type pathKeysHost struct {
	layoutHost
	pnl         *panel.State
	msgs        []string
	groupSelect []bool
}

func (p *pathKeysHost) ActivePanel() *panel.State { return p.pnl }

func (p *pathKeysHost) SetTransientMessage(msg string, _ ui.MessageUrgency) {
	p.msgs = append(p.msgs, msg)
}
func (p *pathKeysHost) OpenOutputGroupSelect(selectMode bool) {
	p.groupSelect = append(p.groupSelect, selectMode)
}

func pathDialogHandler(t *testing.T, lines []string, paths []dialog.OutputPathMatch) (*Handler, *pathKeysHost) {
	t.Helper()
	h, _ := newOutputDialogHandler(t)
	keys, err := keymap.Build(keymap.DefaultCommandOutputDialogOverlayKeys())
	if err != nil {
		t.Fatal(err)
	}
	h.keysCommandOutputDialog = keys
	if h.keys, err = keymap.Default(); err != nil {
		t.Fatal(err)
	}
	pnl, err := panel.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ph := &pathKeysHost{pnl: &pnl}
	h.host = ph
	h.model.CommandOutputDialog = dialog.CommandOutputDialogState{Open: true, Lines: lines, Paths: paths}
	return h, ph
}

func runeKey(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, 0) }

func TestOutputDialogPathKeys(t *testing.T) {
	lines := manyLines(60)
	paths := []dialog.OutputPathMatch{
		{Line: 2, Start: 0, End: 4, Path: "/x/amber"},
		{Line: 2, Start: 5, End: 9, Path: "/x/birch"},
		{Line: 40, Start: 0, End: 4, Path: "/x/cedar"},
	}
	h, ph := pathDialogHandler(t, lines, paths)
	st := &h.model.CommandOutputDialog
	h.HandleOutputDialogKey(runeKey('j'))
	if st.PathCursor != 2 {
		t.Fatalf("j: cursor=%d", st.PathCursor)
	}
	visH := h.outputDialogVisH(*st)
	if line := 40; st.Scroll > line || line >= st.Scroll+visH || st.Follow {
		t.Fatalf("j: scroll=%d visH=%d follow=%v", st.Scroll, visH, st.Follow)
	}
	h.HandleOutputDialogKey(runeKey('k'))
	if st.PathCursor != 0 || st.Scroll > 2 {
		t.Fatalf("k: cursor=%d scroll=%d", st.PathCursor, st.Scroll)
	}
	h.HandleOutputDialogKey(runeKey('l'))
	h.HandleOutputDialogKey(runeKey('l'))
	h.HandleOutputDialogKey(runeKey('l'))
	if st.PathCursor != 2 {
		t.Fatalf("l x3 (clamped): cursor=%d", st.PathCursor)
	}
	h.HandleOutputDialogKey(runeKey('h'))
	if st.PathCursor != 1 {
		t.Fatalf("h: cursor=%d", st.PathCursor)
	}

	h.HandleOutputDialogKey(runeKey(' '))
	if !ph.pnl.IsSelectedPath("/x/birch") || st.PathCursor != 2 {
		t.Fatalf("space: selected=%v cursor=%d", ph.pnl.IsSelectedPath("/x/birch"), st.PathCursor)
	}
	h.HandleOutputDialogKey(key(tcell.KeyLeft))
	h.HandleOutputDialogKey(key(tcell.KeyInsert))
	if ph.pnl.IsSelectedPath("/x/birch") {
		t.Fatal("insert did not toggle selection off")
	}
	if !st.Open {
		t.Fatal("dialog closed")
	}
}

func TestOutputDialogNoPathsStillScrolls(t *testing.T) {
	h, _ := pathDialogHandler(t, manyLines(60), nil)
	st := &h.model.CommandOutputDialog
	h.HandleOutputDialogKey(key(tcell.KeyDown))
	h.HandleOutputDialogKey(runeKey('j'))
	if st.Scroll != 2 {
		t.Fatalf("scroll = %d, want 2", st.Scroll)
	}
	h.HandleOutputDialogKey(key(tcell.KeyUp))
	if st.Scroll != 1 {
		t.Fatalf("scroll = %d, want 1", st.Scroll)
	}
}

func TestOutputDialogGroupSelectionKeys(t *testing.T) {
	paths := []dialog.OutputPathMatch{
		{Line: 0, Path: "/x/amber"}, {Line: 1, Path: "/x/birch"}, {Line: 2, Path: "/x/amber"},
	}
	h, ph := pathDialogHandler(t, manyLines(3), paths)
	ph.pnl.AddSelection("/x/amber")

	h.HandleOutputDialogKey(runeKey('*'))
	if ph.pnl.IsSelectedPath("/x/amber") || !ph.pnl.IsSelectedPath("/x/birch") {
		t.Fatal("invert should toggle each unique path once")
	}
	if len(ph.msgs) != 1 || ph.msgs[0] != "Inverted 2" {
		t.Fatalf("messages = %v", ph.msgs)
	}

	h.HandleOutputDialogKey(runeKey('+'))
	h.HandleOutputDialogKey(runeKey('-'))
	if want := []bool{true, false}; !slices.Equal(ph.groupSelect, want) {
		t.Fatalf("group select calls = %v, want %v", ph.groupSelect, want)
	}
}
