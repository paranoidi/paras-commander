package app

import (
	"context"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ops"
)

func TestHandleElevateDialogKeyAnswersWaiter(t *testing.T) {
	app := testAppMinimal(t)
	app.applyElevateOpen(elevateOpenPayload{req: ops.PermissionRequest{Op: "delete", Path: "/srv/otter"}})
	reply := make(chan ops.PermissionAnswer, 1)
	app.elevate.wait = &elevateWait{id: 1, reply: reply}

	for _, r := range "heron" {
		app.handleElevateDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleElevateDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	select {
	case ans := <-reply:
		if ans.Decision != ops.PermissionElevate || ans.Password != "heron" {
			t.Fatalf("answer = %+v", ans)
		}
	case <-time.After(time.Second):
		t.Fatal("no answer")
	}
	if app.model.ElevateDialog.Open {
		t.Fatal("dialog should close after answering")
	}
}

func TestHandleElevateDialogAltShortcuts(t *testing.T) {
	for _, tc := range []struct {
		r    rune
		want ops.PermissionDecision
	}{{'s', ops.PermissionSkip}, {'c', ops.PermissionAbort}} {
		app := testAppMinimal(t)
		app.applyElevateOpen(elevateOpenPayload{req: ops.PermissionRequest{Op: "rename", Path: "/srv/otter"}})
		reply := make(chan ops.PermissionAnswer, 1)
		app.elevate.wait = &elevateWait{id: 1, reply: reply}
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, tc.r, tcell.ModAlt))
		if ans := <-reply; ans.Decision != tc.want {
			t.Fatalf("Alt+%c: decision = %v, want %v", tc.r, ans.Decision, tc.want)
		}
	}
}

func TestPromptElevateCanceledContextAborts(t *testing.T) {
	app := testAppMinimal(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ans := app.promptElevate(ctx, ops.PermissionRequest{Op: "delete", Path: "/srv/otter"}); ans.Decision != ops.PermissionAbort {
		t.Fatalf("decision = %v, want abort", ans.Decision)
	}
}
