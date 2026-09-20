package dialog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

func TestExecuteChownEmptyFieldsAreRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "willow.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	model := &ui.Model{}
	if err := model.Primary.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := model.Secondary.Load(dir); err != nil {
		t.Fatalf("Load secondary: %v", err)
	}
	if !model.Primary.SelectVisibleEntry("willow.txt") {
		t.Fatal("willow.txt not visible")
	}
	cfg := config.Default()
	cfg.Preview.Prefetch = false
	fh := &identityTestHost{model: model, cfg: cfg}
	screen := uitest.Screen(t, 80, 24)
	h := New(Deps{
		Host:   fh,
		Screen: screen,
		Model:  model,
		Preview: previewctrl.New(previewctrl.Deps{
			Host:   fh,
			Screen: screen,
			Model:  model,
			Ctx:    context.Background(),
		}),
	})

	tests := []struct {
		name    string
		user    string
		group   string
		wantErr bool
	}{
		{name: "empty", user: "", group: "", wantErr: true},
		{name: "whitespace", user: "  ", group: " \t", wantErr: true},
		{name: "user-only", user: "0", group: "", wantErr: false},
		{name: "group-only", user: "", group: "0", wantErr: false},
		{name: "both", user: "0", group: "0", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fh.errors = nil
			fh.messages = nil
			h.model.FileDialog = uidialog.FileDialogState{
				Open:       true,
				DialogType: uidialog.FileDialogChown,
				Fields: []uidialog.FileDialogField{
					{Label: "User", Value: tt.user},
					{Label: "Group", Value: tt.group},
				},
			}
			h.executeChown()
			if !tt.wantErr {
				h.ApplyRemoteFileOp(waitRemoteFileOp(t, screen))
			}
			joinedErr := strings.Join(fh.errors, "\n")
			joinedMsg := strings.Join(fh.messages, "\n")
			if tt.wantErr {
				if joinedErr == "" {
					t.Fatalf("empty chown reported no error; messages=%q", joinedMsg)
				}
				if strings.Contains(joinedMsg, "Changed owner") {
					t.Fatalf("empty chown reported success: %q", joinedMsg)
				}
				return
			}
			if strings.Contains(joinedMsg, "Changed owner") {
				return
			}
			if strings.Contains(joinedErr, "user or group is required") {
				t.Fatalf("populated chown rejected as empty: %q", joinedErr)
			}
		})
	}
}
