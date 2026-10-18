package dialog

import (
	"fmt"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func TestDeleteDialogDrawsScrollbarWhenListOverflows(t *testing.T) {
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	var entries []DeleteListEntry
	for i := 0; i < 60; i++ {
		entries = append(entries, DeleteListEntry{Name: fmt.Sprintf("willow%02d.txt", i), Type: localfs.EntryFile})
	}
	state := FileDialogState{Open: true, DialogType: FileDialogDelete, DeleteSummary: "60 files", DeleteEntries: entries}
	render := func(style uiscrollbar.Style) []string {
		screen := buttonBlankRowScreen(t)
		DrawFileDialog(screen, layout, state, DialogRenderContext{Styles: theme.Default(), ScrollbarStyle: style}, nil)
		return dialogScreenRows(screen)
	}
	bar, thumb := render(uiscrollbar.StyleBar), render(uiscrollbar.StyleThumb)
	for y := range bar {
		if bar[y] != thumb[y] {
			return
		}
	}
	t.Fatal("overflowing delete list drew no scrollbar")
}
