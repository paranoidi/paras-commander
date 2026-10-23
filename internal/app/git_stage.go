package app

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// gitStageTimeout bounds a single git add/reset invocation.
const gitStageTimeout = 30 * time.Second

// gitRemoteTimeout bounds a git push/pull invocation.
const gitRemoteTimeout = 5 * time.Minute

// gitStageResultPayload delivers an async git add/reset result to the main loop.
type gitStageResultPayload struct {
	panelID  int
	workRoot string
	stage    bool
	n        int
	err      error
}

// runGitStage stages (git add) or unstages (git reset) paths in the work tree at root.
func runGitStage(root string, stage bool, paths []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitStageTimeout)
	defer cancel()
	args := []string{"-C", root, "add", "--"}
	if !stage {
		args = []string{"-C", root, "reset", "-q", "--"}
	}
	out, err := exec.CommandContext(ctx, "git", append(args, paths...)...).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// activeGitRoot returns the active panel's work tree root, or "" after showing why there is none.
func (a *App) activeGitRoot() string {
	dir, err := a.activePanel().Path.FilePath()
	if err != nil {
		a.setTransientMessage("Not a local directory", ui.MessageUrgencyInfo)
		return ""
	}
	root := gitignore.ValidWorkTreeRoot(dir)
	if root == "" {
		a.setTransientMessage("Not in a git repository", ui.MessageUrgencyWarn)
	}
	return root
}

// gitStageActive stages or unstages the active panel's selection, or the cursor entry and
// then advances the cursor.
func (a *App) gitStageActive(stage bool) {
	p := a.activePanel()
	root := a.activeGitRoot()
	if root == "" {
		return
	}
	paths := panelTargetPaths(p)
	if len(paths) == 0 {
		return
	}
	if len(p.SelectedPaths) == 0 && p.Cursor < p.VisibleEntryCount()-1 {
		p.Move(1, a.activeViewportRows())
	}
	panelID := a.model.ActivePanel
	go func() {
		err := runGitStage(root, stage, paths)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(gitStageResultPayload{panelID, root, stage, len(paths), err}))
	}()
}

// gitRemoteResultPayload delivers an async git push/pull result to the main loop.
type gitRemoteResultPayload struct {
	panelID  int
	workRoot string
	verb     string // "push" or "pull"
	res      cmdrun.RunResult
}

// gitRemoteActive runs `git push` or `git pull` (verb) in the background for the active panel's
// work tree. The child gets no terminal (Setsid) and GIT_TERMINAL_PROMPT=0, so credentials must
// come from an agent or credential helper instead of an interactive prompt.
func (a *App) gitRemoteActive(verb string) {
	root := a.activeGitRoot()
	if root == "" {
		return
	}
	panelID := a.model.ActivePanel
	a.setTransientMessage("git "+verb+"…", ui.MessageUrgencyInfo)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), gitRemoteTimeout)
		defer cancel()
		res := cmdrun.RunTracked(ctx, []string{"git", "-C", root, verb}, []string{"GIT_TERMINAL_PROMPT=0"}, root, 0, nil, nil)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(gitRemoteResultPayload{panelID, root, verb, res}))
	}()
}

func (a *App) applyGitRemoteResult(d gitRemoteResultPayload) {
	if d.res.LaunchErr != nil || d.res.ExitCode != 0 {
		err := d.res.LaunchErr
		if err == nil {
			msg := strings.TrimSpace(string(d.res.Stderr))
			if msg == "" {
				msg = fmt.Sprintf("exit status %d", d.res.ExitCode)
			}
			err = errors.New(msg)
		}
		a.setErrorMessage("git "+d.verb+" failed", err)
		return
	}
	a.reloadGitPanels(d.panelID, d.workRoot, "git "+d.verb+" done")
}

func (a *App) applyGitStageResult(d gitStageResultPayload) {
	if d.err != nil {
		prefix := "git reset failed"
		if d.stage {
			prefix = "git add failed"
		}
		a.setErrorMessage(prefix, d.err)
		return
	}
	msg := fmt.Sprintf("Unstaged %d item(s)", d.n)
	if d.stage {
		msg = fmt.Sprintf("Staged %d item(s)", d.n)
	}
	a.reloadGitPanels(d.panelID, d.workRoot, msg)
}

// reloadGitPanels reloads panelID with msg and refreshes the other panel when it shows the same
// work tree.
func (a *App) reloadGitPanels(panelID int, workRoot, msg string) {
	a.reloadPanel(panelID, msg)
	other := ui.PrimaryPanel
	if panelID == ui.PrimaryPanel {
		other = ui.SecondaryPanel
	}
	if dir, err := a.panelByID(other).Path.FilePath(); err == nil && gitignore.ValidWorkTreeRoot(dir) == workRoot {
		_ = a.panelByID(other).Refresh(a.activeViewportRows())
	}
}

// gitCommitActive runs an interactive `git commit` (the user's editor) in the active panel's
// work tree, suspending the TUI while it runs.
func (a *App) gitCommitActive() {
	root := a.activeGitRoot()
	if root == "" {
		return
	}
	// ponytail: sync pre-check; `git diff --cached --quiet` is instant and gates the suspend.
	if exec.Command("git", "-C", root, "diff", "--cached", "--quiet").Run() == nil {
		a.setTransientMessage("Nothing staged to commit", ui.MessageUrgencyWarn)
		return
	}
	if err := a.withTerminalReleased(func() error {
		return cmdrun.RunInteractive(context.Background(), []string{"git", "-C", root, "commit"}, root)
	}); err != nil {
		a.setErrorMessage("git commit failed or aborted", err)
		return
	}
	a.reloadPanel(a.model.ActivePanel, "Committed")
}
