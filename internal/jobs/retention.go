package jobs

// RetentionPolicy controls which finished jobs are kept visible.
type RetentionPolicy struct {
	ShowFinished bool // Whether finished jobs are kept visible.
	KeepFinished int  // Maximum number of finished jobs to retain (0 = unlimited).
}

// Apply cleans up the queue according to the retention policy.
func (p RetentionPolicy) Apply(q *Queue) {
	if !p.ShowFinished {
		q.ClearFinished()
		return
	}
	if p.KeepFinished > 0 {
		q.RetainLastN(p.KeepFinished)
	}
}
