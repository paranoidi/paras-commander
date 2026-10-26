package dialog

import (
	"fmt"
	"time"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DedupProgressView is the find-duplicates scan modal's widget spec. The body follows the scan
// phase (walking status, hash-size confirmation, or the hashing bars with count and ETA); the
// confirm gate has OK / Cancel, every other phase only Cancel. The row count is fixed per phase
// so the dialog never resizes mid-scan. now is the clock the ETA is read against.
func DedupProgressView(snap comparepkg.DedupSnapshot, userHomeDir string, now time.Time) dlg.Spec {
	label := func(text string, fit dlg.Fit, center bool) dlg.Item {
		return dlg.Item{Kind: dlg.KindLabel, Label: text, Fit: fit, Center: center}
	}
	blank := dlg.Item{Kind: dlg.KindLabel}
	items := []dlg.Item{
		label("Directory:", dlg.FitRight, false),
		label(primitive.PathWithHomeTilde(snap.Root.String(), userHomeDir), dlg.FitPath, false),
		{Kind: dlg.KindSeparator},
	}
	buttons := []dlg.Button{dlg.CancelButton}
	switch snap.Phase {
	case comparepkg.DedupWalking:
		items = append(items,
			label("Walking directories"+string(primitive.Ellipsis)+dedupWalkFilesSuffix(snap.Walked), dlg.FitPath, false), blank, blank)
	case comparepkg.DedupAwaitConfirm:
		items = append(items, label(formatDedupByteSize(snap.HashBytesTotal)+" to hash. Continue?", dlg.FitRight, false), blank, blank)
		buttons = []dlg.Button{dlg.OKButton, buttons[0]}
	case comparepkg.DedupHashing:
		current := snap.Current
		if current == "" {
			current = "Hashing files" + string(primitive.Ellipsis)
		}
		// The per-file bar (large files only) is a blank row otherwise.
		file := blank
		if snap.CurrentFile != "" {
			file = dlg.Item{Kind: dlg.KindBar, Label: snap.CurrentFile, Frac: dedupFrac(snap.CurrentFileDone, snap.CurrentFileSize)}
		}
		eta := jobs.FormatETA(jobs.StatusRunning, snap.HashStarted, now, snap.HashBytesTotal, snap.HashedBytes, snap.HashTotal, snap.Hashed, 0, 0)
		items = append(items,
			dlg.Item{Kind: dlg.KindBar, Label: current, Frac: dedupFrac(snap.HashedBytes, snap.HashBytesTotal)},
			file,
			label(fmt.Sprintf("%d/%d", snap.Hashed, snap.HashTotal), dlg.FitRight, true),
			label("ETA "+eta, dlg.FitRight, true))
	default:
		items = append(items, label("Scanning"+string(primitive.Ellipsis), dlg.FitRight, false), blank, blank)
	}
	return dlg.Spec{
		Title: "Find Duplicates", Width: PreferredFormDialogWidth, Unclamped: true,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: buttons,
	}
}

func dedupWalkFilesSuffix(walked int) string {
	noun := "files"
	if walked == 1 {
		noun = "file"
	}
	return fmt.Sprintf(" %d %s", walked, noun)
}

// dedupFrac returns done/total clamped to [0,1]; zero when total is unknown.
func dedupFrac(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	if done >= total {
		return 1
	}
	if done < 0 {
		return 0
	}
	return float64(done) / float64(total)
}

func formatDedupByteSize(n int64) string {
	if n < 0 {
		n = 0
	}
	const (
		KiB = int64(1024)
		MiB = KiB * 1024
		GiB = MiB * 1024
		TiB = GiB * 1024
	)
	switch {
	case n < KiB:
		return fmt.Sprintf("%d B", n)
	case n < MiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(KiB))
	case n < GiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(MiB))
	case n < TiB:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(GiB))
	default:
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(TiB))
	}
}
