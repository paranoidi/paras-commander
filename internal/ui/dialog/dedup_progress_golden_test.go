package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the find-duplicates progress modal: every phase, both button layouts and
// focus positions, short and overlong paths / labels, the per-file bar, a narrow width and a
// clamped height. The ETA is deterministic because HashStarted stays zero.

func dedupProgressGoldenState(focus string) DedupProgressDialogState {
	st := DedupProgressDialogState{Open: true}
	if focus == "cancel" {
		st.Focus = dlg.Cancel
	}
	return st
}

func dedupProgressGoldenSnap(phase comparepkg.DedupPhase, long bool) comparepkg.DedupSnapshot {
	root := "/srv/meadow/orchard"
	if long {
		root = "/srv/" + strings.Repeat("glacier-harbor/", 6) + "orchard"
	}
	return comparepkg.DedupSnapshot{Root: pathloc.MustParse(root), Phase: phase}
}

func TestDedupProgressGoldenRender(t *testing.T) {
	goldenBandRows["dedup_progress"] = 16
	var cases []simpleGolden
	add := func(name string, w, h int, st DedupProgressDialogState, snap comparepkg.DedupSnapshot) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawDedupProgressDialog(s, l, st, snap, asciiDialogStyles(), "")
		}})
	}
	walk := func(n int, long bool) comparepkg.DedupSnapshot {
		s := dedupProgressGoldenSnap(comparepkg.DedupWalking, long)
		s.Walked = n
		return s
	}
	add("walking_0", 80, 24, dedupProgressGoldenState("ok"), walk(0, false))
	add("walking_1", 80, 24, dedupProgressGoldenState("ok"), walk(1, false))
	add("walking_many", 80, 24, dedupProgressGoldenState("ok"), walk(48213, false))
	add("walking_longroot", 80, 24, dedupProgressGoldenState("ok"), walk(7, true))

	confirm := dedupProgressGoldenSnap(comparepkg.DedupAwaitConfirm, false)
	confirm.HashBytesTotal = 5 << 30
	for _, focus := range []string{"ok", "cancel"} {
		add("confirm_focus-"+focus, 80, 24, dedupProgressGoldenState(focus), confirm)
	}
	confirmSmall := confirm
	confirmSmall.HashBytesTotal = 900
	add("confirm_bytes", 80, 24, dedupProgressGoldenState("ok"), confirmSmall)
	confirmHuge := confirm
	confirmHuge.HashBytesTotal = 3 << 40
	add("confirm_huge", 80, 24, dedupProgressGoldenState("cancel"), confirmHuge)

	hash := func(cur, file string, hashed, total int, hb, tb int64) comparepkg.DedupSnapshot {
		s := dedupProgressGoldenSnap(comparepkg.DedupHashing, false)
		s.Current, s.CurrentFile = cur, file
		s.Hashed, s.HashTotal, s.HashedBytes, s.HashBytesTotal = hashed, total, hb, tb
		if file != "" {
			s.CurrentFileSize, s.CurrentFileDone = 1<<29, 1<<28
		}
		return s
	}
	add("hash_nolabel", 80, 24, dedupProgressGoldenState("ok"), hash("", "", 0, 4, 0, 400))
	add("hash_started", 80, 24, dedupProgressGoldenState("ok"), hash("nested", "", 1, 4, 100, 400))
	add("hash_half", 80, 24, dedupProgressGoldenState("ok"), hash("media/archive", "", 2, 4, 200, 400))
	add("hash_done", 80, 24, dedupProgressGoldenState("ok"), hash("media/archive", "", 4, 4, 400, 400))
	add("hash_overshoot", 80, 24, dedupProgressGoldenState("ok"), hash("media", "", 5, 4, 900, 400))
	add("hash_unknown_total", 80, 24, dedupProgressGoldenState("ok"), hash("media", "", 1, 4, 10, 0))
	add("hash_file_bar", 80, 24, dedupProgressGoldenState("ok"), hash("media/archive", "voyage.iso", 1, 4, 1<<29, 1<<30))
	add("hash_longlabel", 80, 24, dedupProgressGoldenState("ok"), hash(strings.Repeat("segment/", 12)+"verylongfilename.txt", strings.Repeat("quartz-", 20)+"pebble.iso", 14, 56, 14, 56))
	add("hash_longroot", 80, 24, dedupProgressGoldenState("ok"), func() comparepkg.DedupSnapshot {
		s := hash("nested", "", 3, 9, 30, 90)
		s.Root = dedupProgressGoldenSnap(comparepkg.DedupHashing, true).Root
		return s
	}())

	for _, p := range []comparepkg.DedupPhase{comparepkg.DedupDone, comparepkg.DedupError, comparepkg.DedupCanceled} {
		add(fmt.Sprintf("other_phase%d", p), 80, 24, dedupProgressGoldenState("ok"), dedupProgressGoldenSnap(p, false))
	}

	add("narrow60_walking", 60, 24, dedupProgressGoldenState("ok"), walk(12, true))
	add("narrow60_confirm", 60, 24, dedupProgressGoldenState("cancel"), confirm)
	add("narrow60_hash", 60, 24, dedupProgressGoldenState("ok"), hash("media/archive", "voyage.iso", 1, 4, 100, 400))
	add("narrow40_hash", 40, 24, dedupProgressGoldenState("ok"), hash("media/archive", "voyage.iso", 1, 4, 100, 400))
	add("short10_walking", 80, 10, dedupProgressGoldenState("ok"), walk(3, false))
	add("short8_hash", 80, 8, dedupProgressGoldenState("ok"), hash("media", "voyage.iso", 1, 4, 100, 400))
	add("short6_confirm", 80, 6, dedupProgressGoldenState("ok"), confirm)
	runSimpleGoldens(t, "dedup_progress", cases)
}
