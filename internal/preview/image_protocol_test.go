package preview

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestResolveImageProtocol(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	cases := []struct {
		name string
		cfg  string
		env  map[string]string
		want previewpanel.ImageProtocol
	}{
		{name: "force sixel", cfg: "sixel", want: previewpanel.ImageProtocolSixel},
		{name: "force kitty", cfg: "kitty", want: previewpanel.ImageProtocolKitty},
		{name: "auto wezterm", cfg: "auto", env: map[string]string{"TERM_PROGRAM": "WezTerm", "TERM": "xterm-256color"}, want: previewpanel.ImageProtocolSixel},
		{name: "auto ghostty TERM_PROGRAM", cfg: "auto", env: map[string]string{"TERM_PROGRAM": "ghostty"}, want: previewpanel.ImageProtocolKitty},
		{name: "auto kitty TERM_PROGRAM", cfg: "auto", env: map[string]string{"TERM_PROGRAM": "kitty"}, want: previewpanel.ImageProtocolKitty},
		{name: "auto xterm-kitty TERM", cfg: "auto", env: map[string]string{"TERM": "xterm-kitty"}, want: previewpanel.ImageProtocolKitty},
		{name: "auto ghostty TERM prefix", cfg: "auto", env: map[string]string{"TERM": "ghostty"}, want: previewpanel.ImageProtocolKitty},
		{name: "empty cfg is auto", cfg: "", env: map[string]string{"TERM_PROGRAM": "ghostty"}, want: previewpanel.ImageProtocolKitty},
		{name: "bogus cfg is auto", cfg: "nope", env: map[string]string{}, want: previewpanel.ImageProtocolSixel},
		{name: "explicit config constants", cfg: config.PreviewImageProtocolKitty, want: previewpanel.ImageProtocolKitty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveImageProtocol(tc.cfg, env(tc.env))
			if got != tc.want {
				t.Fatalf("ResolveImageProtocol(%q) = %v, want %v", tc.cfg, got, tc.want)
			}
		})
	}
}
