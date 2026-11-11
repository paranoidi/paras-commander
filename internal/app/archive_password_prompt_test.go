package app

import (
	"context"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestArchivePasswordDialogOKAnswersWaiter(t *testing.T) {
	app := testAppMinimal(t)
	app.applyArchivePasswordOpen(archivePasswordOpenPayload{path: "/srv/otter.7z"})
	reply := make(chan archivePasswordAnswer, 1)
	app.archivePw.wait = &archivePasswordWait{id: 1, reply: reply}
	for _, r := range "heron" {
		app.handleArchivePasswordDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))
	if ans := <-reply; !ans.ok || ans.password != "heron" {
		t.Fatalf("answer = %+v", ans)
	}
	if app.model.ArchivePasswordDialog.Open || app.model.ArchivePasswordDialog.Password.Value != "" {
		t.Fatal("dialog should close and drop the password")
	}
}

func TestArchivePasswordDialogEnterAnswersAndEscSkips(t *testing.T) {
	for _, tc := range []struct {
		key    tcell.Key
		wantOK bool
	}{{tcell.KeyEnter, true}, {tcell.KeyEscape, false}} {
		app := testAppMinimal(t)
		app.applyArchivePasswordOpen(archivePasswordOpenPayload{path: "/srv/otter.7z", retry: true})
		reply := make(chan archivePasswordAnswer, 1)
		app.archivePw.wait = &archivePasswordWait{id: 1, reply: reply}
		for _, r := range "heron" {
			app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
		}
		app.handleKey(tcell.NewEventKey(tc.key, 0, tcell.ModNone))
		select {
		case ans := <-reply:
			if ans.ok != tc.wantOK {
				t.Fatalf("key %v: answer = %+v, want ok=%v", tc.key, ans, tc.wantOK)
			}
		case <-time.After(time.Second):
			t.Fatalf("key %v: no answer", tc.key)
		}
		if app.model.ArchivePasswordDialog.Open {
			t.Fatalf("key %v: dialog should close", tc.key)
		}
	}
}

func TestPromptArchivePasswordCanceledContextSkips(t *testing.T) {
	app := testAppMinimal(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if pw, ok := app.promptArchivePassword(ctx, "/srv/otter.7z", false); ok || pw != "" {
		t.Fatalf("got %q, %v; want skipped", pw, ok)
	}
}
