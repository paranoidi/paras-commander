package app

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	metactrl "github.com/paranoidi/paras-commander/internal/apphandler/meta"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestHandleInterruptPayloadAppliesPostedBookmarkLoad(t *testing.T) {
	root := t.TempDir()
	harbor := filepath.Join(root, "harbor")
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.PathPicker = dialog.PathPickerState{
		Open:    true,
		Title:   "Bookmarks",
		Purpose: dialog.PathPickerPurposeNavigate,
	}
	payload := bookmarkIOLoadPayload([]dialog.PathPickerItem{{Name: "harbor", Path: harbor}})
	if err := screen.PostEvent(tcell.NewEventInterrupt(payload)); err != nil {
		t.Fatalf("PostEvent: %v", err)
	}

	applyNextInterruptEvent(t, app, screen)

	if !app.model.PathPicker.Open {
		t.Fatal("posted bookmark load closed the picker")
	}
	if len(app.model.PathPicker.Items) != 1 || app.model.PathPicker.Items[0].Path != harbor {
		t.Fatalf("items after event-loop apply = %+v, want [{harbor %s}]", app.model.PathPicker.Items, harbor)
	}
	if len(app.model.PathPicker.Ranked) != 1 {
		t.Fatalf("ranked = %v, want one entry after apply", app.model.PathPicker.Ranked)
	}
}

func TestHandleInterruptPayloadConsumesMetaRenderFlush(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	drainPendingEvents(screen)

	app.metaCtrl.HandleWake(metactrl.WakePayload{})
	if err := screen.PostEvent(tcell.NewEventInterrupt(metactrl.RenderFlushPayload{})); err != nil {
		t.Fatalf("PostEvent: %v", err)
	}

	applyNextInterruptEvent(t, app, screen)

	// HandleRenderFlush must stop the debounce armed by HandleWake. Without that
	// call, AfterFunc still posts a second RenderFlushPayload after ~16ms.
	deadline := time.Now().Add(40 * time.Millisecond)
	for time.Now().Before(deadline) {
		for screen.HasPendingEvent() {
			ev := screen.PollEvent()
			ie, ok := ev.(*tcell.EventInterrupt)
			if !ok {
				continue
			}
			if _, ok := ie.Data().(metactrl.RenderFlushPayload); ok {
				t.Fatal("RenderFlushPayload posted again; event loop should consume the armed debounce")
			}
		}
		time.Sleep(time.Millisecond)
	}
}

// bookmarkIOLoadPayload tags a BookmarkIOPayload as a marks-file load. kind is
// unexported so only the bookmark worker can set it; tests reconstruct that tag.
func bookmarkIOLoadPayload(items []dialog.PathPickerItem) dialogctrl.BookmarkIOPayload {
	p := dialogctrl.BookmarkIOPayload{Items: items}
	kind := reflect.ValueOf(&p).Elem().FieldByName("kind")
	reflect.NewAt(kind.Type(), kind.Addr().UnsafePointer()).Elem().SetInt(1)
	return p
}

func drainPendingEvents(screen tcell.SimulationScreen) {
	for screen.HasPendingEvent() {
		_ = screen.PollEvent()
	}
}
