package dialog

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/diskusage"
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

func TestApplyDeleteDialogScanNeedNotJobFiltered(t *testing.T) {
	t.Parallel()
	s := newSnapshotHarness(t, true)
	s.h.diskUsage = diskusage.New()
	s.h.OpenDeleteDialog(ui.PrimaryPanel)
	p := s.h.host.ActivePanel()
	s.h.ApplyDeleteDialogScanNeed(DeleteDialogScanNeedPayload{
		Need: []string{s.dir},
		Gen:  p.SelectionDerivedGen(),
		Path: p.PathString(),
	})
	if s.h.deleteDialogScanFP == "" {
		t.Fatal("scan need was dropped; summary would stay pending")
	}
}

func TestDeleteDialogScansTargetPanelInCompareView(t *testing.T) {
	t.Parallel()
	s := newSnapshotHarness(t, true)
	s.h.diskUsage = diskusage.New()
	m := s.h.model
	m.ViewMode = ui.ViewCompare
	m.ActivePanel = ui.PrimaryPanel
	if !m.Secondary.SelectVisibleEntry("meadow.txt") {
		t.Fatal("meadow.txt not visible")
	}
	s.h.OpenDeleteDialog(ui.SecondaryPanel) // compare deletes the focused column's panel, not the active one

	s.h.ReconcileDeleteDialogScans()
	if s.h.deleteDialogScanArmedGen != m.Secondary.SelectionDerivedGen() {
		t.Fatal("reconcile skipped or used the wrong panel in compare view")
	}
	s.h.ApplyDeleteDialogScanNeed(DeleteDialogScanNeedPayload{
		Need: []string{s.dir},
		Gen:  m.Secondary.SelectionDerivedGen(),
		Path: m.Secondary.PathString(),
	})
	if s.h.deleteDialogScanFP == "" {
		t.Fatal("scan need for the target panel was dropped; summary would stay pending")
	}
}

func TestOpenDeleteDialogForPathKeepsStaticSummary(t *testing.T) {
	t.Parallel()
	s := newSnapshotHarness(t, true)
	s.h.diskUsage = diskusage.New()
	s.h.model.ViewMode = ui.ViewCompare
	s.h.OpenDeleteDialogForPath(filepath.Join(s.dir, "meadow.txt"), "meadow.txt")
	want := s.h.model.FileDialog.DeleteSummary
	s.h.ReconcileDeleteDialogScans()
	s.h.RefreshDeleteDialogSummary()
	if got := s.h.model.FileDialog.DeleteSummary; got != want {
		t.Fatalf("summary changed to %q, want %q", got, want)
	}
}
