package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// TestDialogLabelsAreFollowedByContentRow guards the project's dialog layout standard: a label
// row is followed directly by the input/value it labels, with no blank row between. Covers the
// dialogs that drew their content directly under the label; add a case here when a new labelled
// row appears.
func TestDialogLabelsAreFollowedByContentRow(t *testing.T) {
	const w, h = 100, 40
	layout := Layout{Width: w, Height: h}
	styles := theme.Default()

	cases := []struct {
		name   string
		draw   func(screen tcell.Screen)
		labels []string
	}{
		{
			name: "add bookmark",
			draw: func(screen tcell.Screen) {
				DrawFileDialog(screen, layout, FileDialogState{
					Open:       true,
					DialogType: FileDialogAddBookmark,
					Message:    "/home/user/thicket/meadow",
					Fields:     []FileDialogField{{Label: "Name", Value: "harborlantern"}},
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"Path:", "Name:"},
		},
		{
			name: "group select",
			draw: func(screen tcell.Screen) {
				DrawGroupSelectDialog(screen, layout, GroupSelectState{
					Mode:        "select",
					PatternMode: panel.GroupPatternShell,
					Text:        "*.badger",
					TextCursor:  8,
				}, styles)
			},
			labels: []string{"Pattern:"},
		},
		{
			name: "mass rename",
			draw: func(screen tcell.Screen) {
				DrawFileDialog(screen, layout, FileDialogState{
					Open:       true,
					DialogType: FileDialogMassRename,
					Fields: []FileDialogField{
						{Label: "Find", Value: "walrus"},
						{Label: "Replace", Value: "otter"},
					},
					MassRenameMode:          MassRenameModeUISimple,
					MassRenamePreviewBefore: []string{"walrus.txt"},
					MassRenamePreviewAfter:  []string{"otter.txt"},
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"Find (0):", "Replace:"},
		},
		{
			name: "rename",
			draw: func(screen tcell.Screen) {
				DrawFileDialog(screen, layout, FileDialogState{
					Open:       true,
					DialogType: FileDialogRename,
					Fields:     []FileDialogField{{Label: "New name", Value: "badgerthicket.txt"}},
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"New name:"},
		},
		{
			name: "mkdir with post-actions",
			draw: func(screen tcell.Screen) {
				DrawFileDialog(screen, layout, FileDialogState{
					Open:             true,
					DialogType:       FileDialogMkdir,
					Fields:           []FileDialogField{{Label: "Directory name", Value: "meadow"}},
					MkdirShowActions: true,
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"Directory name:"},
		},
		{
			name: "rename tool preview",
			draw: func(screen tcell.Screen) {
				DrawFileDialog(screen, layout, FileDialogState{
					Open:        true,
					DialogType:  FileDialogRename,
					RenamePhase: RenamePhaseSanitize,
					Fields:      []FileDialogField{{Label: "New name", Value: "badger thicket.txt"}},
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"Preview:"},
		},
		{
			name: "mass rename save prompt",
			draw: func(screen tcell.Screen) {
				DrawFileDialog(screen, layout, FileDialogState{
					Open:            true,
					DialogType:      FileDialogMassRename,
					MassRenamePhase: MassRenamePhaseSavePrompt,
					Fields: []FileDialogField{
						{Label: "Name", Value: "harbor"},
						{Label: "Description", Value: "strip camera prefix"},
					},
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"Name:", "Description:"},
		},
		{
			name: "run for each",
			draw: func(screen tcell.Screen) {
				DrawFileDialog(screen, layout, FileDialogState{
					Open:            true,
					DialogType:      FileDialogRunForEach,
					Message:         "Run a command for each selected entry.",
					Fields:          []FileDialogField{{Label: "Command", Value: "convert %f out.png"}},
					RunForEachPools: []string{"thicket", "meadow"},
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"Command:", "Worker pool (optional):"},
		},
		{
			name: "transfer copy destination",
			draw: func(screen tcell.Screen) {
				DrawTransferDialog(screen, layout, TransferDialogState{
					Open:        true,
					Kind:        TransferKindCopy,
					Phase:       TransferPhaseDestination,
					Destination: FileDialogField{Value: "/home/user/meadow", PathPicker: true},
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"Destination:"},
		},
		{
			name: "transfer move destination",
			draw: func(screen tcell.Screen) {
				DrawTransferDialog(screen, layout, TransferDialogState{
					Open:        true,
					Kind:        TransferKindMove,
					Phase:       TransferPhaseDestination,
					Destination: FileDialogField{Value: "/home/user/meadow", PathPicker: true},
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"Destination:"},
		},
		{
			name: "transfer self-copy rename",
			draw: func(screen tcell.Screen) {
				DrawTransferDialog(screen, layout, TransferDialogState{
					Open:            true,
					Kind:            TransferKindCopy,
					Phase:           TransferPhaseSelfCopyRename,
					SelfCopyNewName: FileDialogField{Value: "badger copy.txt"},
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"New name:"},
		},
		{
			name: "transfer multi-location",
			draw: func(screen tcell.Screen) {
				DrawTransferDialog(screen, layout, TransferDialogState{
					Open:        true,
					Kind:        TransferKindCopy,
					Phase:       TransferPhaseDestination,
					Destination: FileDialogField{Value: "/home/user/meadow", PathPicker: true},
					CommonRoot:  "/home/user/thicket",
					Entries: []DeleteListEntry{
						{Name: "badger.txt"},
						{Name: "otter.txt"},
					},
				}, DialogRenderContext{Styles: styles}, nil)
			},
			labels: []string{"Source:", "Destination:"},
		},
		{
			name: "flatten",
			draw: func(screen tcell.Screen) {
				DrawFlattenDialog(screen, layout, FlattenDialogState{
					Open:        true,
					Destination: FileDialogField{Value: "/home/user/meadow", PathPicker: true},
				}, styles)
			},
			labels: []string{"Destination:"},
		},
		{
			name: "filter",
			draw: func(screen tcell.Screen) {
				DrawFilterDialog(screen, layout, FilterDialogState{
					Open:        true,
					PatternMode: panel.GroupPatternShell,
					Text:        "*.badger",
					TextCursor:  8,
				}, styles)
			},
			labels: []string{"Pattern:"},
		},
		{
			name: "configuration",
			draw: func(screen tcell.Screen) {
				DrawConfigDialog(screen, layout, ConfigDialogState{Open: true}, styles)
			},
			labels: []string{"View options:", "Scroll mode:", "Default listing format:"},
		},
		{
			name: "compare merge",
			draw: func(screen tcell.Screen) {
				DrawCompareMergeDialog(screen, layout, CompareMergeDialogState{
					Open:          true,
					PrimaryPath:   "/home/user/thicket",
					SecondaryPath: "/home/user/meadow",
					PreviewText:   "12 files",
				}, styles, "/home/user")
			},
			labels: []string{"Destination:", "Transfer:", "Operation:"},
		},
		{
			name: "image capabilities",
			draw: func(screen tcell.Screen) {
				DrawImageCapabilityDialog(screen, layout, ImageCapabilityDialogState{Open: true}, styles)
			},
			labels: []string{"Confirm terminal capabilities:", "Active protocol:"},
		},
		{
			name: "calibrate debounce",
			draw: func(screen tcell.Screen) {
				DrawDebounceCalibrateDialog(screen, layout, DebounceCalibrateDialogState{
					Open: true, Value: "120", ImageValue: "250",
				}, styles)
			},
			labels: []string{"Debounce (ms):", "Image preview debounce (ms):"},
		},
		{
			name: "dedup progress",
			draw: func(screen tcell.Screen) {
				DrawDedupProgressDialog(screen, layout, DedupProgressDialogState{Open: true},
					comparepkg.DedupSnapshot{
						Root:   pathloc.FileMust("/home/user/thicket"),
						Phase:  comparepkg.DedupWalking,
						Walked: 42,
					}, styles, "/home/user")
			},
			labels: []string{"Directory:"},
		},
		{
			name: "sftp connect",
			draw: func(screen tcell.Screen) {
				DrawSFTPConnectDialog(screen, layout, SFTPConnectDialogState{
					Open:         true,
					DisplayLines: []string{"harbor", "thicket"},
					Ranked:       []int{0, 1},
					MatchRanges:  make([][]search.Range, 2),
					Location:     FileDialogField{Value: "/srv/meadow"},
				}, styles)
			},
			labels: []string{"Location:"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			defer screen.Fini()
			screen.SetSize(w, h)
			tc.draw(screen)

			rows := make([]string, h)
			for y := 0; y < h; y++ {
				rows[y] = strings.TrimSpace(strings.Trim(tcelltest.TextAt(screen, 0, y, w), "│ "))
			}
			for _, label := range tc.labels {
				y := -1
				for i, row := range rows {
					if strings.HasPrefix(row, label) {
						y = i
						break
					}
				}
				if y < 0 {
					t.Fatalf("label %q not drawn; rows:\n%s", label, strings.Join(rows, "\n"))
				}
				if y+1 >= len(rows) {
					t.Fatalf("label %q drawn too low at row %d", label, y)
				}
				if rows[y+1] == "" {
					t.Errorf("row below %q is blank, want the labelled content directly beneath it", label)
				}
			}
		})
	}
}
