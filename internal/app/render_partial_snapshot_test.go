package app

import (
	"fmt"
	"sync"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
)

// TestPartialPaintersSnapshotModelUnderCommandsMu races preview-field writes against
// renderBrowserListNavUpdate and paintDiskUsageBrowserUpdate. Those painters must copy
// a.model under commandsMu, matching full render's RLock contract.
func TestPartialPaintersSnapshotModelUnderCommandsMu(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewBrowser
	app.model.ActiveSubFocus = ui.SubFocusFileList

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			app.commandsMu.Lock()
			app.model.FilePreview = ui.FilePreviewState{
				Open:         true,
				CombinedText: fmt.Sprintf("preview-%d", i),
				TitleBase:    "notes.txt",
			}
			app.model.CarouselFilePreview = ui.FilePreviewState{
				Open:         true,
				CombinedText: fmt.Sprintf("carousel-%d", i),
			}
			app.model.FullscreenFilePreview = ui.FilePreviewState{
				Open:         true,
				CombinedText: fmt.Sprintf("full-%d", i),
			}
			app.commandsMu.Unlock()
		}
	}()

	for range 80 {
		app.renderBrowserListNavUpdate(ui.PrimaryPanel)
		_ = app.paintDiskUsageBrowserUpdate()
	}
	close(stop)
	wg.Wait()
}
