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
// separately anyway). Deliberately narrower than tmuxOuterTerminalSupportsKittyGraphics below:
// this one gates Unicode-placeholder support specifically, which WezTerm doesn't have even
// though it does implement the Kitty graphics protocol itself.
func tmuxOuterTerminalIsKittyOrGhostty() bool {
	t := tmuxClientTermType()
	return strings.Contains(t, "kitty") || strings.Contains(t, "ghostty")
}

// tmuxOuterTerminalSupportsKittyGraphics reports whether tmux's client_termtype identifies a
// terminal known to implement the Kitty graphics protocol at all (cursor-relative "default
// placement" — not necessarily Unicode placeholders; see TmuxSupportsKittyUnicodePlaceholders
// for that narrower, WezTerm-excluding check). environ("TMUX") must already be confirmed
// non-empty by the caller.
func tmuxOuterTerminalSupportsKittyGraphics() bool {
	t := tmuxClientTermType()
	return strings.Contains(t, "kitty") || strings.Contains(t, "ghostty") || strings.Contains(t, "wezterm")
}

// ResolveImageProtocol picks the terminal graphics protocol from config and the environment.
// environ is typically os.Getenv. Invalid/empty cfg is treated as auto.
//
// Kitty is preferred over Sixel for any terminal known to implement it, WezTerm included, both
// outside and under tmux. Under tmux, WezTerm's Kitty support has no Unicode-placeholder mode
// (see TmuxSupportsKittyUnicodePlaceholders), so it goes through cursor-relative placement via
// tmux passthrough — the same passthrough-plus-cursor-ceremony path Sixel uses when it isn't
// tmux-natively-stored, and it shares that path's cursor-position race (lesson 3/8): confirmed
// live to intermittently misplace at 0,0 or vanish. Sixel under tmux has its own, different
// known gap (lesson 11: can vanish on an unrelated tmux redraw sweep, unrelated to cursor
// position). Neither is fully fixed; this is a deliberate choice of Kitty's failure mode over
// Sixel's (see llm-docs/graphics-implementation-lessons.md lesson 12/13) — revisit if either
// upstream issue closes, or if the cursor-ceremony mitigation in `emitImageAfterShow` can be
// made more reliable specifically against WezTerm.
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
		if environ("TMUX") != "" && tmuxOuterTerminalSupportsKittyGraphics() {
			return previewpanel.ImageProtocolKitty
		}
		prog := strings.ToLower(strings.TrimSpace(environ("TERM_PROGRAM")))
		if prog == "kitty" || prog == "ghostty" || prog == "wezterm" {
			return previewpanel.ImageProtocolKitty
		}
		term := strings.ToLower(strings.TrimSpace(environ("TERM")))
		// Ghostty's default TERM is xterm-ghostty (not a "ghostty…" prefix); TERM_PROGRAM
		// is often missing over SSH, so match the substring. WezTerm's default TERM doesn't
		// self-identify (typically xterm-256color), so it relies on TERM_PROGRAM above.
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
// Otherwise uses the same auto/sixel/kitty resolution as still-image previews.
func ResolveVideoThumbProtocol(imagesEnabled bool, cfg string, environ func(string) string) previewpanel.ImageProtocol {
	if !imagesEnabled {
		return previewpanel.ImageProtocolNone
	}
	return ResolveImageProtocol(cfg, environ)
}
