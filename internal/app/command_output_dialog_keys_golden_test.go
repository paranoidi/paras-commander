package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	commandsctrl "github.com/paranoidi/paras-commander/internal/apphandler/commands"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Key-script goldens for the command output dialog, driven through App.handleKey with the harness
// of simple_dialog_keys_golden_test.go. Async content is injected deterministically: "@live+N"
// streams N more lines into a running dialog, "@started" ends the queued wait and "@done" delivers
// the finished result. The cancel func counts how often Cancel killed the run. Path fixtures are
// created in the temp dir and shown relative to it, so no absolute temp path reaches a golden.

// cmdOutRect is the dialog on an 80x24 screen (64 wide, 14 tall, centered).
const cmdOutRectX, cmdOutRectY, cmdOutRectW, cmdOutRectH = 8, 5, 64, 14

var cmdOutCancels = map[*App]*int{}

func cmdOutLines(n int) []string {
	words := []string{"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor"}
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("line %02d %s %s", i, words[i%len(words)], words[(i*3+1)%len(words)])
	}
	return out
}

// cmdOutFocusName names the running-state button focus.
func cmdOutFocusName(st dialog.CommandOutputDialogState) string {
	if !st.Running {
		return "ok"
	}
	if st.Focus == dialog.OutputCancel {
		return "cancel"
	}
	return "background"
}

func cmdOutPathFiles(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"harbor.txt", "willow.txt", "lantern.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func cmdOutStdout(n int, withPaths bool) string {
	lines := cmdOutLines(n)
	if withPaths && n >= 12 {
		lines[2] = "built harbor.txt and willow.txt ok"
		lines[3] = "meadow"
		lines[9] = "see lantern.txt:12 here"
		lines[n-1] = "final harbor.txt"
	}
	return strings.Join(lines, "\n") + "\n"
}

// cmdOutDialog describes one scenario. mode: "running" or "queued" (streaming dialog opened with
// a few lines) or "done" (finished result).
func cmdOutDialog(name, mode string, lines int, withPaths bool, exit int) keyGoldenDialog {
	var workDir string // the temp dir of the script under test (scripts run one at a time)
	return keyGoldenDialog{
		name: name, rx: cmdOutRectX, ry: cmdOutRectY, rw: cmdOutRectW, rh: cmdOutRectH,
		serial: true, // setup writes workDir and the shared cmdOutCancels map
		setup: func(t *testing.T, dir string) *App {
			workDir = dir
			app := newApp(t, newScreen(t, 80, 24), dir)
			app.model.Message = ""
			cancels := new(int)
			cmdOutCancels[app] = cancels
			t.Cleanup(func() { delete(cmdOutCancels, app) })
			if withPaths {
				cmdOutPathFiles(t, dir)
			}
			switch mode {
			case "running", "queued":
				app.commandsCtrl.OpenRunningOutputDialog("row1", "Build willow", mode == "queued", func() { *cancels++ }, "", "")
				app.model.CommandOutputDialog.Lines = cmdOutLines(lines)
			default:
				res := cmdrun.RunResult{Stdout: []byte(cmdOutStdout(lines, withPaths)), ExitCode: exit}
				app.model.CommandOutputDialog = commandsctrl.OutputDialogResult("row1", "Build willow", res, "", "", dir)
			}
			return app
		},
		state: func(app *App) string {
			st := app.model.CommandOutputDialog
			sel := 0
			for _, m := range st.Paths {
				if app.activePanel().IsSelectedPath(m.Path) {
					sel++
				}
			}
			cursorLine := -1
			if len(st.Paths) > 0 {
				cursorLine = st.Paths[min(st.PathCursor, len(st.Paths)-1)].Line
			}
			return fmt.Sprintf("title=%q focus=%s running=%v queued=%v lines=%d scroll=%d follow=%v paths=%d cursor=%d(line %d) selected=%d cancels=%d groupselect=%v",
				st.Title, cmdOutFocusName(st), st.Running, st.Queued, len(st.Lines), st.Scroll, st.Follow,
				len(st.Paths), st.PathCursor, cursorLine, sel, *cmdOutCancels[app], app.model.GroupSelect.Open)
		},
		open: func(app *App) bool { return app.model.CommandOutputDialog.Open },
		outcome: func(app *App, quit bool) string {
			return fmt.Sprintf("quit=%v cancels=%d message=%q", quit, *cmdOutCancels[app], app.model.Message)
		},
		action: func(t *testing.T, app *App, tok string) {
			switch {
			case tok == "@started":
				app.commandsCtrl.ApplyWake(commandsctrl.WakePayload{OutputDialogStartedRunID: "row1"})
			case strings.HasPrefix(tok, "@live+"):
				n, err := strconv.Atoi(strings.TrimPrefix(tok, "@live+"))
				if err != nil {
					t.Fatalf("bad token %q", tok)
				}
				stdout := strings.Join(cmdOutLines(len(app.model.CommandOutputDialog.Lines)+n), "\n") + "\n"
				app.commandsCtrl.ApplyWake(commandsctrl.WakePayload{OutputDialogLive: &commandsctrl.OutputDialogLive{RunID: "row1", Stdout: stdout}})
			case tok == "@done":
				res := cmdrun.RunResult{Stdout: []byte(cmdOutStdout(len(app.model.CommandOutputDialog.Lines)+2, withPaths))}
				st := commandsctrl.OutputDialogResult("row1", "Build willow", res, "", "", workDir)
				app.commandsCtrl.ApplyWake(commandsctrl.WakePayload{OpenOutputDialog: &st})
			default:
				t.Fatalf("bad token %q", tok)
			}
		},
	}
}

func cmdOutButtonScripts() []simpleKeyScript {
	return []simpleKeyScript{
		{"left-right", "Left Right Right Left Left Right"},
		{"tab", "Tab Tab Tab Backtab Backtab"},
		{"enter-background", "Enter"},
		{"enter-cancel", "Right Enter"},
		{"space-background", "Space"},
		{"space-cancel", "Right Space"},
		{"alt-b", "Right Alt+b"},
		{"alt-upper-b", "Alt+B"},
		{"alt-c", "Alt+c"},
		{"alt-o", "Right Alt+o"},
		{"alt-enter", "Right Alt+Enter"},
		{"esc", "Esc"},
		{"esc-on-cancel", "Right Esc"},
		{"plain-letters", "b c o x y Enter"},
		{"alt-unknown", "Alt+x Alt+q Right Alt+z Enter"},
		{"function-keys", "F5 F8 Enter"},
		{"vi-keys-are-text", "j k h l Enter"},
		{"insert-delete", "Insert Delete Backspace Enter"},
	}
}

func cmdOutScrollScripts() []simpleKeyScript {
	return []simpleKeyScript{
		{"follow-down-noop", "Down Down"},
		{"up-unfollows", "Up"},
		{"up-down-edges", "Up Up Up Down Down Down Down"},
		{"up-to-top", "Home Up Up"},
		{"pgup-pgdn", "PgUp PgUp PgDn PgDn PgDn"},
		{"pgup-from-follow", "PgUp"},
		{"pgdn-at-bottom", "PgDn PgDn"},
		{"home-end", "Home End Home End"},
		{"home-then-down", "Home Down Down Down"},
		{"end-refollows", "Home Down End"},
		{"down-to-bottom-refollows", "Home PgDn PgDn PgDn PgDn PgDn"},
		{"scroll-then-tab-buttons", "Up Up Tab Right Down"},
		{"scroll-keeps-button-focus", "Right Up Down PgUp End"},
		{"ctrl-home-end", "Ctrl+Home Ctrl+End"},
		{"stream-follow", "@live+5 @live+5"},
		{"stream-while-scrolled-up", "Up Up @live+5 @live+5 End @live+5"},
		{"stream-while-at-top", "Home @live+3 End @live+3"},
		{"done-follow", "@done"},
		{"done-scrolled-up", "Up Up Up @done"},
		{"done-then-ok", "@done Enter"},
		{"done-then-keys", "@done Home Down PgDn Alt+o"},
		{"done-then-esc", "@done Esc"},
		{"done-then-cancel", "@done Alt+c"},
	}
}

func TestCommandOutputRunningKeyScripts(t *testing.T) {
	scripts := append(cmdOutButtonScripts(), cmdOutScrollScripts()...)
	runKeyGoldenScripts(t, cmdOutDialog("command_output_running", "running", 40, false, 0), scripts)
	runKeyGoldenScripts(t, cmdOutDialog("command_output_running_short", "running", 3, false, 0), []simpleKeyScript{
		{"scroll-noop", "Up Down PgUp PgDn Home End"},
		{"stream-grows-past-view", "@live+4 @live+4 @live+4"},
		{"stream-grows-then-up", "@live+12 Up Up @live+5"},
		{"buttons", "Right Left Tab Enter"},
	})
	runKeyGoldenScripts(t, cmdOutDialog("command_output_running_empty", "running", 0, false, 0), []simpleKeyScript{
		{"scroll-noop", "Up Down PgUp PgDn Home End"},
		{"first-lines-stream", "@live+3 @live+10"},
		{"cancel-empty", "Right Enter"},
	})
	runKeyGoldenScripts(t, cmdOutDialog("command_output_queued", "queued", 0, false, 0), []simpleKeyScript{
		{"started", "@started @live+3"},
		{"esc-while-queued", "Esc"},
		{"cancel-while-queued", "Alt+c"},
		{"right-enter", "Right Enter"},
	})
}

func cmdOutFinishedScripts() []simpleKeyScript {
	return []simpleKeyScript{
		{"enter", "Enter"},
		{"space", "Space"},
		{"alt-o", "Alt+o"},
		{"alt-c", "Alt+c"},
		{"alt-enter", "Alt+Enter"},
		{"esc", "Esc"},
		{"alt-b-ignored", "Alt+b Down"},
		{"left-right", "Left Right Left"},
		{"tab", "Tab Backtab Tab Enter"},
		{"plain-letters", "o c b x"},
		{"alt-unknown", "Alt+x Alt+q Down"},
		{"function-keys", "F5 F8 Down"},
		{"down-to-bottom", strings.Repeat("Down ", 34)},
		{"up-from-top", "Up Up"},
		{"pgdn-pgup", "PgDn PgDn PgDn PgDn PgUp PgUp"},
		{"pgup-at-top", "PgUp"},
		{"pgdn-at-bottom", "End PgDn Down"},
		{"home-end", "End Home End Home"},
		{"vi-motion", "j j j k k"},
		{"vi-pages", "j j End k Home"},
		{"scroll-then-enter", "PgDn Down Enter"},
		{"ctrl-home-end", "Ctrl+End Ctrl+Home"},
	}
}

func TestCommandOutputFinishedKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, cmdOutDialog("command_output_done", "done", 40, false, 0), cmdOutFinishedScripts())
	runKeyGoldenScripts(t, cmdOutDialog("command_output_done_exit", "done", 3, false, 3), []simpleKeyScript{
		{"open", "Down"},
		{"scroll-noop", "Down Up PgDn PgUp Home End j k"},
		{"enter", "Enter"},
		{"esc", "Esc"},
		{"space", "Space"},
	})
	runKeyGoldenScripts(t, cmdOutDialog("command_output_done_empty", "done", 0, false, 0), []simpleKeyScript{
		{"scroll-noop", "Down Up PgDn PgUp Home End"},
		{"enter", "Enter"},
		{"alt-o", "Alt+o"},
	})
}

func TestCommandOutputPathsKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, cmdOutDialog("command_output_paths", "done", 30, true, 0), []simpleKeyScript{
		{"down-up-by-line", "Down Down Down Up Up Up"},
		{"down-to-end", strings.Repeat("Down ", 8)},
		{"left-right-by-match", "Right Right Right Right Right Right Left Left Left Left Left Left Left"},
		{"right-then-down", "Right Down Down"},
		{"toggle-space", "Space Space"},
		{"toggle-insert", "Insert Insert"},
		{"toggle-advances-and-scrolls", "Space Space Space Space Space Space Space"},
		{"toggle-last-stays", strings.Repeat("Space ", 7)},
		{"vi-motion", "j j l l k h"},
		{"pgdn-pgup", "PgDn PgUp PgDn PgDn"},
		{"home-end", "Down Down End Home"},
		{"scroll-follows-cursor", strings.Repeat("Down ", 4) + "PgDn Up Up"},
		{"enter-closes", "Down Enter"},
		{"esc-closes", "Right Esc"},
		{"alt-o", "Space Alt+o"},
		{"alt-c", "Space Alt+c"},
		{"tab-then-keys", "Tab Down Space Enter"},
		{"invert", "Space *"},
		{"select-group-open-esc", "+ Esc"},
		{"unselect-group-open-esc", "- Esc"},
		{"plain-letters", "o c b x Down"},
		{"alt-b-ignored", "Alt+b Down"},
	})
	runKeyGoldenScripts(t, cmdOutDialog("command_output_paths_short", "done", 8, true, 0), []simpleKeyScript{
		{"walk", "Down Down Down Right Right Right Left Left Left Left Left Left Left"},
		{"toggle", "Space Space Space"},
	})
}
