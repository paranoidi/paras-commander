package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestDeleteDialogDescendIntoMountPoints(t *testing.T) {
	t.Parallel()
	if !deleteDialogDescendIntoMountPoints {
		t.Fatal("delete dialog must scan across mount points (Samba, etc.)")
	}
}

func TestDeleteDialogOpenExcludesDanglingDirs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		open               bool
		dialogType         dialog.FileDialogType
		deleteDanglingDirs bool
		wantOpen           bool
	}{
		{
			name:               "regular delete dialog is open",
			open:               true,
			dialogType:         dialog.FileDialogDelete,
			deleteDanglingDirs: false,
			wantOpen:           true,
		},
		{
			name:               "dangling dirs dialog is not open for refresh purposes",
			open:               true,
			dialogType:         dialog.FileDialogDelete,
			deleteDanglingDirs: true,
			wantOpen:           false,
		},
		{
			name:               "closed delete dialog is not open",
			open:               false,
			dialogType:         dialog.FileDialogDelete,
			deleteDanglingDirs: false,
			wantOpen:           false,
		},
		{
			name:               "non-delete dialog type is not open",
			open:               true,
			dialogType:         dialog.FileDialogRename,
			deleteDanglingDirs: false,
			wantOpen:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := &ui.Model{
				FileDialog: dialog.FileDialogState{
					Open:               tt.open,
					DialogType:         tt.dialogType,
					DeleteDanglingDirs: tt.deleteDanglingDirs,
				},
			}

			h := &Handler{
				model: model,
			}

			got := h.deleteDialogOpen()
			if got != tt.wantOpen {
				t.Fatalf("deleteDialogOpen() = %v, want %v", got, tt.wantOpen)
			}
		})
	}
}
