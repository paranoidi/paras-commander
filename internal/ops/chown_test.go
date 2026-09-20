package ops

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/testutil"
)

func TestPlanChownEmptyAndPartialFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "otter.txt")
	testutil.WriteFile(t, filePath)
	source := Source{
		Kind:    SourceCursor,
		Entries: []localfs.Entry{{Name: "otter.txt", Path: filePath}},
	}

	tests := []struct {
		name    string
		user    string
		group   string
		wantErr bool
		wantUID int
		wantGID int
	}{
		{name: "empty", user: "", group: "", wantErr: true},
		{name: "whitespace", user: "  ", group: "\t", wantErr: true},
		{name: "user-only", user: "0", group: "", wantUID: 0, wantGID: -1},
		{name: "group-only", user: "", group: "0", wantUID: -1, wantGID: 0},
		{name: "both", user: "0", group: "0", wantUID: 0, wantGID: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan, err := PlanChown(source, tt.user, tt.group)
			if tt.wantErr {
				if err == nil {
					t.Fatal("PlanChown() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("PlanChown() error = %v", err)
			}
			if plan.UID != tt.wantUID {
				t.Fatalf("UID = %d, want %d", plan.UID, tt.wantUID)
			}
			if plan.GID != tt.wantGID {
				t.Fatalf("GID = %d, want %d", plan.GID, tt.wantGID)
			}
		})
	}
}
