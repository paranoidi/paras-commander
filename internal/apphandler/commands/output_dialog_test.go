package commands

import (
	"errors"
	"slices"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestOutputDialogResultKeepsStderr(t *testing.T) {
	res := cmdrun.RunResult{Stdout: []byte("out\n"), Stderr: []byte("err\n"), ExitCode: 2}
	st := OutputDialogResult("id", "Show", res, "", "")
	if st.Title != "Show (exit 2)" || st.Running || !st.Open || st.RunID != "id" {
		t.Fatalf("state: %+v", st)
	}
	if want := []string{"out", "--- stderr ---", "err"}; !slices.Equal(st.Lines, want) {
		t.Fatalf("Lines = %#v, want %#v", st.Lines, want)
	}
	st = OutputDialogResult("id", "Show", cmdrun.RunResult{LaunchErr: errors.New("boom")}, "", "")
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
	res := OutputDialogResult("row1", "T", cmdrun.RunResult{Stdout: []byte("x")}, "", "")
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
	res := OutputDialogResult("row1", "T", cmdrun.RunResult{}, "", "")
	h.ApplyWake(WakePayload{OpenOutputDialog: &res, NotifyLog: "l", NotifyBanner: "b"})
	if h.model.CommandOutputDialog.Open || bh.banners != 1 || *canceled {
		t.Fatalf("open=%v banners=%d canceled=%v", h.model.CommandOutputDialog.Open, bh.banners, *canceled)
	}

	h, bh, _ = runningHandler(t)
	other := OutputDialogResult("other", "T", cmdrun.RunResult{}, "", "")
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
