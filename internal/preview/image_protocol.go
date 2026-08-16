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

// tmuxOuterTerminalSupportsUnicodePlaceholders reports whether tmux's client_termtype
// identifies the actually-attached terminal as one known to support Kitty's Unicode-placeholder
// image display: Kitty and Ghostty always, plus any terminal the user has explicitly confirmed
// via cfg.TerminalKittyPlaceholder == "yes" (set through the M-F3 image-capabilities dialog) —
// for terminals like WezTerm where placeholder support is a build-specific capability
// client_termtype can't reliably confirm on its own. environ("TMUX") must already be confirmed
// non-empty by the caller (this does not check it itself, since callers need that check
// separately anyway).
func tmuxOuterTerminalSupportsUnicodePlaceholders(cfg config.PreviewConfig) bool {
	t := tmuxClientTermType()
	if strings.Contains(t, "kitty") || strings.Contains(t, "ghostty") {
		return true
	}
	return strings.ToLower(strings.TrimSpace(cfg.TerminalKittyPlaceholder)) == config.PreviewTerminalCapabilityYes
}

// tmuxOuterTerminalSupportsKittyGraphics reports whether tmux's client_termtype identifies a
// terminal known to implement the Kitty graphics protocol at all (cursor-relative "default
// placement" — not necessarily Unicode placeholders; see TmuxSupportsKittyUnicodePlaceholders).
// environ("TMUX") must already be confirmed non-empty by the caller.
func tmuxOuterTerminalSupportsKittyGraphics() bool {
	t := tmuxClientTermType()
	return strings.Contains(t, "kitty") || strings.Contains(t, "ghostty") || strings.Contains(t, "wezterm")
}

// kittyGraphicsConfirmedByEnv reports whether the environment alone (TERM_PROGRAM/TERM outside
// tmux, or tmux's client_termtype under tmux) positively identifies a terminal known to
// implement the Kitty graphics protocol — as opposed to just falling through to Sixel because
// nothing else matched, which is a low-confidence guess rather than a confirmation. Used by
// CapabilityUncertain; ResolveImageProtocol inlines the same checks via imageProtocolHeuristic.
func kittyGraphicsConfirmedByEnv(environ func(string) string) bool {
	if environ("TMUX") != "" && tmuxOuterTerminalSupportsKittyGraphics() {
		return true
	}
	prog := strings.ToLower(strings.TrimSpace(environ("TERM_PROGRAM")))
	if prog == "kitty" || prog == "ghostty" || prog == "wezterm" {
		return true
	}
	term := strings.ToLower(strings.TrimSpace(environ("TERM")))
	// Ghostty's default TERM is xterm-ghostty (not a "ghostty…" prefix); TERM_PROGRAM
	// is often missing over SSH, so match the substring. WezTerm's default TERM doesn't
	// self-identify (typically xterm-256color), so it relies on TERM_PROGRAM above.
	return term == "xterm-kitty" || strings.Contains(term, "ghostty")
}

// imageProtocolHeuristic is the TERM/TERM_PROGRAM/tmux-client_termtype guess ResolveImageProtocol
// falls back to when neither TerminalSixel nor TerminalKitty settles the question.
func imageProtocolHeuristic(environ func(string) string) previewpanel.ImageProtocol {
	if kittyGraphicsConfirmedByEnv(environ) {
		return previewpanel.ImageProtocolKitty
	}
	return previewpanel.ImageProtocolSixel
}

// ResolveImageProtocol picks the terminal graphics protocol from config and the environment.
// environ is typically os.Getenv. An empty/invalid cfg.ImageProtocol is treated as auto.
//
// For "auto", cfg.TerminalSixel/TerminalKitty (tri-state user confirmations set via the M-F3
// image-capabilities dialog) are consulted before the TERM/tmux heuristic: if exactly one of
// them is "yes", that protocol wins outright; if the heuristic's guess is contradicted by a
// "no" for that protocol, the other protocol is used instead; otherwise the heuristic's guess
// stands. Kitty is preferred over Sixel for any terminal known to implement it, WezTerm
// included, both outside and under tmux. Under tmux, whether that Kitty traffic goes out as a
// race-free Unicode placeholder or as cursor-relative-via-passthrough depends on whether the
// outer terminal is confirmed placeholder-capable — see TmuxSupportsKittyUnicodePlaceholders.
func ResolveImageProtocol(cfg config.PreviewConfig, environ func(string) string) previewpanel.ImageProtocol {
	switch strings.ToLower(strings.TrimSpace(cfg.ImageProtocol)) {
	case config.PreviewImageProtocolSixel:
		return previewpanel.ImageProtocolSixel
	case config.PreviewImageProtocolKitty:
		return previewpanel.ImageProtocolKitty
	default:
		// auto (and unknown values)
		if environ == nil {
			return previewpanel.ImageProtocolSixel
		}
		sixel := strings.ToLower(strings.TrimSpace(cfg.TerminalSixel))
		kitty := strings.ToLower(strings.TrimSpace(cfg.TerminalKitty))
		if kitty == config.PreviewTerminalCapabilityYes && sixel != config.PreviewTerminalCapabilityYes {
			return previewpanel.ImageProtocolKitty
		}
		if sixel == config.PreviewTerminalCapabilityYes && kitty != config.PreviewTerminalCapabilityYes {
			return previewpanel.ImageProtocolSixel
		}
		guess := imageProtocolHeuristic(environ)
		if guess == previewpanel.ImageProtocolKitty && kitty == config.PreviewTerminalCapabilityNo {
			return previewpanel.ImageProtocolSixel
		}
		if guess == previewpanel.ImageProtocolSixel && sixel == config.PreviewTerminalCapabilityNo {
			return previewpanel.ImageProtocolKitty
		}
		return guess
	}
}

// TmuxSupportsKittyUnicodePlaceholders reports whether, under tmux, the actually-attached
// outer terminal is known to support Kitty's Unicode-placeholder image display: Kitty and
// Ghostty always, plus any terminal confirmed via cfg.TerminalKittyPlaceholder == "yes" (set
// through the M-F3 image-capabilities dialog). Unlike ResolveImageProtocol's "auto" path, this
// must be checked even when the Kitty protocol was reached via an explicit image_protocol=kitty
// config override: forcing Kitty protocol does not make an otherwise-unsupported terminal
// understand Unicode placeholders, so a caller under tmux that skips this check and uses
// placeholder mode anyway sends cells the terminal can't interpret — nothing renders, on any
// terminal that ends up here without support. environ is typically os.Getenv.
func TmuxSupportsKittyUnicodePlaceholders(environ func(string) string, cfg config.PreviewConfig) bool {
	if environ == nil || environ("TMUX") == "" {
		return false
	}
	return tmuxOuterTerminalSupportsUnicodePlaceholders(cfg)
}

// CapabilityUncertain reports whether the image protocol/placeholder capability that would be
// used for the next preview was decided by low-confidence guesswork rather than an explicit
// user confirmation or a strong environment signal — the predicate behind the bottom-left
// footer hint that opens the M-F3 image-capabilities dialog.
//
// An explicit cfg.ImageProtocol override (sixel/kitty, not auto) always answers false: the user
// already settled which protocol to use. Otherwise the protocol ResolveImageProtocol would pick
// is resolved, and only the tri-state(s) relevant to that protocol are checked:
//   - Sixel: uncertain whenever cfg.TerminalSixel is still "auto" — the Sixel branch is only
//     ever reached as a "nothing else matched" fallback, never a positive confirmation.
//   - Kitty: the base protocol choice is always either an explicit "yes" or a positive
//     environment match (imageProtocolHeuristic only returns Kitty when kittyGraphicsConfirmedByEnv
//     agrees), so it's never uncertain by itself. Under tmux, though, whether Unicode-placeholder
//     display is used is a separate question: uncertain whenever cfg.TerminalKittyPlaceholder is
//     still "auto" and tmuxOuterTerminalSupportsUnicodePlaceholders can't confirm it either (the
//     WezTerm-under-tmux case this feature exists for).
func CapabilityUncertain(cfg config.PreviewConfig, environ func(string) string) bool {
	switch strings.ToLower(strings.TrimSpace(cfg.ImageProtocol)) {
	case config.PreviewImageProtocolSixel, config.PreviewImageProtocolKitty:
		return false
	}
	if environ == nil {
		environ = func(string) string { return "" }
	}
	switch ResolveImageProtocol(cfg, environ) {
	case previewpanel.ImageProtocolSixel:
		return strings.ToLower(strings.TrimSpace(cfg.TerminalSixel)) != config.PreviewTerminalCapabilityYes &&
			strings.ToLower(strings.TrimSpace(cfg.TerminalSixel)) != config.PreviewTerminalCapabilityNo
	case previewpanel.ImageProtocolKitty:
		if environ("TMUX") == "" {
			return false
		}
		placeholder := strings.ToLower(strings.TrimSpace(cfg.TerminalKittyPlaceholder))
		if placeholder == config.PreviewTerminalCapabilityYes || placeholder == config.PreviewTerminalCapabilityNo {
			return false
		}
		return !tmuxOuterTerminalSupportsUnicodePlaceholders(cfg)
	default:
		return false
	}
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

// kittyOrGhosttyConfirmedByEnv is like kittyGraphicsConfirmedByEnv but deliberately excludes
// WezTerm: it identifies specifically Kitty or Ghostty, the only terminals whose Unicode-
// placeholder support can be auto-detected without an explicit user confirmation (see
// tmuxOuterTerminalSupportsUnicodePlaceholders and DetectTerminalCapabilities below) — WezTerm
// may be Kitty-graphics capable (kittyGraphicsConfirmedByEnv agrees) but its placeholder support
// is a build-specific capability neither TERM_PROGRAM/TERM nor client_termtype can confirm.
func kittyOrGhosttyConfirmedByEnv(environ func(string) string) bool {
	if environ("TMUX") != "" {
		t := tmuxClientTermType()
		return strings.Contains(t, "kitty") || strings.Contains(t, "ghostty")
	}
	prog := strings.ToLower(strings.TrimSpace(environ("TERM_PROGRAM")))
	if prog == "kitty" || prog == "ghostty" {
		return true
	}
	term := strings.ToLower(strings.TrimSpace(environ("TERM")))
	return term == "xterm-kitty" || strings.Contains(term, "ghostty")
}

// DetectTerminalCapabilities returns a best-guess snapshot of terminal graphics support from the
// environment/tmux introspection alone, ignoring any existing user tri-state confirmations in
// config — this is what seeds the M-F3 image-capabilities dialog's checkboxes when the user
// presses F5 "Auto detect".
//
//   - kitty mirrors ResolveImageProtocol's heuristic (kittyGraphicsConfirmedByEnv), so it
//     includes WezTerm.
//   - kittyPlaceholder is deliberately narrower (kittyOrGhosttyConfirmedByEnv, Kitty/Ghostty
//     only) since placeholder support on other Kitty-capable terminals like WezTerm cannot be
//     auto-detected — confirming it is the entire reason this dialog exists. It is always a
//     subset of kitty (kittyOrGhosttyConfirmedByEnv implies kittyGraphicsConfirmedByEnv), so this
//     never contradicts the dialog's own "placeholder implies Kitty" checkbox rule.
//   - sixel, under tmux, uses TmuxSupportsNativeSixel's real client_termfeatures signal. Outside
//     tmux there is no equivalent signal, so it's guessed as the logical complement of kitty —
//     the same "nothing else matched" fallback ResolveImageProtocol's own heuristic uses.
func DetectTerminalCapabilities(environ func(string) string) (sixel, kitty, kittyPlaceholder bool) {
	if environ == nil {
		environ = func(string) string { return "" }
	}
	kitty = kittyGraphicsConfirmedByEnv(environ)
	if environ("TMUX") != "" {
		sixel = TmuxSupportsNativeSixel(environ)
	} else {
		sixel = !kitty
	}
	kittyPlaceholder = kittyOrGhosttyConfirmedByEnv(environ)
	return sixel, kitty, kittyPlaceholder
}

// ResolveVideoThumbProtocol picks the graphics protocol for video thumbnail grids.
// When imagesEnabled is false ([preview].images), returns ImageProtocolNone.
// Otherwise uses the same auto/sixel/kitty resolution as still-image previews.
func ResolveVideoThumbProtocol(imagesEnabled bool, cfg config.PreviewConfig, environ func(string) string) previewpanel.ImageProtocol {
	if !imagesEnabled {
		return previewpanel.ImageProtocolNone
	}
	return ResolveImageProtocol(cfg, environ)
}
