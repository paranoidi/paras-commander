package dialog

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

const (
	validateQueryA = "/wandering/turnip"
	validateQueryB = "/hedgehog/lantern"
)

// delayedPathExists is a test fake for path-existence checks. Each query can block until
// release is called, so a slower check A can finish after a newer check B has been armed.
type delayedPathExists struct {
	mu      sync.Mutex
	invalid map[string]bool
	started map[string]chan struct{}
	block   map[string]chan struct{}
}

func newDelayedPathExists() *delayedPathExists {
	return &delayedPathExists{
		invalid: map[string]bool{},
		started: map[string]chan struct{}{},
		block:   map[string]chan struct{}{},
	}
}

func (d *delayedPathExists) set(query string, invalid, hold bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.invalid[query] = invalid
	d.started[query] = make(chan struct{})
	if hold {
		d.block[query] = make(chan struct{})
	}
}

func (d *delayedPathExists) waitStarted(t *testing.T, query string) {
	t.Helper()
	d.mu.Lock()
	ch := d.started[query]
	d.mu.Unlock()
	if ch == nil {
		t.Fatalf("no started channel for %q", query)
	}
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for existence check of %q to start", query)
	}
}

func (d *delayedPathExists) release(query string) {
	d.mu.Lock()
	ch := d.block[query]
	d.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (d *delayedPathExists) check(_ string, _ string, raw string) bool {
	d.mu.Lock()
	started := d.started[raw]
	block := d.block[raw]
	invalid := d.invalid[raw]
	d.mu.Unlock()
	if started != nil {
		select {
		case <-started:
		default:
			close(started)
		}
	}
	if block != nil {
		<-block
	}
	return invalid
}

func newPathValidateHandler(t *testing.T, fake *delayedPathExists) (*Handler, tcell.SimulationScreen) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	cfg := config.Default()
	cfg.UI.PathPickerValidateDelayMS = 0
	fh := &fakeMassRenamePatternHost{cfg: cfg}
	h := &Handler{
		host:         fh,
		screen:       screen,
		model:        &ui.Model{},
		pathExistsFn: fake.check,
	}
	return h, screen
}

func waitInterruptData(t *testing.T, screen tcell.SimulationScreen) any {
	t.Helper()
	done := make(chan tcell.Event, 1)
	go func() { done <- screen.PollEvent() }()
	select {
	case ev := <-done:
		interruptEv, ok := ev.(*tcell.EventInterrupt)
		if !ok {
			t.Fatalf("PollEvent returned %T, want *tcell.EventInterrupt", ev)
		}
		return interruptEv.Data()
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for validate interrupt")
	}
	return nil
}

func applyNextPathPickerValidate(t *testing.T, h *Handler, screen tcell.SimulationScreen) {
	t.Helper()
	d, ok := waitInterruptData(t, screen).(PathPickerValidatePayload)
	if !ok {
		t.Fatal("expected PathPickerValidatePayload")
	}
	h.ApplyPathPickerValidatePayload(d)
}

func applyNextTransferDestValidate(t *testing.T, h *Handler, screen tcell.SimulationScreen) {
	t.Helper()
	d, ok := waitInterruptData(t, screen).(TransferDestValidatePayload)
	if !ok {
		t.Fatal("expected TransferDestValidatePayload")
	}
	h.ApplyTransferDestValidatePayload(d)
}

func TestPathPickerValidateQueryBSupersedesA(t *testing.T) {
	fake := newDelayedPathExists()
	fake.set(validateQueryA, true, true)
	fake.set(validateQueryB, false, true)
	h, screen := newPathValidateHandler(t, fake)

	h.model.PathPicker.Open = true
	h.model.PathPicker.Query = validateQueryA
	h.ArmPathPickerValidateTimer()
	fake.waitStarted(t, validateQueryA)

	h.model.PathPicker.Query = validateQueryB
	h.ArmPathPickerValidateTimer()
	fake.waitStarted(t, validateQueryB)

	fake.release(validateQueryA)
	applyNextPathPickerValidate(t, h, screen)
	if h.model.PathPicker.QueryPathInvalid {
		t.Fatal("stale check A must not mark query B invalid")
	}
	if !h.model.PathPicker.QueryPathCheckPending {
		t.Fatal("pending must stay set until check B is applied")
	}

	fake.release(validateQueryB)
	applyNextPathPickerValidate(t, h, screen)
	if h.model.PathPicker.QueryPathInvalid {
		t.Fatal("query B exists; QueryPathInvalid should be false")
	}
	if h.model.PathPicker.QueryPathCheckPending {
		t.Fatal("pending should clear when check B is applied")
	}
}

func TestPathPickerValidateCloseReopenDropsStale(t *testing.T) {
	fake := newDelayedPathExists()
	fake.set(validateQueryA, true, true)
	h, screen := newPathValidateHandler(t, fake)

	h.model.PathPicker.Open = true
	h.model.PathPicker.Query = validateQueryA
	h.ArmPathPickerValidateTimer()
	fake.waitStarted(t, validateQueryA)

	h.ClosePathPicker()
	h.model.PathPicker.Open = true
	h.model.PathPicker.Query = validateQueryA

	fake.release(validateQueryA)
	applyNextPathPickerValidate(t, h, screen)
	if h.model.PathPicker.QueryPathInvalid {
		t.Fatal("closed-then-reopened picker must ignore the in-flight check from the previous open")
	}
}

func TestPathPickerValidateTimerDoesNotRaceModel(t *testing.T) {
	fake := newDelayedPathExists()
	fake.set(validateQueryA, true, true)
	h, screen := newPathValidateHandler(t, fake)

	h.model.PathPicker.Open = true
	h.model.PathPicker.Query = validateQueryA
	h.ArmPathPickerValidateTimer()
	fake.waitStarted(t, validateQueryA)

	for i := 0; i < 1000; i++ {
		_ = h.model.PathPicker.QueryPathInvalid
		_ = h.model.PathPicker.QueryPathCheckPending
		h.model.PathPicker.Query = validateQueryA
		runtime.Gosched()
	}

	fake.release(validateQueryA)
	applyNextPathPickerValidate(t, h, screen)
	if !h.model.PathPicker.QueryPathInvalid {
		t.Fatal("expected invalid after applying check A")
	}
}

func TestTransferDestValidateQueryBSupersedesA(t *testing.T) {
	fake := newDelayedPathExists()
	fake.set(validateQueryA, true, true)
	fake.set(validateQueryB, false, true)
	h, screen := newPathValidateHandler(t, fake)

	h.model.TransferDialog.Open = true
	h.model.TransferDialog.Phase = uidialog.TransferPhaseDestination
	h.model.TransferDialog.Destination.Value = validateQueryA
	h.ArmTransferDestinationValidateTimer()
	fake.waitStarted(t, validateQueryA)

	h.model.TransferDialog.Destination.Value = validateQueryB
	h.ArmTransferDestinationValidateTimer()
	fake.waitStarted(t, validateQueryB)

	fake.release(validateQueryA)
	applyNextTransferDestValidate(t, h, screen)
	if h.model.TransferDialog.DestPathInvalid {
		t.Fatal("stale check A must not mark destination B invalid")
	}
	if !h.model.TransferDialog.DestPathCheckPending {
		t.Fatal("pending must stay set until check B is applied")
	}

	fake.release(validateQueryB)
	applyNextTransferDestValidate(t, h, screen)
	if h.model.TransferDialog.DestPathInvalid {
		t.Fatal("destination B exists; DestPathInvalid should be false")
	}
	if h.model.TransferDialog.DestPathCheckPending {
		t.Fatal("pending should clear when check B is applied")
	}
}

func TestTransferDestValidateCloseReopenDropsStale(t *testing.T) {
	fake := newDelayedPathExists()
	fake.set(validateQueryA, true, true)
	h, screen := newPathValidateHandler(t, fake)

	h.model.TransferDialog.Open = true
	h.model.TransferDialog.Phase = uidialog.TransferPhaseDestination
	h.model.TransferDialog.Destination.Value = validateQueryA
	h.ArmTransferDestinationValidateTimer()
	fake.waitStarted(t, validateQueryA)

	h.CloseTransferDialog()
	h.model.TransferDialog.Open = true
	h.model.TransferDialog.Phase = uidialog.TransferPhaseDestination
	h.model.TransferDialog.Destination.Value = validateQueryA

	fake.release(validateQueryA)
	applyNextTransferDestValidate(t, h, screen)
	if h.model.TransferDialog.DestPathInvalid {
		t.Fatal("closed-then-reopened transfer dialog must ignore the in-flight check from the previous open")
	}
}

func TestFlattenDestValidateQueryBSupersedesA(t *testing.T) {
	fake := newDelayedPathExists()
	fake.set(validateQueryA, true, true)
	fake.set(validateQueryB, false, true)
	h, screen := newPathValidateHandler(t, fake)

	h.model.FlattenDialog.Open = true
	h.model.FlattenDialog.Destination.Value = validateQueryA
	h.ArmFlattenDestinationValidateTimer()
	fake.waitStarted(t, validateQueryA)

	h.model.FlattenDialog.Destination.Value = validateQueryB
	h.ArmFlattenDestinationValidateTimer()
	fake.waitStarted(t, validateQueryB)

	fake.release(validateQueryA)
	applyNextTransferDestValidate(t, h, screen)
	if h.model.FlattenDialog.DestPathInvalid {
		t.Fatal("stale check A must not mark flatten destination B invalid")
	}
	if !h.model.FlattenDialog.DestPathCheckPending {
		t.Fatal("pending must stay set until check B is applied")
	}

	fake.release(validateQueryB)
	applyNextTransferDestValidate(t, h, screen)
	if h.model.FlattenDialog.DestPathInvalid {
		t.Fatal("flatten destination B exists; DestPathInvalid should be false")
	}
	if h.model.FlattenDialog.DestPathCheckPending {
		t.Fatal("pending should clear when check B is applied")
	}
}

func TestFlattenDestValidateCloseReopenDropsStale(t *testing.T) {
	fake := newDelayedPathExists()
	fake.set(validateQueryA, true, true)
	h, screen := newPathValidateHandler(t, fake)

	h.model.FlattenDialog.Open = true
	h.model.FlattenDialog.Destination.Value = validateQueryA
	h.ArmFlattenDestinationValidateTimer()
	fake.waitStarted(t, validateQueryA)

	h.CloseFlattenDialog()
	h.model.FlattenDialog.Open = true
	h.model.FlattenDialog.Destination.Value = validateQueryA

	fake.release(validateQueryA)
	applyNextTransferDestValidate(t, h, screen)
	if h.model.FlattenDialog.DestPathInvalid {
		t.Fatal("closed-then-reopened flatten dialog must ignore the in-flight check from the previous open")
	}
}
