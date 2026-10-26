package jobs

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/sched"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// opsViewKey identifies what the Operations window lists: a job's log, a result mask and a
// fuzzy query.
type opsViewKey struct {
	log   *oplog.Log
	mask  oplog.Mask
	query string
	ci    bool
}

// opsState is the Operations window's background work: a fuzzy scan of the current query and
// a fetch of rows that live only in a log's spill file. Goroutines write results under mu and
// post a jobs wake; RefreshOpsWindow reads them on the UI thread.
type opsState struct {
	mu sync.Mutex

	scanKey     opsViewKey
	scanMatches []uint32 // ascending record numbers
	scanUpto    uint32   // records scanned
	scanRunning bool
	scanFailed  bool
	scanCancel  context.CancelFunc

	fetchRunning bool
	doneLog      *oplog.Log // last finished fetch
	doneNums     []uint32
	doneRecs     []oplog.Record

	// UI-thread only: what the current window was built from, when, and the trailing wake that
	// shows rows held back by the refresh rate limit.
	lastInputs opsInputs
	lastBuilt  time.Time
	trailing   sched.Debouncer
}

// opsInputs is everything a user action changes about the Operations window. A change rebuilds
// the window at once; job activity alone rebuilds it at most every
// [jobs].operations_refresh_interval_ms.
type opsInputs struct {
	jobID        string
	key          opsViewKey
	ready        bool // the window was built at least once
	scroll, rows int
}

// RefreshOpsWindow prepares Model.JobsOpsWindow, the rows the Operations pane draws, for the
// selected job and the view's filters and scroll. It runs on the UI thread every frame while
// the jobs view is open and never blocks on disk: rows held in memory are copied directly,
// rows only in a spill file and fuzzy matching run in goroutines that wake the UI when done.
//
// Rebuilds driven only by job activity (new rows, scan progress) are rate limited to
// [jobs].operations_refresh_interval_ms so the list stays readable; a trailing wake shows what
// was held back. A changed job, filter, scroll or pane size, and rows still pending a spill
// read, rebuild at once.
func (h *Handler) RefreshOpsWindow() {
	h.refreshOpsWindow(false)
}

func (h *Handler) refreshOpsWindow(force bool) {
	if h.model.ViewMode != ui.ViewJobs {
		h.stopOpsWork()
		return
	}
	sel := h.selectedJobEntry()
	win := ui.JobsOpsWindow{JobID: sel.ID}
	if sel.Ops == nil {
		h.model.JobsOpsWindow = win
		h.ops.lastInputs = opsInputs{}
		return
	}
	st := &h.model.JobsView
	key := opsViewKey{log: sel.Ops, mask: st.OpsResultShown().Mask(), query: st.OpsFilter.Query, ci: h.model.JobsFilterCaseInsensitive}
	rows := h.opsContentRows()
	inputs := opsInputs{jobID: sel.ID, key: key, ready: true, scroll: st.OpsScroll, rows: rows}
	if !force && h.opsThrottled(inputs) {
		return
	}
	query := search.Parse(key.query)

	var nums []uint32
	if query.Empty() {
		h.stopScan()
		win.Total = key.log.ViewLen(key.mask)
		st.OpsScroll = min(max(st.OpsScroll, 0), ui.JobsOpsMaxScroll(win.Total, rows))
		nums = key.log.ViewNums(key.mask, st.OpsScroll, rows)
	} else {
		var matches []uint32
		matches, win.Scanning = h.scanFor(key)
		win.Total = len(matches)
		st.OpsScroll = min(max(st.OpsScroll, 0), ui.JobsOpsMaxScroll(win.Total, rows))
		for i := st.OpsScroll; i < st.OpsScroll+rows && i < win.Total; i++ {
			nums = append(nums, matches[win.Total-1-i])
		}
	}
	win.Offset = st.OpsScroll
	recs, ok := key.log.RecordsInMemory(nums)
	if !ok {
		recs, ok = h.fetchFor(key.log, nums)
	}
	opts := search.Options{CaseInsensitive: key.ci}
	win.Rows = make([]ui.JobOperationRow, len(nums))
	for i := range nums {
		if !ok {
			win.Rows[i].Pending = true
			continue
		}
		win.Rows[i].Op = ui.JobOperationFromRecord(recs[i])
		if !query.Empty() {
			win.Rows[i].Ranges = query.Match(recs[i].Label, opts).Ranges
		}
	}
	h.model.JobsOpsWindow = win
	inputs.scroll = st.OpsScroll // after clamping, so the next frame sees the same inputs
	h.ops.lastInputs, h.ops.lastBuilt = inputs, time.Now()
}

// opsThrottled reports whether the window must keep its current rows: inputs are unchanged, no
// row is waiting for a spill read and the refresh interval has not passed since the last build.
// It then arms the trailing wake (once) for when the interval ends.
func (h *Handler) opsThrottled(inputs opsInputs) bool {
	o := &h.ops
	interval := time.Duration(h.config.Jobs.OperationsRefreshIntervalMS) * time.Millisecond
	if interval <= 0 || inputs != o.lastInputs || opsWindowPending(h.model.JobsOpsWindow) {
		return false
	}
	wait := interval - time.Since(o.lastBuilt)
	if wait <= 0 {
		return false
	}
	if !o.trailing.Armed() {
		o.trailing.Arm(wait, h.postOpsWake)
	}
	return true
}

func opsWindowPending(win ui.JobsOpsWindow) bool {
	for _, r := range win.Rows {
		if r.Pending {
			return true
		}
	}
	return false
}

// opsContentRows is how many rows the Operations pane shows: the maximized pane's or the split
// pane's content height.
func (h *Handler) opsContentRows() int {
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if h.model.JobsView.Maximized {
		return ui.JobsPanelContentRows(ui.JobsMaximizedRect(layout))
	}
	_, rects := h.jobsPaneLayout()
	return ui.JobsPanelContentRows(rects[ui.JobsPaneOperations])
}

// scanFor returns the fuzzy matches of key found so far and whether a scan is still running. It
// starts a scan when key changed, or continues the last one over records appended since.
func (h *Handler) scanFor(key opsViewKey) ([]uint32, bool) {
	o := &h.ops
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.scanKey != key {
		if o.scanCancel != nil {
			o.scanCancel()
		}
		o.scanKey, o.scanMatches, o.scanUpto, o.scanRunning, o.scanFailed, o.scanCancel = key, nil, 0, false, false, nil
	}
	if !o.scanRunning && !o.scanFailed && key.log.Len() > o.scanUpto {
		h.startScanLocked(key, o.scanUpto)
	}
	return o.scanMatches, o.scanRunning
}

// startScanLocked runs key's scan from record from in a goroutine. o.mu must be held.
func (h *Handler) startScanLocked(key opsViewKey, from uint32) {
	o := &h.ops
	ctx, cancel := context.WithCancel(context.Background())
	o.scanRunning, o.scanCancel = true, cancel
	go func() {
		current := func() bool { return o.scanKey == key && ctx.Err() == nil }
		upto, err := key.log.Scan(ctx, from, key.mask, search.Parse(key.query), search.Options{CaseInsensitive: key.ci}, func(batch []uint32) {
			o.mu.Lock()
			if current() {
				o.scanMatches = append(o.scanMatches, batch...)
			}
			o.mu.Unlock()
			h.postOpsWake()
		})
		o.mu.Lock()
		if current() {
			o.scanRunning, o.scanUpto, o.scanFailed = false, upto, err != nil
		}
		o.mu.Unlock()
		h.postOpsWake()
	}()
}

// fetchFor returns log's records nums when a background read of exactly them has finished.
// Otherwise it starts one, unless a read is already running (its finish wakes the UI, and the
// next frame asks for the rows it then needs), and reports ok=false.
func (h *Handler) fetchFor(log *oplog.Log, nums []uint32) ([]oplog.Record, bool) {
	o := &h.ops
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.doneLog == log && slices.Equal(o.doneNums, nums) {
		return o.doneRecs, true
	}
	if o.fetchRunning {
		return nil, false
	}
	o.fetchRunning = true
	want := slices.Clone(nums)
	go func() {
		recs, err := log.ReadRecords(want)
		o.mu.Lock()
		o.fetchRunning = false
		if err == nil {
			o.doneLog, o.doneNums, o.doneRecs = log, want, recs
		}
		o.mu.Unlock()
		h.postOpsWake()
	}()
	return nil, false
}

func (h *Handler) stopScan() {
	o := &h.ops
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.scanCancel != nil {
		o.scanCancel()
	}
	o.scanKey, o.scanMatches, o.scanUpto, o.scanRunning, o.scanFailed, o.scanCancel = opsViewKey{}, nil, 0, false, false, nil
}

// stopOpsWork cancels the Operations scan and forgets fetched rows (jobs view closed).
func (h *Handler) stopOpsWork() {
	h.stopScan()
	o := &h.ops
	o.mu.Lock()
	o.doneLog, o.doneNums, o.doneRecs = nil, nil, nil
	o.mu.Unlock()
	o.trailing.Invalidate()
	o.lastInputs = opsInputs{}
	h.model.JobsOpsWindow = ui.JobsOpsWindow{}
}

// postOpsWake asks the UI loop to repaint (and so RefreshOpsWindow) after background work.
func (h *Handler) postOpsWake() {
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(OpsWakePayload{}))
}

// operationsFocused reports whether the Operations pane has focus.
func (h *Handler) operationsFocused() bool {
	order, _ := h.jobsPaneLayout()
	fp := h.model.JobsView.FocusPane
	return fp >= 0 && fp < len(order) && order[fp] == ui.JobsPaneOperations
}

// plainRune reports an unmodified (or shifted) printable key, the keys that type into the
// Operations fuzzy filter.
func plainRune(event *tcell.EventKey) bool {
	return event.Key() == tcell.KeyRune && event.Modifiers()&^tcell.ModShift == 0
}

// handleOperationsFilterKey edits an open Operations fuzzy filter (query set or editing, pane
// focused): printable runes type, Backspace deletes, Ctrl+Backspace and Ctrl+L clear, Esc closes
// the filter. It reports whether the key was consumed; scrolling and actions fall through.
func (h *Handler) handleOperationsFilterKey(event *tcell.EventKey) bool {
	f := &h.model.JobsView.OpsFilter
	if f.Query == "" && !f.Editing {
		return false
	}
	switch event.Key() {
	case tcell.KeyEsc:
		h.setOperationsQuery(quickfilter.Filter{})
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if event.Modifiers()&tcell.ModCtrl != 0 {
			h.clearOperationsQuery()
			return true
		}
		if next, changed := f.Backspace(); changed { // an empty query closes the filter
			f.Query = next
			h.model.JobsView.OpsScroll = 0
		}
	case tcell.KeyCtrlL:
		h.clearOperationsQuery()
	case tcell.KeyRune:
		if !plainRune(event) {
			return false
		}
		h.typeOperationsRune(event.Rune())
	default:
		return false
	}
	return true
}

// typeOperationsRune types r into the Operations fuzzy filter, opening it if needed.
func (h *Handler) typeOperationsRune(r rune) {
	f := &h.model.JobsView.OpsFilter
	f.Query = f.InsertRune(r)
	h.model.JobsView.OpsScroll = 0
}

// clearOperationsQuery empties the fuzzy query while the filter stays open for typing.
func (h *Handler) clearOperationsQuery() {
	h.setOperationsQuery(quickfilter.Filter{Editing: true})
}

func (h *Handler) setOperationsQuery(f quickfilter.Filter) {
	h.model.JobsView.OpsFilter = f
	h.model.JobsView.OpsScroll = 0
}

// ResetOperationsFilter shows every result and drops the fuzzy query.
func (h *Handler) ResetOperationsFilter() {
	h.model.JobsView.OpsResultHidden = dialog.JobResultNone
	h.setOperationsQuery(quickfilter.Filter{})
}

// OpenResultFilterDialog opens the Operations result picker on the current result filter.
func (h *Handler) OpenResultFilterDialog() {
	h.model.JobsResultFilterDialog = dialog.JobsResultFilterDialogState{
		Open:   true,
		Filter: h.model.JobsView.OpsResultShown(),
	}
}

// HandleResultFilterDialogKey feeds a key to the open result picker.
func (h *Handler) HandleResultFilterDialogKey(event *tcell.EventKey) {
	d := &h.model.JobsResultFilterDialog
	if ev, ok := dlg.HandleKey(dialog.JobsResultFilterView(*d), &d.Focus, event); ok {
		h.ApplyResultFilterDialogEvent(ev)
	}
}

// ApplyResultFilterDialogEvent acts on a widget event of the result picker (key or click).
func (h *Handler) ApplyResultFilterDialogEvent(ev dlg.Event) {
	switch ev.ID {
	case dlg.OK:
		h.ResultFilterDialogOK()
	case dlg.Cancel:
		h.ResultFilterDialogCancel()
	default:
		h.model.JobsResultFilterDialog.Apply(ev)
	}
}

// ResultFilterDialogOK applies the picked results to the Operations pane.
func (h *Handler) ResultFilterDialogOK() {
	h.model.JobsView.OpsResultHidden = dialog.JobResultAll &^ h.model.JobsResultFilterDialog.Filter
	h.model.JobsView.OpsScroll = 0
	h.ResultFilterDialogCancel()
}

// ResultFilterDialogCancel closes the result picker without applying it.
func (h *Handler) ResultFilterDialogCancel() {
	h.model.JobsResultFilterDialog = dialog.JobsResultFilterDialogState{}
}
