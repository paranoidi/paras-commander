package app

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// imageOverlay tracks the currently locked/emitted terminal image on the tty, for the
// cursor-relative placement path (outside tmux, or non-Kitty protocols under tmux — see
// placeholderImage below for the tmux+Kitty path).
type imageOverlay struct {
	last        previewpanel.ImagePlacement
	lastSet     bool
	lastCols    int
	lastRows    int
	pendingEmit bool
	// pendingDelete is set when a previously emitted Kitty image must be deleted
	// before the next Show (plan cleared or replaced).
	pendingDelete bool
}

// placeholderImage tracks the Kitty image data most recently transmitted for tmux
// Unicode-placeholder display (previewpanel.ImagePlacement.UnicodePlaceholder). Positioning
// for this mode is handled entirely by normal screen cells that previewpanel.Draw writes, so
// unlike imageOverlay this only needs to track *data* transmission, independent of cursor
// position, geometry, or LockRegion.
type placeholderImage struct {
	sent    bool
	payload string
}

func (a *App) imageOverlaySuppressed() bool {
	return a.model.ModalDialogOpen() || a.model.Menu.Open
}

// reconcileImageBeforeShow updates LockRegion state for the upcoming Show().
// Returns true when the locked region changed (forces Show past the hash cache).
func (a *App) reconcileImageBeforeShow(plan *previewpanel.ImagePlacement) (forceShow bool) {
	if plan != nil && plan.UnicodePlaceholder {
		changed := a.reconcilePlaceholderImage(plan)
		// Defensive: tear down any leftover cursor-relative overlay state. Not expected in
		// practice (protocol/tmux status doesn't change mid-session), but keeps the two
		// mechanisms from ever fighting over the same locked region.
		if a.image.lastSet {
			a.screen.LockRegion(a.image.last.X, a.image.last.Y, a.image.lastCols, a.image.lastRows, false)
			a.image = imageOverlay{}
			return true
		}
		return changed
	}
	if a.placeholderImg.sent {
		if a.reconcilePlaceholderImage(nil) {
			forceShow = true
		}
	}

	var cols, rows int
	if plan != nil {
		cw, ch := previewpanel.CellPixelDims(a.screen)
		cols = (plan.PxW + cw - 1) / cw
		rows = (plan.PxH + ch - 1) / ch
		if cols < 1 {
			cols = 1
		}
		if rows < 1 {
			rows = 1
		}
		if cols > plan.MaxCols || rows > plan.MaxRows {
			plan = nil
		}
	}

	// Path is ignored: during stale-while-revalidate the held payload is shown under the
	// new filename; tearing down the overlay for a path-only change flashes the cell buffer.
	if plan != nil && a.image.lastSet &&
		plan.Payload == a.image.last.Payload &&
		plan.Protocol == a.image.last.Protocol &&
		plan.X == a.image.last.X &&
		plan.Y == a.image.last.Y &&
		cols == a.image.lastCols &&
		rows == a.image.lastRows {
		a.image.last.Path = plan.Path
		return false
	}

	if a.image.lastSet {
		wasKitty := a.image.last.Protocol == previewpanel.ImageProtocolKitty
		// Per the Kitty graphics spec, re-transmitting image data under the same id requires
		// deleting the existing image and its placements first; a bare re-transmit without
		// that delete is undefined behavior. Some terminals (Kitty itself) tolerate skipping
		// it, but others (WezTerm) do not: without the delete, successive replacements can
		// each remain visible and get flushed to the screen as a visible slideshow instead of
		// cleanly replacing in place. Always delete before installing a different Kitty image.
		a.screen.LockRegion(a.image.last.X, a.image.last.Y, a.image.lastCols, a.image.lastRows, false)
		a.image.lastSet = false
		a.image.last = previewpanel.ImagePlacement{}
		a.image.lastCols = 0
		a.image.lastRows = 0
		if wasKitty {
			a.image.pendingDelete = true
		}
		forceShow = true
	}

	if plan != nil {
		a.screen.LockRegion(plan.X, plan.Y, cols, rows, true)
		a.image.last = *plan
		a.image.lastSet = true
		a.image.lastCols = cols
		a.image.lastRows = rows
		a.image.pendingEmit = true
		forceShow = true
	}
	return forceShow
}

// emitImageAfterShow writes pending Kitty deletes and/or the image payload to the tty for the
// cursor-relative placement path. No-op for tmux Unicode-placeholder images: those are
// transmitted synchronously in reconcilePlaceholderImage (before Show, since the terminal
// must know the image data before Show draws the placeholder cells that reference it) and
// displayed via normal screen cells, not an out-of-band write here.
//
// This path is still reachable under tmux: Sixel always uses it (no placeholder equivalent
// exists for Sixel), and so does Kitty when the outer terminal isn't confirmed to support
// Unicode placeholders — Kitty and Ghostty always are; other terminals (e.g. WezTerm) only if
// the user has confirmed it via [preview].terminal_kitty_placeholder = "yes" (the M-F3
// image-capabilities dialog), since client_termtype alone can't confirm placeholder support
// for them.
// tmux has no native understanding of Kitty's escape sequences — it must be told to forward
// them verbatim via passthrough (writeImagePayload/writeKittyDelete below). Sixel is different:
// when the attached outer terminal's tmux-resolved features include sixel
// (preview.TmuxSupportsNativeSixel), tmux parses a bare sixel DCS itself, stores the image, and
// redraws it after every tmux-side invalidate — so it is sent unwrapped, exactly like the
// no-tmux path below, instead of passthrough-wrapped.
func (a *App) emitImageAfterShow() {
	tty, ok := a.screen.Tty()
	if !ok {
		a.image.pendingEmit = false
		a.image.pendingDelete = false
		return
	}
	if a.image.pendingDelete {
		a.image.pendingDelete = false
		writeKittyDelete(tty)
	}
	if !a.image.pendingEmit {
		return
	}
	a.image.pendingEmit = false
	p := a.image.last
	bareNativeSixel := p.Protocol == previewpanel.ImageProtocolSixel && preview.TmuxSupportsNativeSixel(os.Getenv)
	if inTmux() && !bareNativeSixel {
		// tmux only positions the *outer* terminal's cursor when the pane cursor is visible,
		// and does so from its own event loop — while a passthrough-wrapped payload bypasses
		// that loop and reaches the outer terminal directly. A tcell app runs with the cursor
		// hidden, so without this ceremony the image lands at a stale outer-cursor position
		// (Sixel at the bottom row scrolls the whole screen). Match yazi's Emulator::move_lock
		// tmux path: save cursor, then move + show repeatedly (each write hits the tty
		// unbuffered), give tmux a moment to redraw the outer cursor, send the payload, then
		// hide + restore. Synchronized output (?2026) must NOT wrap this under tmux — it
		// defers exactly the redraw the sleep is waiting for.
		_, _ = fmt.Fprintf(tty, "\x1b7\x1b[%d;%dH\x1b[?25h", p.Y+1, p.X+1)
		_, _ = fmt.Fprintf(tty, "\x1b[%d;%dH\x1b[?25h", p.Y+1, p.X+1)
		_, _ = fmt.Fprintf(tty, "\x1b[%d;%dH\x1b[?25h", p.Y+1, p.X+1)
		time.Sleep(time.Millisecond)
		writeImagePayload(tty, p.Payload, p.Protocol)
		_, _ = fmt.Fprint(tty, "\x1b[?25l\x1b8")
		return
	}
	// Bare native sixel under tmux takes this branch too: tmux parses it inline as normal pane
	// content via its own screen cursor, the same as a real terminal would, so no passthrough
	// cursor ceremony is needed — plain CUP positions tmux's pane cursor correctly.
	_, _ = fmt.Fprintf(tty, "\x1b[?2026h\x1b7\x1b[%d;%dH", p.Y+1, p.X+1)
	writeImagePayload(tty, p.Payload, p.Protocol)
	_, _ = fmt.Fprint(tty, "\x1b8\x1b[?2026l")
}

func inTmux() bool {
	return os.Getenv("TMUX") != ""
}

// writeImagePayload writes an already-encoded image payload (Sixel or Kitty) to w. Outside
// tmux, or for Sixel when the attached outer terminal's tmux-resolved features include sixel
// (preview.TmuxSupportsNativeSixel), it's written bare — tmux then parses and stores the sixel
// image itself, same as any other terminal would. Otherwise, under tmux, it's split into its
// individual ST-terminated escape sequences and each is wrapped separately in tmux's
// passthrough envelope (see splitTerminatedSequences/tmuxPassthroughWrap for why: several
// ST-terminated sequences inside one outer wrap is a documented tmux bug). Sixel payloads are a
// single DCS sequence with no internal ESC bytes besides its own introducer/terminator, so
// splitting yields exactly one piece — this still wraps the whole thing in one envelope, just
// via the same general-purpose path Kitty's chunks use.
func writeImagePayload(w io.Writer, payload string, proto previewpanel.ImageProtocol) {
	if !inTmux() {
		_, _ = io.WriteString(w, payload)
		return
	}
	if proto == previewpanel.ImageProtocolSixel && preview.TmuxSupportsNativeSixel(os.Getenv) {
		_, _ = io.WriteString(w, payload)
		return
	}
	for _, chunk := range splitTerminatedSequences(payload) {
		_, _ = io.WriteString(w, tmuxPassthroughWrap(chunk))
	}
}

func writeKittyDelete(w io.Writer) {
	seq := kittyDeleteSequence()
	if inTmux() {
		seq = tmuxPassthroughWrap(seq)
	}
	_, _ = io.WriteString(w, seq)
}

// resetImageOverlay unlocks any locked region, deletes a Kitty image if needed, and clears
// both cursor-relative and Unicode-placeholder overlay state. Clearing placeholderImg is
// required so the next render re-transmits image data after Suspend/Resume (external editor)
// or Sync, which typically wipe the terminal's graphics registry — otherwise
// reconcilePlaceholderImage would early-out on an unchanged payload and leave placeholder
// cells with no backing image.
func (a *App) resetImageOverlay() {
	needKittyDelete := a.placeholderImg.sent ||
		(a.image.lastSet && a.image.last.Protocol == previewpanel.ImageProtocolKitty)
	if a.image.lastSet {
		a.screen.LockRegion(a.image.last.X, a.image.last.Y, a.image.lastCols, a.image.lastRows, false)
	}
	if needKittyDelete {
		if tty, ok := a.screen.Tty(); ok {
			writeKittyDelete(tty)
		}
	}
	a.image = imageOverlay{}
	a.placeholderImg = placeholderImage{}
}

// resetImageOverlayForResize is resetImageOverlay's resize call site. Plain resetImageOverlay
// unconditionally clears the tracked placement, which forces the next reconcileImageBeforeShow
// to treat the image as brand new and always retransmit it — even when its content and
// position turn out to be unchanged by the resize. For a bare-native-sixel image under tmux,
// this leaves the tracked placement alone instead, so the normal payload/position comparison
// decides whether anything actually needs to move: tmux frees its own copy of the image on
// resize (see internal/apphandler/preview.refreshPreviewTargetAfterResize, which under the same
// condition also skips regenerating the payload itself), so the preview simply goes blank until
// something else reloads it, instead of paying for an eager re-decode/re-encode/retransmit on
// every single resize event. Passthrough Sixel and Kitty always reset here regardless: tmux
// never stores those, and a resize-driven Sync can otherwise leave them undisplayed.
func (a *App) resetImageOverlayForResize() {
	if a.image.lastSet && a.image.last.Protocol == previewpanel.ImageProtocolSixel &&
		preview.TmuxSupportsNativeSixel(os.Getenv) {
		return
	}
	a.resetImageOverlay()
}

// reconcilePlaceholderImage (re)transmits the Kitty image data backing a tmux
// Unicode-placeholder display when the payload changed, deleting the previous transmission
// under the same fixed id first (same rule as the cursor-relative path). plan == nil (or a
// non-placeholder plan) clears the tracked state and deletes the last transmission, if any.
// This is purely about registering image *data* with the terminal — position-independent, so
// unlike emitImageAfterShow it runs synchronously here, before Show, since the terminal must
// already know the image data by the time Show draws the placeholder cells referencing it.
// Returns true when a delete or transmit was actually sent, so the caller can force Show()
// past the render hash-cache: the placeholder grid's cell bytes (rune/diacritics/color) encode
// only row, column, and the fixed KittyGraphicsImageID, never which image is currently backing
// that id, so two different images at the same on-screen grid size produce byte-for-byte
// identical cell content — the hash cache can't see the change and, uncorrected, Show() (and so
// the terminal redraw the real Kitty terminal needs to notice the new data) gets skipped even
// though fresh image bytes just went out (see llm-docs/graphics-implementation-lessons.md
// lesson 15).
func (a *App) reconcilePlaceholderImage(plan *previewpanel.ImagePlacement) (changed bool) {
	if plan == nil || !plan.UnicodePlaceholder {
		if !a.placeholderImg.sent {
			return false
		}
		a.placeholderImg = placeholderImage{}
		if tty, ok := a.screen.Tty(); ok {
			_, _ = io.WriteString(tty, tmuxPassthroughWrap(kittyDeleteSequence()))
		}
		return true
	}
	if a.placeholderImg.sent && a.placeholderImg.payload == plan.Payload {
		return false
	}
	tty, ok := a.screen.Tty()
	if !ok {
		return false
	}
	if a.placeholderImg.sent {
		_, _ = io.WriteString(tty, tmuxPassthroughWrap(kittyDeleteSequence()))
	}
	for _, chunk := range splitTerminatedSequences(plan.Payload) {
		_, _ = io.WriteString(tty, tmuxPassthroughWrap(chunk))
	}
	a.placeholderImg = placeholderImage{sent: true, payload: plan.Payload}
	return true
}

func kittyDeleteSequence() string {
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d\x1b\\", previewpanel.KittyGraphicsImageID)
}

// tmuxPassthroughWrap wraps a single already-terminated escape sequence in tmux's DCS
// passthrough envelope (`set -g allow-passthrough on` in tmux.conf) so tmux forwards it
// verbatim to the outer terminal instead of interpreting it as pane content. Every ESC byte
// in the wrapped sequence must be doubled per the tmux passthrough format.
func tmuxPassthroughWrap(seq string) string {
	return "\x1bPtmux;" + strings.ReplaceAll(seq, "\x1b", "\x1b\x1b") + "\x1b\\"
}

// splitTerminatedSequences splits payload into its individual ST-terminated (`\x1b\\`)
// escape sequences, e.g. the concatenated per-chunk Kitty APC transmit sequences. Each one
// needs its own tmux passthrough wrap: wrapping several ST-terminated inner sequences inside
// one outer wrap is a documented tmux bug (spurious characters in the output).
func splitTerminatedSequences(payload string) []string {
	const st = "\x1b\\"
	var out []string
	for {
		idx := strings.Index(payload, st)
		if idx < 0 {
			if payload != "" {
				out = append(out, payload)
			}
			return out
		}
		out = append(out, payload[:idx+len(st)])
		payload = payload[idx+len(st):]
	}
}
