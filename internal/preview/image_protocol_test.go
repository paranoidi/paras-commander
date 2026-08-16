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
		{name: "auto wezterm", cfg: "auto", env: map[string]string{"TERM_PROGRAM": "WezTerm", "TERM": "xterm-256color"}, want: previewpanel.ImageProtocolKitty},
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
			got := ResolveImageProtocol(config.PreviewConfig{ImageProtocol: tc.cfg}, env(tc.env))
			if got != tc.want {
				t.Fatalf("ResolveImageProtocol(%q) = %v, want %v", tc.cfg, got, tc.want)
			}
		})
	}
}

func TestResolveImageProtocolTerminalTriState(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	cases := []struct {
		name  string
		sixel string
		kitty string
		env   map[string]string
		want  previewpanel.ImageProtocol
	}{
		{name: "kitty confirmed yes overrides sixel-leaning env", sixel: "auto", kitty: "yes", env: map[string]string{}, want: previewpanel.ImageProtocolKitty},
		{name: "sixel confirmed yes overrides kitty-leaning env", sixel: "yes", kitty: "auto", env: map[string]string{"TERM_PROGRAM": "kitty"}, want: previewpanel.ImageProtocolSixel},
		{name: "kitty confirmed no falls back to sixel", sixel: "auto", kitty: "no", env: map[string]string{"TERM_PROGRAM": "kitty"}, want: previewpanel.ImageProtocolSixel},
		{name: "sixel confirmed no falls back to kitty when env agrees kitty is absent", sixel: "no", kitty: "auto", env: map[string]string{}, want: previewpanel.ImageProtocolKitty},
		{name: "both auto keeps heuristic", sixel: "auto", kitty: "auto", env: map[string]string{"TERM_PROGRAM": "kitty"}, want: previewpanel.ImageProtocolKitty},
		{name: "both yes keeps heuristic (ambiguous override ignored)", sixel: "yes", kitty: "yes", env: map[string]string{}, want: previewpanel.ImageProtocolSixel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.PreviewConfig{ImageProtocol: "auto", TerminalSixel: tc.sixel, TerminalKitty: tc.kitty}
			got := ResolveImageProtocol(cfg, env(tc.env))
			if got != tc.want {
				t.Fatalf("ResolveImageProtocol = %v, want %v", got, tc.want)
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

	cfg := config.PreviewConfig{ImageProtocol: "auto"}

	tmuxClientTermType = func() string { return "ghostty 1.3.1" }
	if got := ResolveImageProtocol(cfg, envUnderTmux); got != previewpanel.ImageProtocolKitty {
		t.Fatalf("ResolveImageProtocol under tmux+ghostty = %v, want Kitty", got)
	}

	tmuxClientTermType = func() string { return "wezterm 20260716" }
	if got := ResolveImageProtocol(cfg, envUnderTmux); got != previewpanel.ImageProtocolKitty {
		t.Fatalf("ResolveImageProtocol under tmux+wezterm = %v, want Kitty", got)
	}
}

func TestResolveVideoThumbProtocol(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	cases := []struct {
		name   string
		images bool
		cfg    string
		env    map[string]string
		want   previewpanel.ImageProtocol
	}{
		{name: "images off", images: false, cfg: "auto", env: map[string]string{"TERM_PROGRAM": "kitty"}, want: previewpanel.ImageProtocolNone},
		{name: "force sixel", images: true, cfg: "sixel", env: map[string]string{}, want: previewpanel.ImageProtocolSixel},
		{name: "force kitty", images: true, cfg: "kitty", env: map[string]string{}, want: previewpanel.ImageProtocolKitty},
		{name: "auto kitty", images: true, cfg: "auto", env: map[string]string{"TERM_PROGRAM": "kitty"}, want: previewpanel.ImageProtocolKitty},
		{name: "auto ghostty", images: true, cfg: "auto", env: map[string]string{"TERM_PROGRAM": "ghostty"}, want: previewpanel.ImageProtocolKitty},
		{name: "auto wezterm kitty", images: true, cfg: "auto", env: map[string]string{"TERM_PROGRAM": "WezTerm"}, want: previewpanel.ImageProtocolKitty},
		{name: "auto unknown sixel", images: true, cfg: "auto", env: map[string]string{}, want: previewpanel.ImageProtocolSixel},
		{name: "auto xterm-kitty TERM", images: true, cfg: "auto", env: map[string]string{"TERM": "xterm-kitty"}, want: previewpanel.ImageProtocolKitty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveVideoThumbProtocol(tc.images, config.PreviewConfig{ImageProtocol: tc.cfg}, env(tc.env))
			if got != tc.want {
				t.Fatalf("ResolveVideoThumbProtocol = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveVideoThumbProtocolTmux(t *testing.T) {
	orig := tmuxClientTermType
	t.Cleanup(func() { tmuxClientTermType = orig })

	envUnderTmux := func(k string) string {
		switch k {
		case "TMUX":
			return "/tmp/tmux-1000/default,1234,0"
		case "TERM_PROGRAM":
			return "tmux"
		case "TERM":
			return "tmux-256color"
		default:
			return ""
		}
	}

	cfg := config.PreviewConfig{ImageProtocol: "auto"}

	tmuxClientTermType = func() string { return "ghostty 1.3.1" }
	if got := ResolveVideoThumbProtocol(true, cfg, envUnderTmux); got != previewpanel.ImageProtocolKitty {
		t.Fatalf("tmux+ghostty = %v, want Kitty", got)
	}
	tmuxClientTermType = func() string { return "wezterm 20260716" }
	if got := ResolveVideoThumbProtocol(true, cfg, envUnderTmux); got != previewpanel.ImageProtocolKitty {
		t.Fatalf("tmux+wezterm = %v, want Kitty", got)
	}
}

func TestTmuxSupportsKittyUnicodePlaceholders(t *testing.T) {
	orig := tmuxClientTermType
	t.Cleanup(func() { tmuxClientTermType = orig })

	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	tmuxEnv := map[string]string{"TMUX": "/tmp/tmux-1000/default,1234,0"}

	if TmuxSupportsKittyUnicodePlaceholders(nil, config.PreviewConfig{}) {
		t.Fatal("nil environ: want false")
	}
	if TmuxSupportsKittyUnicodePlaceholders(env(nil), config.PreviewConfig{}) {
		t.Fatal("empty env (no TMUX): want false")
	}

	tmuxClientTermType = func() string { return "ghostty 1.3.1" }
	if !TmuxSupportsKittyUnicodePlaceholders(env(tmuxEnv), config.PreviewConfig{}) {
		t.Fatal("tmux+ghostty: want true")
	}
	tmuxClientTermType = func() string { return "xterm-kitty" }
	if !TmuxSupportsKittyUnicodePlaceholders(env(tmuxEnv), config.PreviewConfig{}) {
		t.Fatal("tmux+kitty: want true")
	}
	tmuxClientTermType = func() string { return "wezterm 20260716" }
	if TmuxSupportsKittyUnicodePlaceholders(env(tmuxEnv), config.PreviewConfig{TerminalKittyPlaceholder: "auto"}) {
		t.Fatal("tmux+wezterm, no config opt-in: want false")
	}
	if !TmuxSupportsKittyUnicodePlaceholders(env(tmuxEnv), config.PreviewConfig{TerminalKittyPlaceholder: "yes"}) {
		t.Fatal("tmux+wezterm, opted in via config: want true")
	}
	// Outside tmux, client_termtype is irrelevant — placeholders are a tmux-only path.
	tmuxClientTermType = func() string { return "ghostty 1.3.1" }
	if TmuxSupportsKittyUnicodePlaceholders(env(map[string]string{"TERM_PROGRAM": "ghostty"}), config.PreviewConfig{}) {
		t.Fatal("ghostty outside tmux: want false")
	}
}

func TestTmuxSupportsNativeSixel(t *testing.T) {
	orig := tmuxClientTermFeatures
	t.Cleanup(func() { tmuxClientTermFeatures = orig })

	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	tmuxEnv := map[string]string{"TMUX": "/tmp/tmux-1000/default,1234,0"}

	if TmuxSupportsNativeSixel(nil) {
		t.Fatal("nil environ: want false")
	}
	if TmuxSupportsNativeSixel(env(nil)) {
		t.Fatal("empty env (no TMUX): want false")
	}

	tmuxClientTermFeatures = func() string { return "256,rgb,sixel,sync" }
	if !TmuxSupportsNativeSixel(env(tmuxEnv)) {
		t.Fatal("client_termfeatures has sixel: want true")
	}

	tmuxClientTermFeatures = func() string { return "256,rgb,sync" }
	if TmuxSupportsNativeSixel(env(tmuxEnv)) {
		t.Fatal("client_termfeatures without sixel: want false")
	}

	// Outside tmux, client_termfeatures is irrelevant — this is a tmux-only path.
	tmuxClientTermFeatures = func() string { return "sixel" }
	if TmuxSupportsNativeSixel(env(nil)) {
		t.Fatal("sixel feature outside tmux: want false")
	}
}

func TestCapabilityUncertain(t *testing.T) {
	orig := tmuxClientTermType
	t.Cleanup(func() { tmuxClientTermType = orig })

	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	autoCfg := config.PreviewConfig{
		ImageProtocol:            "auto",
		TerminalSixel:            "auto",
		TerminalKitty:            "auto",
		TerminalKittyPlaceholder: "auto",
	}

	t.Run("all-auto outside tmux is uncertain", func(t *testing.T) {
		if !CapabilityUncertain(autoCfg, env(nil)) {
			t.Fatal("want uncertain (sixel fallback guess, unconfirmed)")
		}
	})

	t.Run("all confirmed is not uncertain", func(t *testing.T) {
		cfg := config.PreviewConfig{
			ImageProtocol:            "auto",
			TerminalSixel:            "yes",
			TerminalKitty:            "no",
			TerminalKittyPlaceholder: "yes",
		}
		if CapabilityUncertain(cfg, env(nil)) {
			t.Fatal("want not uncertain")
		}
	})

	t.Run("explicit protocol override is never uncertain", func(t *testing.T) {
		cfg := autoCfg
		cfg.ImageProtocol = "sixel"
		if CapabilityUncertain(cfg, env(nil)) {
			t.Fatal("want not uncertain")
		}
	})

	t.Run("tmux client_termtype kitty is not uncertain even with placeholder auto", func(t *testing.T) {
		tmuxClientTermType = func() string { return "kitty" }
		env := env(map[string]string{"TMUX": "/tmp/tmux-1000/default,1234,0"})
		if CapabilityUncertain(autoCfg, env) {
			t.Fatal("want not uncertain (kitty client_termtype always qualifies)")
		}
	})

	t.Run("tmux client_termtype wezterm is uncertain: kitty confirmed, placeholder not", func(t *testing.T) {
		tmuxClientTermType = func() string { return "wezterm 20260716" }
		env := env(map[string]string{"TMUX": "/tmp/tmux-1000/default,1234,0"})
		if !CapabilityUncertain(autoCfg, env) {
			t.Fatal("want uncertain (placeholder support unconfirmed for WezTerm)")
		}
		cfg := autoCfg
		cfg.TerminalKittyPlaceholder = "yes"
		if CapabilityUncertain(cfg, env) {
			t.Fatal("want not uncertain once placeholder confirmed via config")
		}
	})
}

func TestDetectTerminalCapabilities(t *testing.T) {
	origType, origFeatures := tmuxClientTermType, tmuxClientTermFeatures
	t.Cleanup(func() { tmuxClientTermType, tmuxClientTermFeatures = origType, origFeatures })

	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	tmuxEnv := map[string]string{"TMUX": "/tmp/tmux-1000/default,1234,0"}

	if sixel, kitty, placeholder := DetectTerminalCapabilities(nil); !sixel || kitty || placeholder {
		t.Fatalf("nil environ: got sixel=%v kitty=%v placeholder=%v, want true/false/false", sixel, kitty, placeholder)
	}

	if sixel, kitty, placeholder := DetectTerminalCapabilities(env(map[string]string{"TERM_PROGRAM": "kitty"})); sixel || !kitty || !placeholder {
		t.Fatalf("kitty outside tmux: got sixel=%v kitty=%v placeholder=%v, want false/true/true", sixel, kitty, placeholder)
	}

	if sixel, kitty, placeholder := DetectTerminalCapabilities(env(map[string]string{"TERM_PROGRAM": "wezterm"})); sixel || !kitty || placeholder {
		t.Fatalf("wezterm outside tmux: got sixel=%v kitty=%v placeholder=%v, want false/true/false (placeholder is never auto-detectable for WezTerm)", sixel, kitty, placeholder)
	}

	if sixel, kitty, placeholder := DetectTerminalCapabilities(env(map[string]string{"TERM": "xterm-256color"})); !sixel || kitty || placeholder {
		t.Fatalf("unrecognized terminal outside tmux: got sixel=%v kitty=%v placeholder=%v, want true/false/false", sixel, kitty, placeholder)
	}

	tmuxClientTermType = func() string { return "ghostty 1.3.1" }
	tmuxClientTermFeatures = func() string { return "256,rgb,sync" }
	if sixel, kitty, placeholder := DetectTerminalCapabilities(env(tmuxEnv)); sixel || !kitty || !placeholder {
		t.Fatalf("tmux+ghostty, no sixel feature: got sixel=%v kitty=%v placeholder=%v, want false/true/true", sixel, kitty, placeholder)
	}

	tmuxClientTermType = func() string { return "wezterm 20260716" }
	tmuxClientTermFeatures = func() string { return "256,rgb,sixel,sync" }
	if sixel, kitty, placeholder := DetectTerminalCapabilities(env(tmuxEnv)); !sixel || !kitty || placeholder {
		t.Fatalf("tmux+wezterm with sixel feature: got sixel=%v kitty=%v placeholder=%v, want true/true/false (placeholder never auto-detected for WezTerm)", sixel, kitty, placeholder)
	}
}
