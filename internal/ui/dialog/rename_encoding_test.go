package dialog

import "testing"

func TestRenameEncodingPreviewText(t *testing.T) {
	st := FileDialogState{
		DialogType:  FileDialogRename,
		RenamePhase: RenamePhaseEncoding,
		RenameEncodingCandidates: []RenameEncodingCandidate{
			{Label: "Shift-JIS", UTF8: "日本語"},
			{Label: "EUC-JP", UTF8: "別名"},
		},
		RenameEncodingSelected: 1,
	}
	if got := RenameEncodingPreviewText(st); got != "別名" {
		t.Fatalf("got %q want 別名", got)
	}
}

func TestRenameEncodingOptionCount(t *testing.T) {
	st := FileDialogState{
		RenamePhase: RenamePhaseEncoding,
		RenameEncodingCandidates: []RenameEncodingCandidate{
			{Label: "Shift-JIS", UTF8: "a"},
			{Label: "EUC-JP", UTF8: "b"},
		},
	}
	if got := RenameEncodingOptionCount(st); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
}

func TestRenameToolOptionCountEncoding(t *testing.T) {
	st := FileDialogState{
		RenamePhase: RenamePhaseEncoding,
		RenameEncodingCandidates: []RenameEncodingCandidate{
			{Label: "Shift-JIS", UTF8: "a"},
		},
	}
	if got := renameToolOptionCount(st); got != 1 {
		t.Fatalf("got %d want 1", got)
	}
}

func TestRenameToolPreviewTextEncoding(t *testing.T) {
	st := FileDialogState{
		DialogType:  FileDialogRename,
		RenamePhase: RenamePhaseEncoding,
		RenameEncodingCandidates: []RenameEncodingCandidate{
			{Label: "Shift-JIS", UTF8: "テスト"},
		},
		Fields: []FileDialogField{{Value: "garbled"}},
	}
	if got := renameToolPreviewText(st); got != "テスト" {
		t.Fatalf("got %q want テスト", got)
	}
}
