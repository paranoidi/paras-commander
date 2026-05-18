package jobs

// PlanItem describes one filesystem node in a pre-built copy/move plan (mirrors ops.PlanItem).
type PlanItem struct {
	Src       string
	Dst       string
	IsDir     bool
	IsSymlink bool
	FileSize  int64
}
