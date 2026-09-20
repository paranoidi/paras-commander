package dialog

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// RemoteFileOpKind identifies which foreground remote op a RemoteFileOpPayload completes.
type RemoteFileOpKind int

const (
	RemoteFileOpMkdir RemoteFileOpKind = iota + 1
	RemoteFileOpRename
	RemoteFileOpTransferProbe
	RemoteFileOpFlattenProbe
	RemoteFileOpExtractProbe
)

// RemoteFileOpPayload carries a background remote mkdir/rename or transfer/extract/flatten
// destination-probe result back to the event loop. Gen identifies the start that produced it;
// ApplyRemoteFileOp drops stale results so a superseded op cannot mutate UI state.
type RemoteFileOpPayload struct {
	Gen  uint64
	Kind RemoteFileOpKind
	Err  error

	mkdir    mkdirApply
	rename   renameApply
	transfer transferProbeApply
	flatten  flattenProbeApply
	extract  extractProbeApply
}

type mkdirApply struct {
	plan           ops.MkdirPlan
	action         dialog.MkdirAction
	sources        []string
	openInInactive bool
	priorEntryName string
	panelID        int
}

type renameApply struct {
	plan       ops.RenamePlan
	entry      localfs.Entry
	focusAfter bool
	panelDir   pathloc.Path
	panelID    int
}

type transferProbeApply struct {
	sources     []string
	dest        string
	destLoc     pathloc.Path
	flat        bool
	startPaused bool
	jobType     jobs.Type
	preserve    jobs.TransferPreserve
	nSelf       int
	destIsDir   bool
}

type flattenProbeApply struct {
	sources     []string
	dest        string
	removeEmpty bool
	dirRoots    []string
	nSelf       int
}

type extractProbeApply struct {
	plan    ops.ExtractPlan
	skipped []string
}

func locationIsRemote(raw string) bool {
	loc, err := pathloc.Parse(strings.TrimSpace(raw))
	return err == nil && loc.IsRemote()
}

func (h *Handler) useRemoteFileOp(paths ...string) bool {
	if h.testRemote != nil {
		return true
	}
	for _, p := range paths {
		if locationIsRemote(p) {
			return true
		}
	}
	return false
}

func (h *Handler) nextRemoteFileOpGen() uint64 {
	h.remoteFileOpGen++
	return h.remoteFileOpGen
}

func (h *Handler) invalidateRemoteFileOp() {
	h.remoteFileOpGen++
}

// ApplyRemoteFileOp applies a background remote mkdir/rename/dest-probe unless a newer
// op was started (or the transfer/flatten dialog was cancelled) in the meantime.
func (h *Handler) ApplyRemoteFileOp(p RemoteFileOpPayload) {
	if p.Gen != h.remoteFileOpGen {
		return
	}
	switch p.Kind {
	case RemoteFileOpMkdir:
		h.applyRemoteMkdir(p)
	case RemoteFileOpRename:
		h.applyRemoteRename(p)
	case RemoteFileOpTransferProbe:
		h.applyRemoteTransferProbe(p)
	case RemoteFileOpFlattenProbe:
		h.applyRemoteFlattenProbe(p)
	case RemoteFileOpExtractProbe:
		h.applyRemoteExtractProbe(p)
	}
}

func (h *Handler) startRemoteMkdir(st mkdirApply) {
	gen := h.nextRemoteFileOpGen()
	h.CloseFileDialog()
	panelPath := h.host.ActivePanel().PathString()
	input := st.plan.Name
	screen := h.screen
	backend := h.testRemote
	go func() {
		plan, err := remotePlanAndMkdir(backend, input, panelPath)
		if err == nil {
			st.plan = plan
		}
		if screen == nil {
			return
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt(RemoteFileOpPayload{
			Gen: gen, Kind: RemoteFileOpMkdir, Err: err, mkdir: st,
		}))
	}()
}

func (h *Handler) applyRemoteMkdir(p RemoteFileOpPayload) {
	if p.Err != nil {
		h.host.SetErrorMessage("Mkdir", p.Err)
		return
	}
	h.applyMkdirSuccess(p.mkdir)
}

func (h *Handler) startRemoteRename(st renameApply, newName, panelPath string) {
	gen := h.nextRemoteFileOpGen()
	h.CloseFileDialog()
	screen := h.screen
	backend := h.testRemote
	entry := st.entry
	go func() {
		plan, err := remotePlanAndRename(backend, entry, newName, panelPath)
		if err == nil {
			st.plan = plan
		}
		if screen == nil {
			return
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt(RemoteFileOpPayload{
			Gen: gen, Kind: RemoteFileOpRename, Err: err, rename: st,
		}))
	}()
}

func (h *Handler) applyRemoteRename(p RemoteFileOpPayload) {
	if p.Err != nil {
		if strings.Contains(p.Err.Error(), "source") {
			h.host.SetErrorMessage("Rename source", p.Err)
			return
		}
		if errors.Is(p.Err, errRenameFailed) || strings.HasPrefix(p.Err.Error(), "rename failed") {
			h.host.SetErrorMessage("Rename failed", p.Err)
			return
		}
		h.host.SetErrorMessage("Rename", p.Err)
		return
	}
	h.applyRenameSuccess(p.rename)
}

func (h *Handler) startRemoteTransferProbe(st transferProbeApply) {
	gen := h.nextRemoteFileOpGen()
	screen := h.screen
	backend := h.testRemote
	srcLocs := make([]pathloc.Path, len(st.sources))
	for i, src := range st.sources {
		srcLocs[i] = pathloc.MustParse(src)
	}
	destLoc := st.destLoc
	flat := st.flat
	go func() {
		nSelf, destIsDir := remoteSelfTargetAndDestDir(backend, srcLocs, destLoc, flat)
		st.nSelf = nSelf
		st.destIsDir = destIsDir
		if screen == nil {
			return
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt(RemoteFileOpPayload{
			Gen: gen, Kind: RemoteFileOpTransferProbe, transfer: st,
		}))
	}()
}

func (h *Handler) applyRemoteTransferProbe(p RemoteFileOpPayload) {
	st := p.transfer
	if !h.model.TransferDialog.Open {
		return
	}
	h.finishTransferEnqueue(st, st.nSelf, st.destIsDir, true)
}

func (h *Handler) startRemoteFlattenProbe(st flattenProbeApply, destLoc pathloc.Path, roots []pathloc.Path, recursive bool) {
	gen := h.nextRemoteFileOpGen()
	screen := h.screen
	backend := h.testRemote
	go func() {
		sources, nSelf, err := remoteCollectFlattenAndSelfTarget(backend, roots, destLoc, recursive)
		st.sources = sources
		st.nSelf = nSelf
		if screen == nil {
			return
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt(RemoteFileOpPayload{
			Gen: gen, Kind: RemoteFileOpFlattenProbe, Err: err, flatten: st,
		}))
	}()
}

func (h *Handler) applyRemoteFlattenProbe(p RemoteFileOpPayload) {
	if !h.model.FlattenDialog.Open {
		return
	}
	if p.Err != nil {
		var opsErr *ops.Error
		if errors.As(p.Err, &opsErr) {
			h.host.SetTransientMessage(opsErr.Text, ui.MessageUrgencyWarn)
		} else {
			h.host.SetErrorMessage("Flatten", p.Err)
		}
		return
	}
	h.finishFlattenEnqueue(p.flatten)
}

func (h *Handler) startRemoteExtractProbe(sources []string, dest string) {
	gen := h.nextRemoteFileOpGen()
	h.CloseFileDialog()
	screen := h.screen
	backend := h.testRemote
	go func() {
		if backend != nil {
			if loc, err := pathloc.Parse(dest); err == nil {
				_, _ = backend.Stat(context.Background(), loc)
			}
		}
		tc := archive.ProbeToolchain()
		plan, skipped, err := ops.PlanExtract(sources, dest, tc)
		if screen == nil {
			return
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt(RemoteFileOpPayload{
			Gen: gen, Kind: RemoteFileOpExtractProbe, Err: err,
			extract: extractProbeApply{plan: plan, skipped: skipped},
		}))
	}()
}

func (h *Handler) applyRemoteExtractProbe(p RemoteFileOpPayload) {
	if p.Err != nil {
		h.host.OpenMessageDialog("Extract", p.Err.Error())
		return
	}
	h.finishExtractEnqueue(p.extract.plan, p.extract.skipped)
}

func remotePlanAndMkdir(backend fsbackend.Backend, input, panelPath string) (ops.MkdirPlan, error) {
	if backend != nil {
		parent, err := pathloc.Parse(panelPath)
		if err != nil {
			return ops.MkdirPlan{}, &ops.Error{Op: "mkdir", Text: "invalid panel path", Err: err}
		}
		name := strings.TrimSpace(input)
		if name == "" {
			return ops.MkdirPlan{}, &ops.Error{Op: "mkdir", Text: "directory name is empty"}
		}
		dest, err := parent.Join(name)
		if err != nil {
			return ops.MkdirPlan{}, &ops.Error{Op: "mkdir", Text: err.Error(), Err: err}
		}
		if _, err := backend.Stat(context.Background(), dest); err == nil {
			return ops.MkdirPlan{}, &ops.Error{Op: "mkdir", Text: "target already exists"}
		} else if !remoteIsNotExist(err) {
			return ops.MkdirPlan{}, &ops.Error{Op: "mkdir", Text: "cannot stat target", Err: err}
		}
		if err := backend.Mkdir(context.Background(), dest, 0o755); err != nil {
			return ops.MkdirPlan{}, err
		}
		return ops.MkdirPlan{Path: dest.String(), Name: name}, nil
	}
	plan, err := ops.PlanMkdir(input, panelPath)
	if err != nil {
		return plan, err
	}
	if err := ops.ExecuteMkdir(plan); err != nil {
		return plan, err
	}
	return plan, nil
}

var errRenameFailed = errors.New("rename failed")

func remotePlanAndRename(backend fsbackend.Backend, entry localfs.Entry, newName, panelPath string) (ops.RenamePlan, error) {
	if backend != nil {
		srcLoc, err := pathloc.Parse(entry.Path)
		if err != nil {
			return ops.RenamePlan{}, &ops.Error{Op: "rename", Text: "invalid source path", Err: err}
		}
		dest, err := srcLoc.Parent().Join(strings.TrimSpace(newName))
		if err != nil {
			return ops.RenamePlan{}, &ops.Error{Op: "rename", Text: err.Error(), Err: err}
		}
		if _, err := backend.Stat(context.Background(), dest); err == nil {
			return ops.RenamePlan{}, &ops.Error{Op: "rename", Text: "target already exists"}
		} else if !remoteIsNotExist(err) {
			return ops.RenamePlan{}, &ops.Error{Op: "rename", Text: "cannot stat target", Err: err}
		}
		if err := backend.Rename(context.Background(), srcLoc, dest); err != nil {
			return ops.RenamePlan{}, fmt.Errorf("%w: %v", errRenameFailed, err)
		}
		return ops.RenamePlan{SourcePath: srcLoc.String(), NewName: newName, NewPath: dest.String()}, nil
	}
	plan, err := ops.PlanRename(entry, newName, panelPath)
	if err != nil {
		return plan, err
	}
	if err := ops.ExecuteRename(plan); err != nil {
		return plan, fmt.Errorf("%w: %v", errRenameFailed, err)
	}
	return plan, nil
}

func remoteSelfTargetAndDestDir(backend fsbackend.Backend, sources []pathloc.Path, dest pathloc.Path, flat bool) (nSelf int, destIsDir bool) {
	if backend != nil {
		ent, err := backend.Stat(context.Background(), dest)
		destIsDir = err == nil && ent.Type == fsbackend.EntryDirectory
		return selfTargetCountWithDestDir(sources, dest, flat, destIsDir), destIsDir
	}
	nSelf = ops.SelfTargetCount(sources, dest, flat)
	destIsDir = ops.DestinationIsDirAtEnqueue(dest)
	return nSelf, destIsDir
}

func remoteCollectFlattenAndSelfTarget(backend fsbackend.Backend, roots []pathloc.Path, dest pathloc.Path, recursive bool) ([]string, int, error) {
	if backend != nil {
		if len(roots) > 0 {
			_, _ = backend.List(context.Background(), roots[0])
		}
		nSelf, _ := remoteSelfTargetAndDestDir(backend, roots, dest, true)
		return nil, nSelf, nil
	}
	sources, err := ops.CollectFlattenSources(context.Background(), roots, dest, recursive)
	if err != nil {
		return nil, 0, err
	}
	srcLocs := make([]pathloc.Path, len(sources))
	for i, src := range sources {
		srcLocs[i] = pathloc.MustParse(src)
	}
	return sources, ops.SelfTargetCount(srcLocs, dest, true), nil
}

func selfTargetCountWithDestDir(sources []pathloc.Path, destDir pathloc.Path, flatDestNames, destIsDir bool) int {
	var root pathloc.Path
	if !flatDestNames {
		root = ops.TransferNameRoot(sources)
	}
	n := 0
	for _, src := range sources {
		dst := destDir
		if destIsDir {
			if child, err := destDir.Join(ops.TransferDestName(src, root)); err == nil {
				dst = child
			}
		}
		if ops.PathsEquivalent(src, dst) {
			n++
		}
	}
	return n
}

func remoteIsNotExist(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such file") || strings.Contains(msg, "not found")
}

func (h *Handler) destIsDirForEnqueue(dest string) bool {
	if h.enqueueDestIsDir != nil {
		v := *h.enqueueDestIsDir
		h.enqueueDestIsDir = nil
		return v
	}
	loc, err := pathloc.Parse(dest)
	if err != nil || loc.IsRemote() {
		return false
	}
	return ops.DestinationIsDirAtEnqueue(loc)
}

func (h *Handler) setEnqueueDestIsDir(isDir bool) {
	v := isDir
	h.enqueueDestIsDir = &v
}

func (h *Handler) finishTransferEnqueue(st transferProbeApply, nSelf int, destIsDir bool, destIsDirKnown bool) {
	d := &h.model.TransferDialog
	if nSelf > 0 {
		if len(st.sources) > 1 {
			h.host.SetTransientMessage("Cannot transfer multiple items when some would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		d.Phase = dialog.TransferPhaseSelfCopyRename
		d.SelfCopyDestDir = st.destLoc.String()
		base := st.sources[0]
		if loc, err := pathloc.Parse(base); err == nil {
			base = loc.Base()
		}
		d.SelfCopyOrigBasename = base
		d.SelfCopyNewName = transferSelfCopyNewNamePrefilled(base)
		d.FocusField = 0
		h.transferDestValidate.Invalidate()
		d.DestPathInvalid = false
		d.DestPathCheckPending = false
		h.model.DestinationTargetPrimary = false
		h.model.DestinationTargetSecondary = false
		return
	}
	if destIsDirKnown {
		h.setEnqueueDestIsDir(destIsDir)
	}
	sourcesCopy := append([]string(nil), st.sources...)
	h.host.ActivePanel().ClearSelection()
	h.AddTransferJob(st.jobType, sourcesCopy, st.dest, st.startPaused, st.preserve)
	h.CloseTransferDialog()
	h.setTransferQueuedMessage(st.jobType, st.startPaused)
}

func (h *Handler) finishFlattenEnqueue(st flattenProbeApply) {
	if len(st.sources) == 0 {
		h.host.SetTransientMessage("Nothing to flatten", ui.MessageUrgencyWarn)
		return
	}
	if st.nSelf > 0 {
		if len(st.sources) > 1 {
			h.host.SetTransientMessage("Cannot flatten when some items would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		h.host.SetTransientMessage("Nothing to flatten", ui.MessageUrgencyWarn)
		return
	}
	h.CloseFlattenDialog()
	h.host.ActivePanel().ClearSelection()
	h.jobs.AddFlattenJob(jobsctrl.FlattenJobRequest{
		Sources: st.sources, Dest: st.dest, RemoveEmpty: st.removeEmpty, FlattenRoots: st.dirRoots,
	})
	noun := "items"
	if len(st.sources) == 1 {
		noun = "item"
	}
	h.host.SetTransientMessage(fmt.Sprintf("Flatten queued (%d %s)", len(st.sources), noun), ui.MessageUrgencyInfo)
}

func (h *Handler) finishExtractEnqueue(plan ops.ExtractPlan, skipped []string) {
	p := h.host.ActivePanel()
	p.ClearSelection()
	h.jobs.EnqueueExtractJob(ops.ExtractItemPaths(plan.Items), plan.Destination)
	n := len(plan.Items)
	noun := "archives"
	if n == 1 {
		noun = "archive"
	}
	msg := fmt.Sprintf("Extract queued (%d %s)", n, noun)
	if len(skipped) > 0 {
		msg += fmt.Sprintf("; %d skipped (unsupported or missing tool)", len(skipped))
	}
	h.host.SetTransientMessage(msg, ui.MessageUrgencyInfo)
}

func selectedPanelSources(p *panel.State) []string {
	if p == nil || len(p.SelectedPaths) == 0 {
		return nil
	}
	sources := make([]string, 0, len(p.SelectedPaths))
	for i := 0; i < p.VisibleEntryCount(); i++ {
		e, _, ok := p.VisibleEntry(i)
		if !ok || !p.SelectedPaths[e.Path] {
			continue
		}
		sources = append(sources, e.Path)
	}
	return sources
}
