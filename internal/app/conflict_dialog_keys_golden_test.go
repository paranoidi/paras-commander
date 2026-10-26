package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the quick job-blocker dialog (File exists, directory mismatch, Disk
// space and the Conflict rules screen), driven through App.handleKey with the harness of
// simple_dialog_keys_golden_test.go. A real job blocks in a fake transfer function; the answer the
// dialog submits is read back from that function once the dialog closes. A closed dialog that
// submitted nothing (Postpone, Esc) is told apart by a probe answer sent afterwards: the blocked
// job receives the probe instead of a dialog answer.

var conflictKeyAnswers sync.Map // *App -> chan jobs.BlockerAnswer

type conflictKeyVariant struct {
	name    string
	request jobs.BlockerRequest
	// panel opens the Conflict rules screen directly, as the jobs-view panel does.
	panel bool
}

func conflictKeyRequest(mod func(*jobs.ConflictRequest)) jobs.BlockerRequest {
	c := &jobs.ConflictRequest{
		Source: "/home/kestrel/meadow/orchard.dat", Destination: "/srv/willow/amber/orchard.dat",
		SourceSize: "1.5 MiB", SourceTime: "2024-03-01 10:15", DestSize: "900 KiB", DestTime: "2023-11-20 08:00",
	}
	if mod != nil {
		mod(c)
	}
	return jobs.BlockerRequest{Kind: jobs.BlockerKindConflict, Conflict: c}
}

// conflictFocusName names the focused widget of the dialog.
func conflictFocusName(st dialog.ConflictDialogState) string {
	s := st.Screen("")
	id := dlg.Focused(s.Spec, *s.Focus)
	if row, col, ok := dialog.ConflictRuleCell(id); ok {
		return fmt.Sprintf("r%dc%d", row, col)
	}
	switch id {
	case dialog.ConflictCancelJob, dlg.Cancel:
		return "cancel"
	case dlg.OK:
		return "ok"
	case dialog.ConflictApplyAll:
		return "all"
	case dialog.ConflictVerify:
		return "verify"
	}
	return string(id)
}

func conflictRulesText(r jobs.ConflictRules) string {
	names := [...]string{"ask", "overwrite", "skip", "keep", "delete-source", "rules"}
	acts := func(a [3]jobs.RuleAction) string {
		return names[a[0]] + "," + names[a[1]] + "," + names[a[2]]
	}
	return fmt.Sprintf("time[%s] size[%s] verify=%v same=%s differ=%s", acts(r.Time), acts(r.Size), r.Verify, names[r.Same], names[r.Differ])
}

func conflictKeyState(app *App) string {
	st := app.model.ConflictDialog
	if st.Advanced {
		return fmt.Sprintf("advanced focus=%s %s all=%v panel=%v", conflictFocusName(st), conflictRulesText(st.Rules), st.AdvAll, st.OpenedFromPanel)
	}
	return "focus=" + conflictFocusName(st)
}

func conflictKeyOutcome(app *App, quit bool) string {
	ch, _ := conflictKeyAnswers.Load(app)
	answers := ch.(chan jobs.BlockerAnswer)
	// Probe: if the dialog submitted nothing the blocked job takes this instead.
	app.jobState.SubmitBlockerAnswer("conflict-job", jobs.BlockerAnswer{Decision: "probe"})
	var a jobs.BlockerAnswer
	select {
	case a = <-answers:
	case <-time.After(5 * time.Second):
		return "no answer received"
	}
	out := "submitted nothing (postponed or closed)"
	if a.Decision != "probe" {
		out = "decision=" + string(a.Decision)
		if a.Rules != nil {
			out += " " + conflictRulesText(*a.Rules)
		}
	}
	return fmt.Sprintf("%s quit=%v", out, quit)
}

func conflictKeyDialog(v conflictKeyVariant) keyGoldenDialog {
	return keyGoldenDialog{
		name: "conflict_" + v.name,
		setup: func(t *testing.T, dir string) *App {
			app := newApp(t, newScreen(t, 80, 24), dir)
			app.model.Message = ""
			answers := make(chan jobs.BlockerAnswer, 4)
			conflictKeyAnswers.Store(app, answers)
			t.Cleanup(func() { conflictKeyAnswers.Delete(app) })
			app.jobState.SetTransferFunc(func(_ context.Context, _ *jobs.Job, _ func(jobs.Event), waitBlocker func(jobs.BlockerRequest) jobs.BlockerAnswer) error {
				answers <- waitBlocker(v.request)
				return nil
			})
			app.jobState.AddJob(&jobs.Job{
				ID: "conflict-job", Type: jobs.TypeCopy, Status: jobs.StatusQueued,
				Sources: pathloc.PathsForTest("/src/orchard"), Destination: pathloc.MustParse("/dst"),
			})
			// Registered after newApp's flush cleanup, so it runs first: a job still blocked on a
			// postponed prompt would otherwise stall the flush.
			t.Cleanup(func() {
				app.jobState.SubmitBlockerAnswer("conflict-job", jobs.BlockerAnswer{Decision: jobs.DecisionCancel})
			})
			waitJobsWaitingDecision(t, app, 1)
			if !app.jobsCtrl.HandleAnswerBlockerKey() {
				t.Fatal("blocker dialog did not open")
			}
			if v.panel {
				app.model.ConflictDialog.OpenedFromPanel = true
				app.model.ConflictDialog.StartAdvanced()
			}
			return app
		},
		state:   conflictKeyState,
		open:    func(app *App) bool { return app.model.ConflictDialog.Open },
		outcome: conflictKeyOutcome,
	}
}

func prefixed(prefix string, scripts []simpleKeyScript) []simpleKeyScript {
	out := make([]simpleKeyScript, len(scripts))
	for i, s := range scripts {
		out[i] = simpleKeyScript{s.name, strings.TrimSpace(prefix + " " + s.keys)}
	}
	return out
}

func TestConflictFileKeyScripts(t *testing.T) {
	scripts := []simpleKeyScript{
		{"enter-overwrite", "Enter"},
		{"enter-overwrite-all", "Right Enter"},
		{"enter-advanced", "Right Right Enter"},
		{"enter-skip", "Down Enter"},
		{"enter-skip-all", "Down Right Enter"},
		{"enter-postpone", "Down Right Right Enter"},
		{"enter-cancel", "Down Down Enter"},
		{"space", "Space"},
		{"space-skip", "Down Space"},
		{"space-advanced", "Right Right Space"},
		{"alt-o", "Alt+o"},
		{"alt-upper-o", "Alt+O"},
		{"alt-a", "Alt+a"},
		{"alt-d", "Alt+d"},
		{"alt-s", "Alt+s"},
		{"alt-k", "Alt+k"},
		{"alt-c", "Alt+c"},
		{"alt-p", "Alt+p"},
		{"alt-r-absent", "Alt+r Right"},
		{"alt-b-absent", "Alt+b Alt+x Down"},
		{"esc", "Down Esc"},
		{"f9", "Right F9"},
		{"alt-enter", "Down Right Alt+Enter"},
		{"plain-letters", "o a d s k c p r x"},
		{"advanced-then-esc", "Alt+d Esc"},
		{"advanced-then-alt-enter", "Alt+d Alt+Enter"},
	}
	runKeyGoldenScripts(t, conflictKeyDialog(conflictKeyVariant{name: "file", request: conflictKeyRequest(nil)}), scripts)
}

func TestConflictMismatchKeyScripts(t *testing.T) {
	scripts := []simpleKeyScript{
		{"enter-rename", "Enter"},
		{"enter-rename-all", "Right Enter"},
		{"enter-skip", "Down Enter"},
		{"enter-skip-all", "Down Right Enter"},
		{"enter-postpone", "Down Right Right Enter"},
		{"enter-cancel", "Down Down Enter"},
		{"space", "Space"},
		{"alt-r", "Alt+r"},
		{"alt-a", "Alt+a"},
		{"alt-o-absent", "Alt+o Right"},
		{"alt-d-absent", "Alt+d Right"},
		{"alt-s", "Alt+s"},
		{"alt-k", "Alt+k"},
		{"alt-c", "Alt+c"},
		{"alt-p", "Alt+p"},
		{"esc", "Esc"},
		{"alt-enter", "Down Alt+Enter"},
	}
	dirDest := conflictKeyRequest(func(c *jobs.ConflictRequest) { c.Mismatch, c.DestIsDir = true, true })
	fileDest := conflictKeyRequest(func(c *jobs.ConflictRequest) { c.Mismatch = true })
	runKeyGoldenScripts(t, conflictKeyDialog(conflictKeyVariant{name: "mismatch_dir", request: dirDest}), scripts)
	runKeyGoldenScripts(t, conflictKeyDialog(conflictKeyVariant{name: "mismatch_file", request: fileDest}), scripts[:6])
}

func TestConflictDiskKeyScripts(t *testing.T) {
	scripts := []simpleKeyScript{
		{"enter-retry", "Enter"},
		{"enter-abort", "Right Enter"},
		{"enter-postpone", "Right Right Enter"},
		{"space", "Space"},
		{"alt-r", "Alt+r"},
		{"alt-b", "Alt+b"},
		{"alt-p", "Alt+p"},
		{"alt-a-absent", "Alt+a Alt+o Alt+c Alt+s Right"},
		{"esc", "Right Esc"},
		{"alt-enter", "Right Alt+Enter"},
		{"plain-letters", "r b p x"},
	}
	req := jobs.BlockerRequest{Kind: jobs.BlockerKindDiskSpace, DiskSpace: &jobs.DiskSpaceBlockerRequest{
		Destination: "/srv/willow/amber", RequiredBytes: 5 << 20, AvailableBytes: 900 << 10, AvailableKnown: true,
		NextSource: "/home/kestrel/meadow/orchard.dat",
	}}
	runKeyGoldenScripts(t, conflictKeyDialog(conflictKeyVariant{name: "disk", request: req}), scripts)
}

func conflictAdvScripts() []simpleKeyScript {
	return []simpleKeyScript{
		{"space-select", "Right Space Right Space Right Space"},
		{"enter-select", "Right Right Enter Left Enter"},
		{"greyed-skip", "Right Right Space Down Down Down Right Right Right Right Space"},
		{"greyed-column-fallback", "Right Right Space Down Down Right Down"},
		{"greyed-fallback-up", "Right Right Space Down Down Down Down Right Right Right Up"},
		{"normalize-size", "Down Down Down Right Right Space Up Up Up Right Right Right Space Down Down Down"},
		{"checkbox-space", "Down Down Down Down Down Down Space Down Space Space"},
		{"checkbox-enter-applies", "Down Down Down Down Down Down Enter"},
		{"all-checkbox-enter-applies", "Down Down Down Down Down Down Down Enter"},
		{"enter-ok", "Right Space Down Down Down Down Down Down Down Down Enter"},
		{"enter-cancel", "Down Down Down Down Down Down Down Down Right Enter"},
		{"space-ok", "Down Down Down Down Down Down Down Down Space"},
		{"alt-a", "Alt+a Down Alt+a"},
		{"alt-v", "Alt+v Alt+v Right"},
		{"verify-differ-greys-matrix", "Alt+v Down Down Right Right Right Right Down Space Up Up Up"},
		{"verify-differ-rules-apply", "Alt+v Down Right Right Space Down Left Left Space Alt+Enter"},
		{"alt-o", "Right Space Alt+o"},
		{"alt-c", "Alt+c"},
		{"esc", "Right Space Esc"},
		{"f9", "F9"},
		{"alt-enter", "Right Space Alt+Enter"},
		{"alt-x", "Alt+x Alt+r Alt+s Alt+d Alt+p Down"},
		{"plain-letters", "a v o c x Right"},
	}
}

func TestConflictAdvancedKeyScripts(t *testing.T) {
	req := conflictKeyRequest(nil)
	runKeyGoldenScripts(t, conflictKeyDialog(conflictKeyVariant{name: "adv_from_dialog", request: req}), prefixed("Alt+d", conflictAdvScripts()))
	runKeyGoldenScripts(t, conflictKeyDialog(conflictKeyVariant{name: "adv_from_panel", request: req, panel: true}), conflictAdvScripts())
	noCompare := conflictKeyRequest(func(c *jobs.ConflictRequest) { c.NoCompare = true })
	runKeyGoldenScripts(t, conflictKeyDialog(conflictKeyVariant{name: "adv_nocompare", request: noCompare, panel: true}), conflictAdvScripts()[:len(conflictAdvScripts())-1]) // all but plain-letters
	move := conflictKeyRequest(func(c *jobs.ConflictRequest) { c.Move = true })
	runKeyGoldenScripts(t, conflictKeyDialog(conflictKeyVariant{name: "adv_move", request: move, panel: true}), []simpleKeyScript{
		{"verify-same-delete-source", "Alt+v Down Right Right Right Space Alt+Enter"},
		{"verify-differ-delete-source", "Alt+v Down Down Down Right Space Down Down Enter"},
	})
}

func TestConflictMoveKeyScripts(t *testing.T) {
	scripts := []simpleKeyScript{
		{"enter-delete-source", "Down Down Enter"},
		{"enter-cancel", "Down Down Right Enter"},
		{"alt-e", "Alt+e"},
		{"alt-upper-e", "Alt+E"},
	}
	move := conflictKeyRequest(func(c *jobs.ConflictRequest) { c.Move = true })
	runKeyGoldenScripts(t, conflictKeyDialog(conflictKeyVariant{name: "move", request: move}), scripts)
	moveMismatch := conflictKeyRequest(func(c *jobs.ConflictRequest) { c.Move, c.Mismatch, c.DestIsDir = true, true, true })
	runKeyGoldenScripts(t, conflictKeyDialog(conflictKeyVariant{name: "move_mismatch", request: moveMismatch}), []simpleKeyScript{
		{"alt-e-absent", "Alt+e Down Down"},
	})
}
