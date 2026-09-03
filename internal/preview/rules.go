package preview

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/entrymatch"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// RunRules tries req.Preview.Commands in order against req.Path/req.IsDir. For each rule whose
// When predicates match, the rule's command runs: exit 0 means the command took responsibility
// for the preview (its stdout becomes the Result, as plain/ANSI text or — when it starts with a
// Sixel or Kitty graphics escape sequence — a raw graphics payload), and that Result is
// returned immediately. A non-zero exit means the rule declined; the next matching rule is
// tried. matched is false when no rule matched, or every matching rule declined — the caller
// falls back to its own default behavior (Mode/Command, internal preview, or the directory
// overlay).
func RunRules(ctx context.Context, req Request) (Result, bool) {
	if len(req.Preview.Commands) == 0 {
		return Result{}, false
	}
	entCtx := &entrymatch.Context{
		Row:           &localfs.Entry{Name: filepath.Base(req.Path), Type: entryTypeFor(req.Path, req.IsDir)},
		PanelDir:      filepath.Dir(req.Path),
		ShellPatterns: req.Preview.ShellPatterns,
	}
	for _, rule := range req.Preview.Commands {
		ok, err := entrymatch.EvalWhenAny(rule.When, entCtx)
		if err != nil || !ok {
			continue
		}
		if res, exitOK := runRuleCommand(ctx, req, rule); exitOK {
			return res, true
		}
	}
	return Result{}, false
}

// MatchAnyCommandRule is a cheap, synchronous "would any rule's when fire" pre-check — it runs
// no subprocess. Used before dispatching a directory through the async rule-run path, so the
// caller only leaves the built-in directory listing when a rule could plausibly take over.
func MatchAnyCommandRule(cfg config.PreviewConfig, path string, isDir bool, panelDir string) bool {
	if len(cfg.Commands) == 0 {
		return false
	}
	entCtx := &entrymatch.Context{
		Row:           &localfs.Entry{Name: filepath.Base(path), Type: entryTypeFor(path, isDir)},
		PanelDir:      panelDir,
		ShellPatterns: cfg.ShellPatterns,
	}
	for _, rule := range cfg.Commands {
		if ok, err := entrymatch.EvalWhenAny(rule.When, entCtx); err == nil && ok {
			return true
		}
	}
	return false
}

// entryTypeFor reports isDir directly when the caller already knows it (avoids a redundant
// stat); otherwise it stats path, defaulting to EntryFile on a stat error since a rule that
// requires a directory/symlink match should simply not match a path that can't be inspected.
func entryTypeFor(path string, isDir bool) localfs.EntryType {
	if isDir {
		return localfs.EntryDirectory
	}
	info, err := os.Lstat(path)
	if err != nil {
		return localfs.EntryFile
	}
	return localfs.ClassifyMode(info.Mode())
}

func runRuleCommand(ctx context.Context, req Request, rule config.PreviewCommandRule) (Result, bool) {
	argv, err := cmdrun.PreviewCommandArgv(rule.Command, req.Path, req.TextWidth)
	if err != nil {
		return Result{}, false
	}
	// Resolved independently of req.Image/req.ImageProtocol (those describe pc's own built-in
	// image path, not yet known to apply here) — this only answers the rule command's own DA1
	// terminal query, if it sends one.
	sixelOK := req.Preview.Images && ResolveImageProtocol(req.Preview, os.Getenv) == previewpanel.ImageProtocolSixel
	res := runRuleCommandCapture(ctx, argv, req.WorkDir, config.DefaultPreviewCommandsMaxStreamBytes, sixelOK)
	if res.LaunchErr != nil || res.ExitCode != 0 {
		return Result{}, false
	}
	if res.StdoutTrim {
		// Truncated stdout can't be trusted: a partial Sixel/Kitty escape sequence corrupts the
		// terminal instead of just looking cut off, and partial text is a confusing preview.
		// Treat like a decline rather than showing it.
		return Result{}, false
	}
	if imagePart, textPart, proto, ok := splitGraphicsPayload(res.Stdout); ok {
		// A rule command has no way to know it's being embedded in pc's own cell grid, so it
		// only ever sends a standard cursor-relative auto-display transmission (the only thing
		// that makes sense for a standalone tool) — even under tmux, where that's fragile (see
		// image_overlay.go's cursor-ceremony comments) and where pc's own images instead use
		// Kitty's Unicode-placeholder display specifically to avoid it. This is the one place
		// that gap can be closed: rewrite the command's own chunks onto pc's fixed placeholder
		// image id and request placeholder display, exactly like encodeKittyAPC does for pc's
		// own images — the rest of the pipeline (drawUnicodePlaceholderImage,
		// reconcilePlaceholderImage) already treats any payload the same way regardless of
		// source. This also incidentally removes the "declared size vs. real rendered size"
		// mismatch below for Kitty: unlike cursor-relative mode, placeholder mode has the
		// terminal fit the image to however many placeholder cells reference it, so whatever
		// ImagePxH ends up being IS what renders — not a guess that can drift from reality.
		unicodePlaceholder := proto == previewpanel.ImageProtocolKitty &&
			TmuxSupportsKittyUnicodePlaceholders(os.Getenv, req.Preview)
		if unicodePlaceholder {
			imagePart = rewriteKittyForPlaceholder(imagePart)
		}
		result := Result{
			Source:                  previewpanel.SourceExternalANSI,
			ImagePayload:            string(imagePart),
			ImagePxW:                req.ImageMaxPxW,
			ImagePxH:                req.ImageMaxPxH,
			ImageProtocol:           proto,
			ImageUnicodePlaceholder: unicodePlaceholder,
			ExitCode:                0,
		}
		if len(textPart) > 0 && req.ImageCellPxH > 0 {
			// previewpanel.TotalLines runs the exact same wrapper drawImageBody will use for
			// this CombinedText, so the reserved row count can't drift from the real one the
			// way a hand-rolled estimate (raw newline count, or a character-count approximation
			// that isn't word-wrap-aware) could — both undercounted once a tool's own wrap width
			// (e.g. movie-info's default 79 cols) didn't match the pane's actual, often-narrower
			// req.TextWidth, causing the renderer to re-wrap into more rows than reserved.
			captionRows := previewpanel.TotalLines(
				previewpanel.State{Source: previewpanel.SourceExternalANSI, CombinedText: string(textPart)},
				req.TextWidth, req.BaseStyle)
			if captionRows < 1 {
				captionRows = 1
			}
			const captionSeparatorRows = 1
			contentRows := req.ImageMaxPxH / req.ImageCellPxH
			availableImageRows := contentRows - captionRows - captionSeparatorRows
			if availableImageRows < 0 {
				availableImageRows = 0
			}
			// Default: declare the image as filling exactly the space left after reserving the
			// caption. This is only an approximation of what the terminal will actually render —
			// pc doesn't (and can't, without a decode/re-encode round trip) resize a rule
			// command's own pre-encoded Sixel/Kitty bytes, so the real rendered size is whatever
			// pixel dimensions the command itself baked in, which this reservation knows nothing
			// about. Below, use the payload's own declared dimensions when present and they fit,
			// so the caption lands immediately after the image as actually rendered instead of
			// leaving a gap (or overlapping it) whenever the guess doesn't match reality.
			result.ImagePxH = availableImageRows * req.ImageCellPxH
			if realPxW, realPxH, haveDims := imagePixelDims(imagePart, proto); haveDims {
				realRows := (realPxH + req.ImageCellPxH - 1) / req.ImageCellPxH
				if realRows <= availableImageRows {
					// Height alone decides whether the real size is trustworthy: it's what the
					// caption's vertical position depends on. Width is only clamped (never used
					// to reject the real height) — the terminal renders the tool's own real
					// width regardless of what's declared here; declaring more than the pane's
					// budget would only get the whole placement dropped by
					// reconcileImageBeforeShow's cols>MaxCols check, not actually shrink it.
					result.ImagePxW = min(realPxW, req.ImageMaxPxW)
					result.ImagePxH = realPxH
				}
				// else the real image is taller than what's available alongside the caption:
				// keep the reservation-based guess above — a known approximation for this
				// overflow case, better than dropping the caption or the image outright.
			}
			result.CombinedText = string(textPart)
			// splitGraphicsPayload only ever splits an image found at/near the start of stdout
			// from real text found after it (see its doc comment) — so whenever there's a
			// caption here, the command's own output order was image-then-text. Render it the
			// same way instead of always defaulting to caption-above-image.
			result.ImageFirst = true
		}
		return result, true
	}
	// StdoutTrim is always false here (handled above); only stderr can still be truncated.
	combined, truncated := combineStdoutStderr(res.Stdout, res.Stderr, res.StderrTrim)
	return Result{
		Source:       previewpanel.SourceExternalANSI,
		CombinedText: combined,
		ExitCode:     0,
		Truncated:    truncated,
	}, true
}

// graphicsSniffWindow bounds how far into stdout sniffGraphicsProtocol looks for a Sixel/Kitty
// introducer. Real image tools (chafa, img2sixel, kitty +kitten icat) commonly emit a handful of
// harmless cursor-visibility/mode CSI sequences (e.g. "\x1b[?25l") before the actual DCS/APC
// image data, so the introducer is not necessarily the very first bytes; a bounded window still
// keeps this from ever mistaking a large plain-text/ANSI preview for graphics.
const graphicsSniffWindow = 4096

// sniffGraphicsProtocol reports whether stdout is raw terminal graphics rather than text: a
// Sixel DCS (ESC P) or a Kitty graphics APC (ESC _ G) within its first graphicsSniffWindow bytes.
func sniffGraphicsProtocol(stdout []byte) (previewpanel.ImageProtocol, bool) {
	probe := stdout
	if len(probe) > graphicsSniffWindow {
		probe = probe[:graphicsSniffWindow]
	}
	switch {
	case bytes.Contains(probe, []byte("\x1bP")):
		return previewpanel.ImageProtocolSixel, true
	case bytes.Contains(probe, []byte("\x1b_G")):
		return previewpanel.ImageProtocolKitty, true
	default:
		return 0, false
	}
}

// imagePixelDims reports the encoded pixel dimensions a Sixel or Kitty payload declares for
// itself, if any. Sixel's raster attributes and Kitty's s=/v= transmit params are both optional
// in their respective specs, so ok is false when a payload doesn't declare them — callers fall
// back to an assumed size in that case.
func imagePixelDims(imagePart []byte, proto previewpanel.ImageProtocol) (w, h int, ok bool) {
	switch proto {
	case previewpanel.ImageProtocolSixel:
		return sixelPixelDims(imagePart)
	case previewpanel.ImageProtocolKitty:
		if w, h, ok := kittyPixelDims(imagePart); ok {
			return w, h, true
		}
		return kittyPNGPixelDims(imagePart)
	default:
		return 0, 0, false
	}
}

// sixelPixelDims parses width/height (pixels) from a Sixel DCS's raster attributes header
// ("Pan;Pad;Ph;Pv — e.g. the "1;1;500;750 in "\x1bP0;1q\"1;1;500;750#0;...").
func sixelPixelDims(payload []byte) (w, h int, ok bool) {
	idx := bytes.IndexByte(payload, '"')
	if idx < 0 {
		return 0, 0, false
	}
	rest := payload[idx+1:]
	if end := bytes.IndexByte(rest, '#'); end >= 0 {
		rest = rest[:end]
	}
	fields := strings.Split(string(rest), ";")
	if len(fields) < 4 {
		return 0, 0, false
	}
	ph, errW := strconv.Atoi(strings.TrimSpace(fields[2]))
	pv, errH := strconv.Atoi(strings.TrimSpace(fields[3]))
	if errW != nil || errH != nil || ph <= 0 || pv <= 0 {
		return 0, 0, false
	}
	return ph, pv, true
}

// kittyPixelDims parses the s=/v= (width/height, pixels) control-data keys from a Kitty graphics
// transmit command's first chunk ("\x1b_G<key=value,...>;<payload>\x1b\\"), if present — many
// Kitty transmissions omit them for a self-describing format like PNG (f=100), where the
// terminal is expected to derive size from the image data itself instead; see
// kittyPNGPixelDims for that case. ok is false when s=/v= aren't declared.
func kittyPixelDims(payload []byte) (w, h int, ok bool) {
	if !bytes.HasPrefix(payload, []byte("\x1b_G")) {
		return 0, 0, false
	}
	rest := payload[len("\x1b_G"):]
	semi := bytes.IndexByte(rest, ';')
	if semi < 0 {
		return 0, 0, false
	}
	for _, kv := range strings.Split(string(rest[:semi]), ",") {
		k, v, found := strings.Cut(kv, "=")
		if !found {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		switch k {
		case "s":
			w = n
		case "v":
			h = n
		}
	}
	if w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// kittyPNGPixelDims recovers width/height for a PNG-format (f=100) Kitty transmission that
// omitted s=/v=, by decoding just enough of the transmitted data to read the PNG's own IHDR
// chunk — the image format's authoritative source for its dimensions. A PNG's IHDR is always
// its very first chunk, well within the first ~48 decoded bytes regardless of overall image
// size, so this gathers that much base64 across as many of imagePart's own chunks as needed
// (never assuming any one chunk's payload length is independently a multiple of 4, since a
// third-party sender's own chunking scheme is unknown) rather than decoding the whole image.
func kittyPNGPixelDims(imagePart []byte) (w, h int, ok bool) {
	if !bytes.HasPrefix(imagePart, []byte("\x1b_G")) {
		return 0, 0, false
	}
	firstBody := imagePart[len("\x1b_G"):]
	semi := bytes.IndexByte(firstBody, ';')
	if semi < 0 {
		return 0, 0, false
	}
	isPNG := false
	for _, kv := range strings.Split(string(firstBody[:semi]), ",") {
		if k, v, found := strings.Cut(kv, "="); found && k == "f" && v == "100" {
			isPNG = true
		}
	}
	if !isPNG {
		return 0, 0, false
	}

	const need = 64 // base64 chars; already a multiple of 4, decodes to 48 bytes
	var b64 []byte
	rest := imagePart
	for len(b64) < need && bytes.HasPrefix(rest, []byte("\x1b_G")) {
		body := rest[len("\x1b_G"):]
		s := bytes.IndexByte(body, ';')
		if s < 0 {
			break
		}
		body = body[s+1:]
		term := bytes.Index(body, []byte("\x1b\\"))
		if term < 0 {
			break
		}
		b64 = append(b64, body[:term]...)
		rest = rest[len("\x1b_G")+s+1+term+len("\x1b\\"):]
	}
	if len(b64) > need {
		b64 = b64[:need]
	}
	b64 = b64[:len(b64)-len(b64)%4] // trim to a valid base64 group boundary
	if len(b64) == 0 {
		return 0, 0, false
	}
	decoded, err := base64.StdEncoding.DecodeString(string(b64))
	if err != nil {
		return 0, 0, false
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(decoded))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

// splitGraphicsPayload separates a rule's raw stdout into the graphics chunk (Sixel/Kitty
// sequence, including any leading CSI wrapper before the introducer that tools like chafa emit,
// e.g. "\x1b[?25l", and any immediately-trailing CSI noise like a paired "\x1b[?25h") and any
// real trailing text content (e.g. movie-info's metadata lines after the poster). ok is false
// when stdout contains no recognized graphics introducer at all.
//
// Kitty's graphics protocol caps a single APC transmission at 4096 bytes of payload, so any
// image bigger than that — routine for a base64-encoded poster — arrives as consecutive chunks
// ("\x1b_G...,m=1,...;<payload>\x1b\\" repeated, the last with m=0 or no m key). All of them
// belong to the image; only the byte(s) after the last one are real trailing content.
func splitGraphicsPayload(stdout []byte) (imagePart, textPart []byte, proto previewpanel.ImageProtocol, ok bool) {
	proto, ok = sniffGraphicsProtocol(stdout)
	if !ok {
		return nil, nil, 0, false
	}
	introBytes := []byte("\x1bP")
	if proto == previewpanel.ImageProtocolKitty {
		introBytes = []byte("\x1b_G")
	}
	probe := stdout
	if len(probe) > graphicsSniffWindow {
		probe = probe[:graphicsSniffWindow]
	}
	introIdx := bytes.Index(probe, introBytes)
	if introIdx < 0 {
		return nil, nil, 0, false
	}

	var end int
	if proto == previewpanel.ImageProtocolKitty {
		chunkEnd, more, chunkOK := kittyChunkEnd(stdout, introIdx)
		if !chunkOK {
			// No terminator on the first chunk — can't safely split without risking cutting
			// mid-sequence. Treat the whole thing as the image, matching the Sixel fallback.
			return stdout, nil, proto, true
		}
		end = chunkEnd
		for more && bytes.HasPrefix(stdout[end:], introBytes) {
			nextEnd, nextMore, nextOK := kittyChunkEnd(stdout, end)
			if !nextOK {
				break
			}
			end = nextEnd
			more = nextMore
		}
	} else {
		termIdx := bytes.Index(stdout[introIdx:], []byte("\x1b\\"))
		if termIdx < 0 {
			return stdout, nil, proto, true
		}
		end = introIdx + termIdx + len("\x1b\\")
	}

	imagePart = stdout[:end]
	rest := stdout[end:]
	noise, textRest := stripLeadingCSISequences(rest)
	imagePart = append(append([]byte(nil), imagePart...), noise...)
	textPart = bytes.Trim(textRest, "\r\n")
	return imagePart, textPart, proto, true
}

// kittyChunkEnd finds the end (exclusive, past the "\x1b\\" terminator) of one Kitty APC chunk
// starting at stdout[start:], which must begin with "\x1b_G". more reports whether the chunk's
// own control data (the comma-separated key=value pairs before its first ';') declares "m=1" —
// i.e. more chunks follow this one. ok is false when start doesn't point at a chunk, or the
// chunk has no terminator in stdout.
func kittyChunkEnd(stdout []byte, start int) (end int, more bool, ok bool) {
	rest := stdout[start:]
	if !bytes.HasPrefix(rest, []byte("\x1b_G")) {
		return 0, false, false
	}
	termIdx := bytes.Index(rest, []byte("\x1b\\"))
	if termIdx < 0 {
		return 0, false, false
	}
	end = start + termIdx + len("\x1b\\")
	control := rest[len("\x1b_G"):]
	if semi := bytes.IndexByte(control, ';'); semi >= 0 {
		control = control[:semi]
	}
	for _, kv := range bytes.Split(control, []byte(",")) {
		k, v, found := bytes.Cut(kv, []byte("="))
		if found && string(k) == "m" && string(v) == "1" {
			more = true
		}
	}
	return end, more, true
}

// rewriteKittyForPlaceholder rewrites every chunk of a Kitty transmission (imagePart, as
// produced by splitGraphicsPayload — a whole sequence of consecutive "\x1b_G...\x1b\\" chunks)
// onto pc's fixed placeholder image id, and adds Unicode-placeholder display flags to the first
// chunk — see the call site in runRuleCommand for why. Chunks that don't parse (shouldn't happen
// given they came from splitGraphicsPayload) pass through unchanged.
func rewriteKittyForPlaceholder(imagePart []byte) []byte {
	var out bytes.Buffer
	rest := imagePart
	first := true
	for bytes.HasPrefix(rest, []byte("\x1b_G")) {
		termIdx := bytes.Index(rest, []byte("\x1b\\"))
		if termIdx < 0 {
			break
		}
		chunkEnd := termIdx + len("\x1b\\")
		out.Write(rewriteKittyChunk(rest[:chunkEnd], first))
		rest = rest[chunkEnd:]
		first = false
	}
	out.Write(rest)
	return out.Bytes()
}

// rewriteKittyChunk rewrites one "\x1b_G<control>;<payload>\x1b\\" chunk's control-data: forces
// i=<previewpanel.KittyGraphicsImageID> (drawUnicodePlaceholderImage hardcodes that id when it
// draws the placeholder cells, so the transmitted data must use the same one), and — first only
// — adds U=1,q=2 to request Unicode-placeholder display instead of cursor-relative auto-display,
// exactly matching internal/preview/image.go's encodeKittyAPC for pc's own images.
func rewriteKittyChunk(chunk []byte, first bool) []byte {
	body := chunk[len("\x1b_G"):]
	semi := bytes.IndexByte(body, ';')
	if semi < 0 {
		return chunk
	}
	control, rest := body[:semi], body[semi:] // rest keeps its leading ';' through the terminator

	var kept []string
	for _, kv := range strings.Split(string(control), ",") {
		k, _, found := strings.Cut(kv, "=")
		if !found {
			continue
		}
		switch k {
		case "i", "U", "q":
			continue // overridden below
		}
		kept = append(kept, kv)
	}
	kept = append(kept, "i="+strconv.Itoa(previewpanel.KittyGraphicsImageID))
	if first {
		kept = append(kept, "U=1", "q=2")
	}

	var out bytes.Buffer
	out.WriteString("\x1b_G")
	out.WriteString(strings.Join(kept, ","))
	out.Write(rest)
	return out.Bytes()
}

// stripLeadingCSISequences consumes complete CSI sequences (ESC '[' params... final-byte, final
// byte in 0x40-0x7E) from the front of b — e.g. a cursor-visibility toggle some image tools send
// right after drawing (chafa's trailing "\x1b[?25h"). These belong bundled with the image, not
// shown as caption text. Stops at the first byte that isn't part of a well-formed CSI sequence.
func stripLeadingCSISequences(b []byte) (consumed, rest []byte) {
	i := 0
	for i+1 < len(b) && b[i] == 0x1b && b[i+1] == '[' {
		j := i + 2
		for j < len(b) && b[j] >= 0x20 && b[j] <= 0x3F {
			j++
		}
		if j >= len(b) || b[j] < 0x40 || b[j] > 0x7E {
			break
		}
		i = j + 1
	}
	return b[:i], b[i:]
}
