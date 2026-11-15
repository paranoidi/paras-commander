package dialog

import (
	"errors"

	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// RunUnflatten queues an unflatten job for the active panel's files-only selection (or the cursor
// file). A source with a directory, or a file without an extension, is refused with an error toast.
func (h *Handler) RunUnflatten() {
	p := h.host.ActivePanel()
	sources, err := ops.ValidateUnflattenSource(p)
	if err != nil {
		var opsErr *ops.Error
		if errors.As(err, &opsErr) {
			h.host.SetTransientMessage(opsErr.Text, ui.MessageUrgencyError)
		} else {
			h.host.SetErrorMessage("Unflatten", err)
		}
		return
	}
	paths := make([]string, len(sources))
	for i, s := range sources {
		paths[i] = s.String()
	}
	p.ClearSelection()
	h.jobs.AddUnflattenJob(jobsctrl.UnflattenJobRequest{Sources: paths, Dir: p.PathString()})
	h.host.SetTransientMessage("Unflatten queued", ui.MessageUrgencyInfo)
}
