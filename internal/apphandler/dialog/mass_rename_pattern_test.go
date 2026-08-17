package dialog

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// fakeMassRenamePatternHost is a minimal Host stub for mass-rename-pattern handler tests that
// don't need a real *App.
type fakeMassRenamePatternHost struct {
	cfg      config.Config
	messages []string
	errTitle string
	errMsg   string
}

func (f *fakeMassRenamePatternHost) LayoutForTerminalSize(w, h int) ui.Layout {
	return ui.Layout{Width: w, Height: h}
}
func (f *fakeMassRenamePatternHost) SetTransientMessage(text string, _ ui.MessageUrgency) {
	f.messages = append(f.messages, text)
}
func (f *fakeMassRenamePatternHost) SetErrorMessage(title string, err error) {
	f.errTitle = title
	if err != nil {
		f.errMsg = err.Error()
	}
}
func (f *fakeMassRenamePatternHost) NavigatePanelToPath(int, string, string) error { return nil }
func (f *fakeMassRenamePatternHost) ActivePanel() *panel.State                     { return &panel.State{} }
func (f *fakeMassRenamePatternHost) InactivePanel() *panel.State                   { return &panel.State{} }
func (f *fakeMassRenamePatternHost) InactivePanelID() int                          { return 1 }
func (f *fakeMassRenamePatternHost) PanelByID(int) *panel.State                    { return &panel.State{} }
func (f *fakeMassRenamePatternHost) ActiveViewportRows() int                       { return 20 }
func (f *fakeMassRenamePatternHost) PanelViewportRows(int) int                     { return 20 }
func (f *fakeMassRenamePatternHost) ClearTransientMessage()                        {}
func (f *fakeMassRenamePatternHost) Config() config.Config                         { return f.cfg }
func (f *fakeMassRenamePatternHost) Styles() theme.Theme                           { return theme.Theme{} }
func (f *fakeMassRenamePatternHost) OpenMessageDialog(string, string)              {}
func (f *fakeMassRenamePatternHost) InQuickFilterUI() bool                         { return false }
func (f *fakeMassRenamePatternHost) OpenFileInExternalEditor(string) error         { return nil }
func (f *fakeMassRenamePatternHost) ExecuteSFTPPassword()                          {}
func (f *fakeMassRenamePatternHost) HandlePathPickerScrollingQueryKey(*tcell.EventKey) bool {
	return false
}
func (f *fakeMassRenamePatternHost) SyncFilteredListRanks(lines []string, _ string, _ int, _ bool) ([]int, [][]search.Range) {
	ranked := make([]int, len(lines))
	for i := range lines {
		ranked[i] = i
	}
	return ranked, make([][]search.Range, len(lines))
}
func (f *fakeMassRenamePatternHost) ClampFilteredListSelection(selected *int, rankedLen int) {
	if rankedLen == 0 {
		*selected = 0
		return
	}
	if *selected < 0 {
		*selected = 0
	}
	if *selected >= rankedLen {
		*selected = rankedLen - 1
	}
}
func (f *fakeMassRenamePatternHost) HandleFilteredListSelectionKey(*tcell.EventKey, int, *int, int, func() int, func()) bool {
	return false
}

func newMassRenamePatternTestHandler(t *testing.T) (*Handler, *fakeMassRenamePatternHost) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(120, 40)

	fh := &fakeMassRenamePatternHost{cfg: config.Default()}
	h := &Handler{
		host:      fh,
		screen:    screen,
		model:     &ui.Model{},
		configDir: t.TempDir(),
	}
	return h, fh
}

func openMainMassRenameDialog(h *Handler) {
	h.model.FileDialog = uidialog.FileDialogState{
		Open:               true,
		DialogType:         uidialog.FileDialogMassRename,
		MassRenamePhase:    uidialog.MassRenamePhaseMain,
		MassRenameMode:     uidialog.MassRenameModeUISimple,
		MassRenameCaseFold: true,
		Fields: []uidialog.FileDialogField{
			{Label: "Find", Value: "walrus"},
			{Label: "Replace", Value: "otter"},
		},
	}
}

func TestMassRenameSavePatternFooterEligible(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	if h.MassRenameSavePatternFooterEligible() {
		t.Fatal("eligible with no dialog open")
	}
	openMainMassRenameDialog(h)
	if !h.MassRenameSavePatternFooterEligible() {
		t.Fatal("want eligible on main screen, simple mode")
	}
	h.model.FileDialog.MassRenameMode = uidialog.MassRenameModeUIExternalEditor
	if h.MassRenameSavePatternFooterEligible() {
		t.Fatal("want ineligible in external editor mode")
	}
	h.model.FileDialog.MassRenameMode = uidialog.MassRenameModeUISimple
	h.model.FileDialog.MassRenamePhase = uidialog.MassRenamePhaseSavePrompt
	if h.MassRenameSavePatternFooterEligible() {
		t.Fatal("want ineligible while save prompt already open")
	}
}

func TestMassRenameLoadPatternFooterEligible(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	if h.MassRenameLoadPatternFooterEligible() {
		t.Fatal("eligible with no dialog open")
	}
	openMainMassRenameDialog(h)
	if !h.MassRenameLoadPatternFooterEligible() {
		t.Fatal("want eligible on main screen")
	}
	h.model.FileDialog.MassRenamePhase = uidialog.MassRenamePhaseLoadPicker
	if h.MassRenameLoadPatternFooterEligible() {
		t.Fatal("want ineligible while load picker already open")
	}
}

func TestMassRenameHistoryFooterEligible(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	if h.MassRenameHistoryFooterEligible() {
		t.Fatal("eligible with no dialog open")
	}
	openMainMassRenameDialog(h)
	if !h.MassRenameHistoryFooterEligible() {
		t.Fatal("want eligible on main screen")
	}
	h.model.FileDialog.MassRenamePhase = uidialog.MassRenamePhaseHistoryPicker
	if h.MassRenameHistoryFooterEligible() {
		t.Fatal("want ineligible while history picker already open")
	}
}

func TestMassRenameDeletePatternFooterEligible(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	if h.MassRenameDeletePatternFooterEligible() {
		t.Fatal("want ineligible on main screen")
	}
	h.model.FileDialog.MassRenamePhase = uidialog.MassRenamePhaseLoadPicker
	if h.MassRenameDeletePatternFooterEligible() {
		t.Fatal("want ineligible with empty ranked list")
	}
	h.model.FileDialog.MassRenameLoadPicker = uidialog.MassRenamePatternPickerState{
		Items:  []ops.MassRenamePattern{{Name: "harbor"}},
		Ranked: []int{0},
	}
	if !h.MassRenameDeletePatternFooterEligible() {
		t.Fatal("want eligible with a ranked selection in the load picker")
	}

	h.model.FileDialog.MassRenamePhase = uidialog.MassRenamePhaseHistoryPicker
	if h.MassRenameDeletePatternFooterEligible() {
		t.Fatal("want ineligible with empty ranked list in the history picker")
	}
	h.model.FileDialog.MassRenameHistoryPicker = uidialog.MassRenamePatternPickerState{
		Items:  []ops.MassRenamePattern{{Find: "a", Replace: "b"}},
		Ranked: []int{0},
	}
	if !h.MassRenameDeletePatternFooterEligible() {
		t.Fatal("want eligible with a ranked selection in the history picker")
	}
}

func TestOpenAndCloseMassRenameSavePrompt(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	originalFields := h.model.FileDialog.Fields

	h.openMassRenameSavePrompt()
	d := &h.model.FileDialog
	if d.MassRenamePhase != uidialog.MassRenamePhaseSavePrompt {
		t.Fatalf("phase = %v, want SavePrompt", d.MassRenamePhase)
	}
	if len(d.Fields) != 2 || d.Fields[0].Label != "Name" || d.Fields[1].Label != "Description" {
		t.Fatalf("Fields = %+v, want Name/Description", d.Fields)
	}
	if len(d.MassRenameSavedFields) != 2 || d.MassRenameSavedFields[0].Value != "walrus" {
		t.Fatalf("MassRenameSavedFields not stashed: %+v", d.MassRenameSavedFields)
	}

	h.closeMassRenameSavePrompt()
	if d.MassRenamePhase != uidialog.MassRenamePhaseMain {
		t.Fatalf("phase after close = %v, want Main", d.MassRenamePhase)
	}
	if len(d.Fields) != 2 || d.Fields[0].Value != originalFields[0].Value || d.Fields[1].Value != originalFields[1].Value {
		t.Fatalf("Fields not restored: %+v", d.Fields)
	}
}

func TestConfirmMassRenameSavePromptRejectsEmptyName(t *testing.T) {
	h, fh := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	h.openMassRenameSavePrompt()
	h.model.FileDialog.Fields[0].Value = "   "

	h.confirmMassRenameSavePrompt()

	if h.model.FileDialog.MassRenamePhase != uidialog.MassRenamePhaseSavePrompt {
		t.Fatal("phase should stay SavePrompt when name is empty")
	}
	if len(fh.messages) == 0 {
		t.Fatal("want a warning message for empty name")
	}
}

func TestConfirmMassRenameSavePromptUpsertsAndRestores(t *testing.T) {
	h, fh := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	h.openMassRenameSavePrompt()
	h.model.FileDialog.Fields[0].Value = "harbor"
	h.model.FileDialog.Fields[1].Value = "strip prefix"

	h.confirmMassRenameSavePrompt()

	d := &h.model.FileDialog
	if d.MassRenamePhase != uidialog.MassRenamePhaseMain {
		t.Fatalf("phase = %v, want Main", d.MassRenamePhase)
	}
	if len(d.Fields) != 2 || d.Fields[0].Label != "Find" {
		t.Fatalf("Fields not restored: %+v", d.Fields)
	}
	found := false
	for _, m := range fh.messages {
		if m == "Pattern saved: harbor" {
			found = true
		}
	}
	if !found {
		t.Fatalf("messages = %v, want Pattern saved: harbor", fh.messages)
	}

	path := ops.MassRenamePatternsResolveFile("", h.configDir)
	patterns, err := ops.LoadMassRenamePatterns(path)
	if err != nil {
		t.Fatalf("LoadMassRenamePatterns: %v", err)
	}
	if len(patterns) != 1 || patterns[0].Name != "harbor" || patterns[0].Find != "walrus" || patterns[0].Replace != "otter" {
		t.Fatalf("saved patterns = %+v", patterns)
	}
}

func TestOpenMassRenameLoadPickerLoadsAndRanks(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	path := ops.MassRenamePatternsResolveFile("", h.configDir)
	seed := []ops.MassRenamePattern{
		{Name: "harbor", Description: "strip prefix", Mode: "simple", Find: "IMG_", Replace: ""},
		{Name: "lantern", Description: "regex cleanup", Mode: "regex", Find: `\d+`, Replace: "#"},
	}
	if err := ops.SaveMassRenamePatterns(path, seed); err != nil {
		t.Fatalf("SaveMassRenamePatterns: %v", err)
	}

	h.openMassRenameLoadPicker()

	d := &h.model.FileDialog
	if d.MassRenamePhase != uidialog.MassRenamePhaseLoadPicker {
		t.Fatalf("phase = %v, want LoadPicker", d.MassRenamePhase)
	}
	if len(d.MassRenameLoadPicker.Items) != 2 {
		t.Fatalf("Items = %+v, want 2 loaded", d.MassRenameLoadPicker.Items)
	}
	if len(d.MassRenameLoadPicker.Ranked) != 2 {
		t.Fatalf("Ranked = %+v, want 2", d.MassRenameLoadPicker.Ranked)
	}
}

func TestActivateMassRenameLoadPickerSelectionAppliesPattern(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	path := ops.MassRenamePatternsResolveFile("", h.configDir)
	seed := []ops.MassRenamePattern{
		{Name: "lantern", Description: "regex cleanup", Mode: "regex", Find: `\d+`, Replace: "#", CaseFold: true},
	}
	if err := ops.SaveMassRenamePatterns(path, seed); err != nil {
		t.Fatalf("SaveMassRenamePatterns: %v", err)
	}
	h.openMassRenameLoadPicker()
	h.model.FileDialog.MassRenameLoadPicker.Selected = 0

	h.activateMassRenamePickerSelection()

	d := &h.model.FileDialog
	if d.MassRenamePhase != uidialog.MassRenamePhaseMain {
		t.Fatalf("phase = %v, want Main", d.MassRenamePhase)
	}
	if d.MassRenameMode != uidialog.MassRenameModeUIRegex {
		t.Fatalf("mode = %v, want Regex", d.MassRenameMode)
	}
	if d.Fields[0].Value != `\d+` || d.Fields[1].Value != "#" {
		t.Fatalf("Fields = %+v, want pattern/replacement restored", d.Fields)
	}
	if !d.MassRenameCaseFold {
		t.Fatal("CaseFold not restored")
	}
}

func TestDeleteSelectedMassRenamePatternSplicesAndSaves(t *testing.T) {
	h, fh := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	path := ops.MassRenamePatternsResolveFile("", h.configDir)
	seed := []ops.MassRenamePattern{
		{Name: "harbor", Description: "a"},
		{Name: "lantern", Description: "b"},
	}
	if err := ops.SaveMassRenamePatterns(path, seed); err != nil {
		t.Fatalf("SaveMassRenamePatterns: %v", err)
	}
	h.openMassRenameLoadPicker()
	h.model.FileDialog.MassRenameLoadPicker.Selected = 0

	if !h.deleteSelectedMassRenamePattern() {
		t.Fatal("deleteSelectedMassRenamePattern returned false")
	}

	if len(h.model.FileDialog.MassRenameLoadPicker.Items) != 1 || h.model.FileDialog.MassRenameLoadPicker.Items[0].Name != "lantern" {
		t.Fatalf("Items = %+v, want only lantern", h.model.FileDialog.MassRenameLoadPicker.Items)
	}
	onDisk, err := ops.LoadMassRenamePatterns(path)
	if err != nil {
		t.Fatalf("LoadMassRenamePatterns: %v", err)
	}
	if len(onDisk) != 1 || onDisk[0].Name != "lantern" {
		t.Fatalf("on-disk patterns = %+v, want only lantern", onDisk)
	}
	found := false
	for _, m := range fh.messages {
		if m == "Pattern removed: harbor" {
			found = true
		}
	}
	if !found {
		t.Fatalf("messages = %v, want Pattern removed: harbor", fh.messages)
	}
}

func TestRecordMassRenameHistoryDedupAndCap(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)

	for range maxMassRenameHistory + 5 {
		h.recordMassRenameHistory(h.massRenameCurrentPattern("", "", "find", "replace"))
	}
	if len(h.massRenameHistory) != 1 {
		t.Fatalf("history = %+v, want deduped to 1 entry", h.massRenameHistory)
	}

	for i := range maxMassRenameHistory + 5 {
		find := string(rune('a' + i%26))
		h.recordMassRenameHistory(h.massRenameCurrentPattern("", "", find, "replace"))
	}
	if len(h.massRenameHistory) != maxMassRenameHistory {
		t.Fatalf("history len = %d, want cap %d", len(h.massRenameHistory), maxMassRenameHistory)
	}
	if h.massRenameHistory[0].Find != "y" {
		t.Fatalf("history[0].Find = %q, want most-recent %q", h.massRenameHistory[0].Find, "y")
	}
}

func TestOpenMassRenameHistoryPickerLoadsAndRanks(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	h.recordMassRenameHistory(h.massRenameCurrentPattern("", "", "IMG_", ""))
	h.recordMassRenameHistory(h.massRenameCurrentPattern("", "", "DSC_", "photo_"))

	h.openMassRenameHistoryPicker()

	d := &h.model.FileDialog
	if d.MassRenamePhase != uidialog.MassRenamePhaseHistoryPicker {
		t.Fatalf("phase = %v, want HistoryPicker", d.MassRenamePhase)
	}
	if len(d.MassRenameHistoryPicker.Items) != 2 {
		t.Fatalf("Items = %+v, want 2 loaded", d.MassRenameHistoryPicker.Items)
	}
	if len(d.MassRenameHistoryPicker.Ranked) != 2 {
		t.Fatalf("Ranked = %+v, want 2", d.MassRenameHistoryPicker.Ranked)
	}
	// Most-recent-first: DSC_ was recorded last.
	if d.MassRenameHistoryPicker.Items[0].Find != "DSC_" {
		t.Fatalf("Items[0] = %+v, want the most-recently recorded entry first", d.MassRenameHistoryPicker.Items[0])
	}
}

func TestActivateMassRenamePickerSelectionAppliesHistoryEntry(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	h.recordMassRenameHistory(h.massRenameCurrentPattern("", "", "DSC_", "photo_"))
	h.openMassRenameHistoryPicker()
	h.model.FileDialog.MassRenameHistoryPicker.Selected = 0

	h.activateMassRenamePickerSelection()

	d := &h.model.FileDialog
	if d.MassRenamePhase != uidialog.MassRenamePhaseMain {
		t.Fatalf("phase = %v, want Main", d.MassRenamePhase)
	}
	if d.Fields[0].Value != "DSC_" || d.Fields[1].Value != "photo_" {
		t.Fatalf("Fields = %+v, want history entry restored", d.Fields)
	}
}

func TestDeleteSelectedMassRenamePatternRemovesHistoryEntryInMemoryOnly(t *testing.T) {
	h, fh := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	path := ops.MassRenamePatternsResolveFile("", h.configDir)
	saved := []ops.MassRenamePattern{{Name: "harbor", Description: "a"}}
	if err := ops.SaveMassRenamePatterns(path, saved); err != nil {
		t.Fatalf("SaveMassRenamePatterns: %v", err)
	}
	h.recordMassRenameHistory(h.massRenameCurrentPattern("", "", "DSC_", "photo_"))
	h.openMassRenameHistoryPicker()
	h.model.FileDialog.MassRenameHistoryPicker.Selected = 0

	if !h.deleteSelectedMassRenamePattern() {
		t.Fatal("deleteSelectedMassRenamePattern returned false")
	}

	if len(h.massRenameHistory) != 0 {
		t.Fatalf("massRenameHistory = %+v, want emptied", h.massRenameHistory)
	}
	if len(h.model.FileDialog.MassRenameHistoryPicker.Items) != 0 {
		t.Fatalf("Items = %+v, want emptied", h.model.FileDialog.MassRenameHistoryPicker.Items)
	}
	onDisk, err := ops.LoadMassRenamePatterns(path)
	if err != nil {
		t.Fatalf("LoadMassRenamePatterns: %v", err)
	}
	if len(onDisk) != 1 || onDisk[0].Name != "harbor" {
		t.Fatalf("on-disk patterns = %+v, want untouched (history delete never touches patterns.toml)", onDisk)
	}
	found := false
	for _, m := range fh.messages {
		if m == "Removed from history" {
			found = true
		}
	}
	if !found {
		t.Fatalf("messages = %v, want %q", fh.messages, "Removed from history")
	}
}

func TestRecomputeMassRenamePreviewMatchCount(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	openMainMassRenameDialog(h)
	d := &h.model.FileDialog
	d.MassRenameSources = []uidialog.MassRenameSource{
		{Path: "/tmp/foo1.txt", Name: "foo1.txt"},
		{Path: "/tmp/foo2.txt", Name: "foo2.txt"},
		{Path: "/tmp/bar.txt", Name: "bar.txt"},
	}
	d.Fields[0].Value = "foo"
	d.Fields[1].Value = "baz"

	h.RecomputeMassRenamePreview()

	if d.MassRenameMatchCount != 2 {
		t.Fatalf("MassRenameMatchCount = %d, want 2", d.MassRenameMatchCount)
	}

	// "Show only modified" filters the preview rows but must not change the match count.
	d.MassRenameShowOnlyModified = true
	h.RecomputeMassRenamePreview()
	if d.MassRenameMatchCount != 2 {
		t.Fatalf("MassRenameMatchCount with ShowOnlyModified = %d, want 2", d.MassRenameMatchCount)
	}
}

func TestMassRenamePatternsPathUsesConfigDir(t *testing.T) {
	h, _ := newMassRenamePatternTestHandler(t)
	got := h.massRenamePatternsPath()
	want := filepath.Join(h.configDir, "patterns.toml")
	if got != want {
		t.Fatalf("massRenamePatternsPath() = %q, want %q", got, want)
	}
}
