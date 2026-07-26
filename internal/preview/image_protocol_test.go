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
		{name: "auto ghostty TERM xterm-ghostty", cfg: "auto", env: map[string]string{"TERM": "xterm-ghostty"}, want: previewpanel.ImageProtocolKitty},
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

func TestResolveImageProtocolTmuxUsesClientTermType(t *testing.T) {
	orig := tmuxClientTermType
	t.Cleanup(func() { tmuxClientTermType = orig })

	envUnderTmux := func(k string) string {
		switch k {
		case "TERM_PROGRAM":
			return "tmux" // tmux 3.2+ overwrites this for every pane
		case "TMUX":
			return "/tmp/tmux-1000/default,1234,0"
		case "TERM":
			return "tmux-256color" // tmux's own TERM, not the outer terminal's
		default:
			return ""
		}
	}

	tmuxClientTermType = func() string { return "ghostty 1.3.1" }
	if got := ResolveImageProtocol("auto", envUnderTmux); got != previewpanel.ImageProtocolKitty {
		t.Fatalf("ResolveImageProtocol under tmux+ghostty = %v, want Kitty", got)
	}

	tmuxClientTermType = func() string { return "wezterm 20260716" }
	if got := ResolveImageProtocol("auto", envUnderTmux); got != previewpanel.ImageProtocolSixel {
		t.Fatalf("ResolveImageProtocol under tmux+wezterm = %v, want Sixel", got)
	}
}

func TestTmuxSupportsKittyUnicodePlaceholders(t *testing.T) {
	orig := tmuxClientTermType
	t.Cleanup(func() { tmuxClientTermType = orig })

	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	tmuxEnv := map[string]string{"TMUX": "/tmp/tmux-1000/default,1234,0"}

	if TmuxSupportsKittyUnicodePlaceholders(nil) {
		t.Fatal("nil environ: want false")
	}
	if TmuxSupportsKittyUnicodePlaceholders(env(nil)) {
		t.Fatal("empty env (no TMUX): want false")
	}

	tmuxClientTermType = func() string { return "ghostty 1.3.1" }
	if !TmuxSupportsKittyUnicodePlaceholders(env(tmuxEnv)) {
		t.Fatal("tmux+ghostty: want true")
	}
	tmuxClientTermType = func() string { return "xterm-kitty" }
	if !TmuxSupportsKittyUnicodePlaceholders(env(tmuxEnv)) {
		t.Fatal("tmux+kitty: want true")
	}
	tmuxClientTermType = func() string { return "wezterm 20260716" }
	if TmuxSupportsKittyUnicodePlaceholders(env(tmuxEnv)) {
		t.Fatal("tmux+wezterm: want false (no Unicode placeholder support)")
	}
	// Outside tmux, client_termtype is irrelevant — placeholders are a tmux-only path.
	tmuxClientTermType = func() string { return "ghostty 1.3.1" }
	if TmuxSupportsKittyUnicodePlaceholders(env(map[string]string{"TERM_PROGRAM": "ghostty"})) {
		t.Fatal("ghostty outside tmux: want false")
	}
}
