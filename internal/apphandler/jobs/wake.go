package jobs

// WakePayload is posted through tcell when job channel updates should drain and repaint.
type WakePayload struct{}

// OpsWakePayload is posted when a background Operations fetch or scan has new rows; the app
// repaints the jobs view, whose render refreshes the Operations window (RefreshOpsWindow).
type OpsWakePayload struct{}
