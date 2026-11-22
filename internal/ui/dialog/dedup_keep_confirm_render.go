package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// dedupKeepConfirmMaxListRows caps visible kept-copy rows (a trailing "(+N more)" line takes the
// last slot when the list is longer).
const dedupKeepConfirmMaxListRows = 8

const (
	dedupKeepConfirmListIndent = 2
	dedupKeepConfirmDirPrefix  = "Directory: "
)

func dedupKeepConfirmDir(state DedupKeepConfirmState) string {
	if state.Dir == "" {
		return "."
	}
	return state.Dir
}

func dedupKeepConfirmWidth(state DedupKeepConfirmState, lines []string) int {
	width := utf8.RuneCountInString(dedupKeepConfirmDirPrefix+dedupKeepConfirmDir(state)) + 4
	for _, l := range lines {
		width = max(width, utf8.RuneCountInString(l)+4)
	}
	for _, p := range state.Kept {
		width = max(width, utf8.RuneCountInString(p)+4+dedupKeepConfirmListIndent)
	}
	return min(max(width, 50), 78)
}

// DedupKeepConfirmView describes the confirmation: the directory, how many groups already keep a
// copy elsewhere, those copies (capped at dedupKeepConfirmMaxListRows rows, a trailing "(+N more)"
// line replacing the last one), what each choice does, and Keep previous / Keep this / Cancel.
func DedupKeepConfirmView(state DedupKeepConfirmState) dlg.Spec {
	groups := fmt.Sprintf("%d duplicate groups already keep a copy elsewhere:", state.Groups)
	if state.Groups == 1 {
		groups = "1 duplicate group already keeps a copy elsewhere:"
	}
	help := []string{"Keep previous: leave those groups as they are.", "Keep this: replace those keeps with this directory."}
	items := []dlg.Item{
		{Kind: dlg.KindLabel, Prefix: dedupKeepConfirmDirPrefix, Label: dedupKeepConfirmDir(state), Fit: dlg.FitPath},
		{Kind: dlg.KindLabel, Label: groups},
	}
	shown := len(state.Kept)
	truncated := shown > dedupKeepConfirmMaxListRows
	if truncated {
		shown = dedupKeepConfirmMaxListRows - 1
	}
	for _, p := range state.Kept[:shown] {
		items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: p, Fit: dlg.FitPath, Indent: dedupKeepConfirmListIndent})
	}
	if truncated {
		items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: fmt.Sprintf("(+%d more)", len(state.Kept)-shown), Indent: dedupKeepConfirmListIndent})
	}
	items = append(items, dlg.Item{Kind: dlg.KindLabel})
	for _, l := range help {
		items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: l})
	}
	return dlg.Spec{
		Title: "Keep Directory", Width: dedupKeepConfirmWidth(state, append(help, groups)), Unclamped: true, AltOnly: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{
			{ID: DedupKeepPrevious, Label: "Keep previous", Mnemonic: 'P'},
			{ID: DedupKeepThis, Label: "Keep this", Mnemonic: 'T'},
			dlg.CancelButton,
		},
	}
}

func DrawDedupKeepConfirmDialog(screen tcell.Screen, layout Layout, state DedupKeepConfirmState, ctx DialogRenderContext) {
	dlg.Draw(screen, layout.Width, layout.Height, DedupKeepConfirmView(state), state.Focus, ctx.Styles)
}
