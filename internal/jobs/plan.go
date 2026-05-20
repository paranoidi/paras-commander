package jobs

import "github.com/paranoidi/paras-commander/internal/pathloc"

// PlanItem describes one filesystem node in a pre-built copy/move plan (mirrors ops.PlanItem).
type PlanItem struct {
	Src       pathloc.Path
	Dst       pathloc.Path
	IsDir     bool
	IsSymlink bool
	FileSize  int64
}
