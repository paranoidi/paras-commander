package editor

import (
	"reflect"
	"testing"
)

func TestResolveEditor(t *testing.T) {
	t.Setenv("VISUAL", "emacs -nw")
	t.Setenv("EDITOR", "vim")
	if got := ResolveEditor(); got != "emacs -nw" {
		t.Fatalf("ResolveEditor() = %q, want emacs -nw", got)
	}

	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nvim")
	if got := ResolveEditor(); got != "nvim" {
		t.Fatalf("ResolveEditor() = %q, want nvim", got)
	}

	t.Setenv("EDITOR", "")
	if got := ResolveEditor(); got != "vi" {
		t.Fatalf("ResolveEditor() = %q, want vi", got)
	}
}

func TestEditorArgv(t *testing.T) {
	tests := []struct {
		name   string
		editor string
		path   string
		want   []string
	}{
		{"simple", "vim", "/tmp/a", []string{"vim", "/tmp/a"}},
		{"with flags", "emacs -nw", "/tmp/a", []string{"emacs", "-nw", "/tmp/a"}},
		{"empty editor", "", "/tmp/a", []string{"vi", "/tmp/a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EditorArgv(tt.editor, tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("EditorArgv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEditorArgvInvalid(t *testing.T) {
	if _, err := EditorArgv(`"unclosed`, "/tmp/a"); err == nil {
		t.Fatal("expected parse error")
	}
}
