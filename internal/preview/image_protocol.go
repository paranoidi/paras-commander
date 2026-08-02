package preview

import (
	"os/exec"
	"strings"
	"sync"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// tmuxClientTermType returns tmux's `client_termtype`: the identity string tmux obtained by
// querying the actually-attached terminal (XTVERSION/DA), e.g. "ghostty 1.x.x" or
// "WezTerm 20260716-...". This reflects the real outer terminal regardless of what tmux sets
// TERM/TERM_PROGRAM to inside the pane (tmux 3.2+ overwrites TERM_PROGRAM to "tmux" for every
// pane, and TERM is always tmux's own tmux-256color/screen-256color). Cached for the process
// lifetime (the attached terminal can't change mid-run) since shelling out on every preview
// render would be too slow. Overridable in tests.
var tmuxClientTermType = sync.OnceValue(func() string {
	out, err := exec.Command("tmux", "display-message", "-p", "#{client_termtype}").Output()
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(string(out)))
})

// tmuxOuterTerminalIsKittyOrGhostty reports whether tmux's client_termtype identifies the
// actually-attached terminal as Kitty or Ghostty. environ("TMUX") must already be confirmed
// non-empty by the caller (this does not check it itself, since callers need that check
// separately anyway).
func tmuxOuterTerminalIsKittyOrGhostty() bool {
	t := tmuxClientTermType()
	return strings.Contains(t, "kitty") || strings.Contains(t, "ghostty")
}

// ResolveImageProtocol picks the terminal graphics protocol from config and the environment.
// environ is typically os.Getenv. Invalid/empty cfg is treated as auto.
func ResolveImageProtocol(cfg string, environ func(string) string) previewpanel.ImageProtocol {
	switch strings.ToLower(strings.TrimSpace(cfg)) {
	case config.PreviewImageProtocolSixel:
		return previewpanel.ImageProtocolSixel
	case config.PreviewImageProtocolKitty:
		return previewpanel.ImageProtocolKitty
	default:
		// auto (and unknown values)
		if environ == nil {
			return previewpanel.ImageProtocolSixel
		}
		if environ("TMUX") != "" && tmuxOuterTerminalIsKittyOrGhostty() {
			return previewpanel.ImageProtocolKitty
		}
		prog := strings.ToLower(strings.TrimSpace(environ("TERM_PROGRAM")))
		if prog == "kitty" || prog == "ghostty" {
			return previewpanel.ImageProtocolKitty
		}
		term := strings.ToLower(strings.TrimSpace(environ("TERM")))
		// Ghostty's default TERM is xterm-ghostty (not a "ghostty…" prefix); TERM_PROGRAM
		// is often missing over SSH, so match the substring.
		if term == "xterm-kitty" || strings.Contains(term, "ghostty") {
			return previewpanel.ImageProtocolKitty
		}
		return previewpanel.ImageProtocolSixel
	}
}

// TmuxSupportsKittyUnicodePlaceholders reports whether, under tmux, the actually-attached
// outer terminal is known to support Kitty's Unicode-placeholder image display (currently
// Kitty and Ghostty only). Unlike ResolveImageProtocol's "auto" path,
// this must be checked even when the Kitty protocol was reached via an explicit
// image_protocol=kitty config override: forcing Kitty protocol does not make an
// otherwise-unsupported terminal (e.g. WezTerm) understand Unicode placeholders, so a caller
// under tmux that skips this check and uses placeholder mode anyway sends cells the terminal
// can't interpret — nothing renders, on any terminal that ends up here without support.
// environ is typically os.Getenv.
func TmuxSupportsKittyUnicodePlaceholders(environ func(string) string) bool {
	if environ == nil || environ("TMUX") == "" {
		return false
	}
	return tmuxOuterTerminalIsKittyOrGhostty()
}

// tmuxClientTermFeatures returns tmux's `client_termfeatures`: the comma-separated list of
// terminal features tmux has resolved for the actually-attached outer terminal (its built-in
// terminal-features database keyed by client_termtype, combined with any user
// terminal-overrides). This is the runtime-accurate answer to "will tmux actually redraw a
// bare sixel DCS I send it" — unlike tmux's own DA1 reply to the pane (which only reflects
// whether tmux was compiled with --enable-sixel, not whether the attached terminal supports
// it; tmux falls back to a blank/text placeholder at redraw time otherwise). Cached for the
// process lifetime, same rationale as tmuxClientTermType.
var tmuxClientTermFeatures = sync.OnceValue(func() string {
	out, err := exec.Command("tmux", "display-message", "-p", "#{client_termfeatures}").Output()
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(string(out)))
})

// TmuxSupportsNativeSixel reports whether, under tmux, the attached outer terminal's resolved
// features include sixel. When true, tmux parses a bare (unwrapped) sixel DCS sent to it,
// stores the image, and redraws it itself after every tmux-side invalidate (status tick,
// window switch, etc.) — see tmux-wrap.md. Passthrough-wrapped sixel never reaches that path:
// tmux only recognizes a bare `DCS q` introducer, blind-forwards anything wrapped in
// `DCS tmux;`, and cannot re-send content it never parsed. environ is typically os.Getenv.
func TmuxSupportsNativeSixel(environ func(string) string) bool {
	if environ == nil || environ("TMUX") == "" {
		return false
	}
	return strings.Contains(tmuxClientTermFeatures(), "sixel")
}

// WarmTmuxCaches kicks off tmux's `display-message` capability probes
// (tmuxClientTermType, tmuxClientTermFeatures) in the background as soon as tmux is detected,
// so the first image preview doesn't pay for the `tmux` subprocess synchronously on the render
// path (ResolveImageProtocol / TmuxSupportsKittyUnicodePlaceholders / TmuxSupportsNativeSixel
// all block on these the first time they're called, then hit the sync.OnceValue cache forever
// after). No-op outside tmux. Call once at app startup; safe from any goroutine since
// sync.OnceValue serializes concurrent first calls. environ is typically os.Getenv.
func WarmTmuxCaches(environ func(string) string) {
	if environ == nil || environ("TMUX") == "" {
		return
	}
	go tmuxClientTermType()
	go tmuxClientTermFeatures()
}

// ResolveVideoThumbProtocol picks the graphics protocol for video thumbnail grids.
// When imagesEnabled is false ([preview].images), returns ImageProtocolNone.
// Otherwise uses the same auto/sixel/kitty resolution as still-image previews
// (WezTerm and other non-Kitty/Ghostty terminals get sixel under auto).
func ResolveVideoThumbProtocol(imagesEnabled bool, cfg string, environ func(string) string) previewpanel.ImageProtocol {
	if !imagesEnabled {
		return previewpanel.ImageProtocolNone
	}
	return ResolveImageProtocol(cfg, environ)
}
