package app

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// gitStageTimeout bounds a single git add/reset invocation.
const gitStageTimeout = 30 * time.Second

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

// gitStageActive stages or unstages the active panel's selection (or cursor entry).
func (a *App) gitStageActive(stage bool) {
	p := a.activePanel()
	dir, err := p.Path.FilePath()
	if err != nil {
		a.setTransientMessage("Not a local directory", ui.MessageUrgencyInfo)
		return
	}
	root := gitignore.ValidWorkTreeRoot(dir)
	if root == "" {
		a.setTransientMessage("Not in a git repository", ui.MessageUrgencyWarn)
		return
	}
	paths := panelTargetPaths(p)
	if len(paths) == 0 {
		return
	}
	panelID := a.model.ActivePanel
	go func() {
		err := runGitStage(root, stage, paths)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(gitStageResultPayload{panelID, root, stage, len(paths), err}))
	}()
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
	a.reloadPanel(d.panelID, msg)
	other := ui.PrimaryPanel
	if d.panelID == ui.PrimaryPanel {
		other = ui.SecondaryPanel
	}
	if dir, err := a.panelByID(other).Path.FilePath(); err == nil && gitignore.ValidWorkTreeRoot(dir) == d.workRoot {
		_ = a.panelByID(other).Refresh(a.activeViewportRows())
	}
}
