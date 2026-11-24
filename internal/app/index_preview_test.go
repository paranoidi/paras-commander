package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/ui"
)

// A panel showing the index previews the files its rows name, whichever form its path has.
func TestPreviewReadsTheFileOfAnIndexRow(t *testing.T) {
	for name, view := range map[string]func(dir string) (path, sel string){
		"plain index": func(dir string) (string, string) { return "fileindex://" + dir, "harbor.txt" },
		"structure":   func(dir string) (string, string) { return "fileindex(beach)://" + dir, "harbor.txt" },
		"flat tag list": func(dir string) (string, string) {
			return "fileindex@(beach):///", "fileindex@(beach)://" + dir + "/harbor.txt"
		},
	} {
		t.Run(name, func(t *testing.T) {
			a, screen, dir := tagViewDiskApp(t)
			file := filepath.Join(dir, "harbor.txt")
			if err := os.WriteFile(file, []byte("glowing words\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			path, sel := view(dir)
			tvGoTo(t, a, screen, path)
			if !a.focusedPanel().SelectVisibleEntry(sel) {
				t.Fatalf("no row %q", sel)
			}
			a.handleKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
			drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
				st := a.model.FullscreenFilePreview
				return st.Open && st.Phase == ui.FilePreviewPhaseDone
			})
			st := a.model.FullscreenFilePreview
			a.render()
			if st.Path != file || st.ErrorMsg != "" || !strings.Contains(screenText(a), "glowing words") {
				t.Fatalf("viewer path %q, error %q, screen:\n%s", st.Path, st.ErrorMsg, screenText(a))
			}
			if !strings.HasPrefix(st.ListedPath(), "fileindex") {
				t.Fatalf("a delete would act on %q, want the index record", st.ListedPath())
			}
			a.handleKey(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
			fd := a.model.FileDialog
			if !fd.Open || len(fd.DeleteEntries) != 1 || fd.DeleteEntries[0].Path != st.ListedPath() || !strings.Contains(fd.Note, "fileindex only") {
				t.Fatalf("delete dialog = %+v", fd)
			}
		})
	}
}
