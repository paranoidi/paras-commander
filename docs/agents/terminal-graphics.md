# Terminal graphics: protocols, tmux and terminal quirks

The "why" behind inline image previews (quick view / fullscreen / carousel). Code pointers, data flow and invariants are in [preview.md](preview.md) ("Image previews"). User docs: `docs/graphics.md`. Lesson numbers are cited from code comments; keep them stable.

Lesson index (current conclusion first): L1 Kitty delete-before-retransmit; L2 tmux needs passthrough for Kitty; L3/L8/L14 cursor-position race; L4 Unicode placeholders; L5 tmux 1 MB sequence limit; L6 superseded by L9/L19; L7 one decision point; L9/L19 native Sixel under tmux; L10 tmux `main` differs; L11 Sixel vanish gap (open); L12 superseded by L13; L13 Kitty for WezTerm under tmux (deliberate trade); L15 placeholder redraw trigger; L16 per-build WezTerm placeholder support; L17/L18 tmux measurements; L19 native is the only correct Sixel transport.

## Protocols and selection

- `[preview].image_protocol` = `auto` (default) | `sixel` | `kitty` | `ueberzug`.
- **Überzug++**: not a terminal protocol. The payload is a PNG file path (`<tmp>/pc-ueberzug-<pid>/<sha256>.png`, removed on exit by `RemoveUeberzugFiles`); `ueberzugDaemon` (`internal/app/ueberzug.go`) sends `add`/`remove` JSON lines to `ueberzugpp layer -so x11|wayland` (`UeberzugOutput`). One goroutine owns the process; render-path sends are non-blocking and drop when the buffer is full. No tty writes, no tmux handling (`ueberzugpp` handles pane offsets). `auto` picks it when Sixel would only be the heuristic fallback (`terminal_sixel` not `yes`) and `ueberzugpp` is on `PATH` (cached `ueberzugInstalled`) with a usable display.
- **Sixel**: DEC format, broad support (WezTerm, iTerm2, foot, ...).
- **Kitty graphics**: APC-based, richer. Cursor-relative "default placement" works on Kitty, Ghostty and WezTerm. Unicode placeholders (L4) work on Kitty and Ghostty always, on WezTerm only in placeholder-capable builds (L16).
- tmux never parses Kitty APC (passthrough is mandatory, L2). It parses Sixel only when sent bare (L9, L19).
- `auto` picks Kitty for Kitty/Ghostty/WezTerm and Sixel otherwise, both outside and under tmux (L13), steered by the tri-state `terminal_sixel` / `terminal_kitty` / `terminal_kitty_placeholder` confirmations in the M-F3 dialog.
- **Terminal identity under tmux**: tmux 3.2+ overwrites `TERM_PROGRAM` with `tmux` and `TERM` is tmux's own, so env sniffing fails. `auto` runs `tmux display-message -p '#{client_termtype}'` once (cached) and matches `kitty`/`ghostty`/`wezterm` (`tmuxOuterTerminalSupportsKittyGraphics`). Placeholder eligibility (`tmuxOuterTerminalSupportsUnicodePlaceholders` -> `TmuxSupportsKittyUnicodePlaceholders`) is narrower: kitty/ghostty always, anything else only with `terminal_kitty_placeholder = "yes"`.

## Lesson 1: Kitty re-transmit needs an explicit delete (WezTerm)

- **Symptom**: navigating between images flashed every intermediate image like a slideshow (WezTerm, no tmux).
- **Cause**: the spec requires deleting an image id's image and placements before re-transmitting under that id. The code skipped the delete for same-geometry replacements. Kitty tolerates this; WezTerm leaves each transmission resident and flushes them in turn.
- **Mitigation**: always delete before installing a different Kitty image, whatever the geometry (`reconcileImageBeforeShow`, `internal/app/image_overlay.go`). Outside tmux no cursor ceremony is needed for any terminal: CUP and payload are ordered writes to one process (L14).

## Lesson 2: tmux needs DCS passthrough for Kitty

- **Symptom**: flicker under tmux even after L1.
- **Cause**: tmux does not recognise Kitty APC (`\x1b_G...`); unwrapped bytes are treated as pane content (partly relayed, swallowed or replayed).
- **Mitigation**: wrap as `\x1bPtmux;<payload, every ESC doubled>\x1b\\` (`tmuxPassthroughWrap`). Requires `set -g allow-passthrough on` (off by default; otherwise tmux silently drops it). Wrapping several ST-terminated sequences in one envelope garbles output (tmux bug), so each chunk of a chunked Kitty transmission gets its own wrap (`splitTerminatedSequences`).

## Lesson 3: passthrough alone leaves a cursor-position race

- **Symptom**: images under tmux landing at 0,0 (seen on Ghostty).
- **Cause (initial reading)**: cursor-relative placement needs a CUP first; tmux forwards the CUP on its own redraw schedule while passthrough bytes reach the terminal immediately.
- **Current conclusion**: that diagnosis was incomplete. L8 gives the real mechanism and the working mitigation; L4 is the clean fix where the terminal supports it.

## Lesson 4: Unicode placeholders (Kitty, Ghostty; WezTerm per L16)

- Instead of "draw at the cursor", the client writes ordinary coloured text cells: U+10EEEE plus combining diacritics encoding row, column and the id's high byte, with the foreground colour carrying the rest of the id. The image is transmitted once with `U=1`. Cells go through tmux's normal redraw pipeline, so there is no position race.
- Almost nothing implements this (Kitty, Ghostty; WezTerm only in some builds). Hence it is used under tmux only when `client_termtype` says kitty/ghostty or the user confirmed support (L16).
- **Gotcha**: the ~280-entry diacritic table must be copied verbatim from kitty's `tools/utils/images/rowcolumn_diacritics.go`; any summarised copy silently miscomputes positions. The encoding was cross-checked against the worked example in kitty's docs (id `33554474` = `2*2^24 + 42`: colour byte 42, third diacritic index 2).

## Lesson 5: tmux's hardcoded ~1 MB single-sequence limit (<= 3.5a)

- **Symptom**: large/detailed Sixel previews flicker and vanish under tmux (yazi too).
- **Cause**: tmux through 3.5a silently discards any single escape sequence over ~1 MB (log: `input_dcs_dispatch: 1048575 bytes (discard)`; tmux/tmux discussion #4583). Made configurable as `input-buffer-size` in tmux 3.6 (check with `tmux show-options -s | grep input-buffer-size`).
- Only Sixel is exposed: Kitty is chunked to <= 4096 bytes per wrapped APC, Sixel is one unchunked DCS.
- **Mitigation** (`internal/preview/image.go`, gated on `Request.ImageInTmux`): reduce the palette (`config.DefaultPreviewTmuxSixelColors`; fewer colours compress better in run-length encoding), clamp the decode edge (`tmux_sixel_max_edge_px`), and fall back to the metadata result `"... / too large for tmux"` when the payload reaches `config.DefaultPreviewTmuxSixelMaxBytes`. A large image can still hit the cap; tmux >= 3.6 with a larger `input-buffer-size` removes the ceiling.

## Lesson 6: Sixel under tmux needs passthrough (superseded by L9, L19)

- The original design sent Sixel bare, assuming tmux renders it natively. After an upgrade nothing rendered even with `terminal-features ...:sixel` and `allow-passthrough on` confirmed, while yazi rendered both protocols. Reading yazi (`yazi-emulator/src/mux.rs`, `yazi-adapter/src/drivers/sixel.rs`) showed it wraps every graphics write under tmux, Kitty and Sixel alike. This project then passthrough-wrapped any protocol (`writeImagePayload`, `writeKittyDelete`).
- **Current conclusion**: bare Sixel is the right transport when tmux can render it (L9, L19). Passthrough is the fallback for tmux builds that cannot.
- **Still-valid second bug**: `ImageUnicodePlaceholder` was set whenever Kitty was in effect under tmux, even when forced on a terminal without placeholder support (WezTerm), sending placeholder cells it cannot interpret. `TmuxSupportsKittyUnicodePlaceholders` (`internal/preview/image_protocol.go`) now checks `client_termtype` independently of how Kitty was selected; when false, Kitty falls back to cursor-relative placement (with the L3/L8 race).

## Lesson 7: one decision point for placeholder mode

- **Symptom**: corrupted characters in the preview panel under tmux+WezTerm with `image_protocol=kitty`.
- **Cause**: split brain. The encoder was capability-gated (no `U=1`, cursor-relative) but `previewpanel.Draw` re-derived "Kitty and `$TMUX`" and still painted placeholder cells. Payload encoding and screen display mode are two halves of one handshake.
- **Mitigation**: decide once in `previewRequest`; the flag travels `Request.ImageUnicodePlaceholder` -> `Result` -> `previewpanel.State`, and `Draw` branches on that alone. The env-sniffing `inTmux()` helper is gone. Pinned by `TestDrawImagePlacementModeFollowsStateFlagNotEnvironment`. General rule: never evaluate the same predicate in two places.

## Lesson 8: tmux syncs the outer cursor only while the pane cursor is visible

- **Symptom**: tmux+WezTerm rendered nothing, and the screen scrolled as if text were printed (a Sixel drawn near the bottom scrolls like text): the image was decoded at a stale outer cursor position, then scrolled away.
- **Cause**: tmux does not position the real terminal's cursor while the pane cursor is hidden, and a full-screen tcell app hides it almost always. The pane-relative CUP updated only tmux's model; the passthrough payload rendered at the outer terminal's stale cursor. This explains L3's 0,0 too (a cursor never synced, not an unwinnable race).
- **Mitigation** (`emitImageAfterShow`, mirrors yazi's `Emulator::move_lock`): under tmux, save cursor; write CUP + show-cursor three times (each write unbuffered); sleep ~1 ms so tmux's loop moves the outer cursor; send the wrapped payload; hide cursor and restore.
- **Gotcha**: synchronized output (`\x1b[?2026h/l`) must not wrap this under tmux: it asks tmux to defer exactly the redraw being waited on. It stays outside tmux, where it prevents flicker.
- yazi avoids cursor-relative Kitty under tmux entirely (WezTerm via IIP/Sixel, Kitty/Ghostty via placeholders). L12 confirmed the race live with cursor-relative Kitty on WezTerm; L13 chose it anyway.

## Lesson 9: bare Sixel under tmux, gated on the attached terminal

- **Finding**: passthrough-wrapped Sixel is structurally unable to survive a tmux redraw. PTY captures of WezTerm showed the image appearing, then ~40 ms later a full redraw (`tty_invalidate()` after every passthrough write, `tty_cmd_rawstring` in tmux `tty.c`) painted spaces over it. tmux's `input_dcs_dispatch()` only recognises a bare `DCS q`; a `DCS tmux;` envelope is opaque pane content. Kitty passthrough survives because the outer terminal keeps Kitty placements in a layer cell overwrites cannot touch (a protocol property).
- **Mitigation**: send Sixel bare when tmux can render it for the *attached* terminal. Do not use tmux's DA1 reply (`ESC[?1;2;4c`): its `4` only says tmux was built `--enable-sixel`, with no reference to the client. On redraw `tty_cmd_sixelimage()` checks per-client `TERM_SIXEL` / `TTYC_SXL`, derived from the terminal-features database for the client termtype, and silently blanks when absent. The reliable signal is `#{client_termfeatures}` (`preview.TmuxSupportsNativeSixel`); WezTerm has `sixel` in tmux's built-in database. L19 adds the `#{image_support}` check.
- Bare Sixel needs none of L8's cursor ceremony: tmux parses it as ordinary pane content at its own pane-relative cursor, so `emitImageAfterShow` uses the no-tmux path (save cursor, CUP, payload, restore; no wrap). It also must not be wrapped in synchronized output under tmux (L17).

## Lesson 10: tmux `main` (post 2026-07-21) appears to fix L8/L9 behaviours, but keep the mitigations

- Re-verified with a throwaway PTY harness (`github.com/creack/pty`, an existing dependency) against a tmux `main` build (`next-3.8`, commit `31dccb6b`). Findings: resize still frees native Sixel storage (`screen_resize_cursor` calls `image_free_all`); passthrough-wrapped Sixel survived idle status ticks, window switches and resize; tmux forwards outer-cursor CUP even with the pane cursor hidden.
- **Decision**: do not remove the mitigations. Packaged tmux (3.4, 3.5a, ...) is the population L8/L9 were verified on, and removing them would restore the bugs there. Revisit once the fixes ship in a tagged release with real adoption (then a version-gated relaxation or raised baseline is reasonable).

## Lesson 11: cursor-relative Sixel can vanish under tmux with zero bytes sent (open)

- **Symptom**: a Sixel preview disappears after ~1 s idle.
- **Diagnosis**: instrumenting every direct tty write (`internal/app/image_overlay.go` is the only caller of `screen.Tty()`) showed one transmission per navigation, then silence while the image vanished. Not an app retransmission bug. yazi's resend policy (`same_lock` in `yazi-core/src/tab/preview.rs`) is equally conservative, and yazi reproduced the same vanish once it sat idle 10+ s in the same session (the earlier "yazi is fine" comparison used rapid browsing).
- **Cause**: a cursor-relative Sixel never writes the grid cells it covers, in the app or in tmux. Any tmux repaint of that region (status tick, unrelated activity) repaints from tmux's blank copy. WezTerm's fix for images surviving blank overwrites excludes Sixel (extending it to Sixel was tried twice and reverted; the second attempt broke large images). L19 gives the mechanism for tmux's per-line erase.
- **Decision**: no mitigation is implemented. A periodic idle re-transmit masked the symptom and was rejected: resize is the only acceptable case for re-sending an unchanged image (see preview.md invariant). The panel shows the red ` Sixel+Tmux ` label for the affected combination.

## Lesson 12: Kitty for WezTerm outside tmux only (superseded by L13)

- Because of L11, `auto` was changed to pick Kitty for WezTerm. Run live under tmux, cursor-relative Kitty on WezTerm intermittently rendered at 0,0 or vanished: the L3/L8 race, shared by cursor-relative Kitty and Sixel (both use `emitImageAfterShow` when the terminal lacks placeholders, which WezTerm did). This is why yazi avoids Kitty cursor-relative under tmux+WezTerm. For a time `auto` used Kitty outside tmux and Sixel under tmux. Current behavior is L13.

## Lesson 13: Kitty for WezTerm under tmux too (deliberate trade)

- `auto` picks Kitty for WezTerm both outside and under tmux (`tmuxOuterTerminalSupportsKittyGraphics`: kitty/ghostty/wezterm). `tmuxOuterTerminalIsKittyOrGhostty` stays narrow for placeholders, so WezTerm still gets cursor-relative Kitty under tmux unless L16 applies.
- This does **not** fix the race, and cannot from the app side. It accepts Kitty's intermittent 0,0 misplacement / vanish over Sixel's L11 vanish gap, because Kitty's 24-bit colour beats Sixel's reduced tmux-safe palette (L5). Both failure modes remain real.
- The ` Sixel+Tmux ` indicator still fires for any terminal that resolves to Sixel under tmux (non-Kitty-capable terminals, or an explicit `sixel` override). There is no equivalent indicator for the Kitty cursor race.

## Lesson 14: the race is tmux's architecture, not a WezTerm defect

- Outside tmux no race exists for any terminal (ordered writes to one process).
- Under tmux the mechanism is generic (L3/L8). Kitty/Ghostty never show it because the app never sends them cursor-relative Kitty under tmux: `TmuxSupportsKittyUnicodePlaceholders` is unconditionally true for them, so `previewRequest` always sets `ImageUnicodePlaceholder`, even with `image_protocol=kitty` forced. WezTerm is the only target with no placeholder route, hence the only place the race is visible.
- **Open question**: L10's finding (tmux forwards the cursor with the pane cursor hidden) was verified with a synthetic harness, not a real WezTerm client. Whether the residual misplacement is pure tmux relay timing or has a WezTerm-side component is unconfirmed; settling it needs capture against real WezTerm.

## Lesson 15: placeholder mode and the screen hash cache

- **Symptom** (real Kitty, tmux, `auto`): the current image goes blank after 10+ s idle or after navigating, with no consistent trigger. Toggling quick view always fixed it; moving to an adjacent file only sometimes. yazi and `kitten icat` never reproduced it, ruling out a terminal/tmux gap.
- **Cause**: the payload was transmitted, but `Show()` was skipped by `[ui].screen_render_hash_cache` (`ui.HashScreenLogical`, `internal/app/render.go`) because the placeholder branch of `reconcileImageBeforeShow` always returned `forceShow=false`. Placeholder cell bytes encode only row, column and the fixed `KittyGraphicsImageID`, never which image backs that id, so two images at the same grid size are identical screen content and neither tcell diffing nor the hash can see the change. The skipped `Show()` was the only signal that made the terminal re-associate the new data with the cells.
- **Why the symptoms fit**: tmux-only (outside tmux, cursor-relative placement already forces `Show()`); quick-view toggle fixes it (`resetImageOverlay` changes real cell content); adjacent navigation helps only when the new image changes the grid size.
- **Mitigation**: `reconcilePlaceholderImage` returns whether it transmitted or deleted anything and `reconcileImageBeforeShow` propagates it into `forceShow`. Pinned by `TestReconcileImageBeforeShowForcesShowOnPlaceholderPayloadChange` (`internal/app/image_overlay_test.go`; `tcell.SimulationScreen.Tty()` returns `(nil, false)`, so the test needs a small wrapper screen overriding `Tty()`). L4/L14's "no positioning race" still holds.

## Lesson 16: WezTerm placeholder support is per build, so it needs an opt-in

- Some WezTerm builds implement placeholder handling; most attached builds still do not, and `client_termtype` cannot tell them apart. An unconditional `wezterm` match (the first attempt) would reproduce L7's failure (cells the terminal cannot interpret render nothing) for most WezTerm+tmux users.
- **Mechanism**: `tmuxOuterTerminalSupportsUnicodePlaceholders` (`internal/preview/image_protocol.go`) matches kitty/ghostty unconditionally and anything else only when `[preview].terminal_kitty_placeholder == "yes"` (tri-state `auto`/`yes`/`no`, default `auto`), set through the M-F3 dialog ("Kitty placeholder supported", also Options -> Preview settings). A confirmed WezTerm then takes the race-free placeholder path (L14). Unconfirmed WezTerm stays cursor-relative. `preview.CapabilityUncertain` reports that case so the panel shows the ` M-F3 ` hint.
- **Revisit**: condition on a version floor once a released WezTerm identifies placeholder support; the `client_termtype` version suffix (`"WezTerm 20260716-..."`, `tmuxClientTermType`) is captured but not parsed.
- **Latent fragility**: `encodeKittyAPC` (`internal/preview/image.go`) sends the `U=1` virtual placement without explicit `c=`/`r=`, unlike the spec's example. It works because the terminal's fallback (image pixels / cell pixels) lands near `drawUnicodePlaceholderImage`'s own grid, two independent computations that happen to agree. Fix it with explicit `c=`/`r=` if a "grid looks subtly wrong" report appears (breakage would show first under tmux, where pixel dimensions relay through an extra layer).

## Lesson 17: native Sixel under tmux is tmux re-rendering the image

- **Symptom** (tmux from an experimental image branch, `tmux-pr-5445` / `next-3.8`, WezTerm attached): Sixel often missing on first display, fine on the second; yazi (Sixel-only) unaffected.
- **Measurement** (PTY harness driving the real `pc` in that tmux): `auto` native produced 19 Sixel DCS sequences (~400 KB) per preview, one strip per cell row, each re-declaring the palette, re-sent on every redraw of the region. Passthrough produced one DCS (~366 KB), our own encoder output. Byte volume is comparable; the structural difference is that native hands the terminal strips composited by tmux's own (experimental) image code.
- **Conclusion at the time**: yazi works better because it always passthroughs. **L19 reverses this.** A `[preview].tmux_sixel_transport` setting (`auto`/`native`/`passthrough`) was added for A/B and later removed (L19).
- **Kept change**: bare Sixel under tmux is no longer wrapped in synchronized output. tmux implements DECSET 2026 itself (`screen_write_start_sync`, `MODE_SYNC`), so wrapping defers the very redraw that paints the image. Outside tmux the wrapper stays. Same rule as L8.

## Lesson 18: the "first image of a session vanishes" symptom and tmux's one-time feature-update redraw

- **Symptom** (both Sixel transports, same experimental tmux): the first image shows then disappears (~80% of the time); later displays are fine.
- **Cause (harness reproduction)**: `tty_update_features()` (`tty.c`) ends with `image_tty_update(tty)`, `server_redraw_client(c)`, `tty_invalidate(tty)` and runs once when the outer terminal's primary/secondary/extended DA answers arrive (`tty-keys.c`), or when `tty_start_timer_callback` fires with none. The repaint comes from tmux's grid, which holds nothing where a passthrough image was drawn. Measured: a terminal that never answers probes gets wiped at the ~5 s timer (3/3); one that answers promptly updates at ~20 ms before any image (0/3). Not triggers (3/3 stayed up): short `tmux display-message -p` clients (`WarmTmuxCaches` runs two), `status-interval 1` ticks, `tmux refresh-client`.
- **Rejected: yazi's startup drain** (`Mux::tmux_drain`, a DSR round trip through the passthrough envelope before drawing). Implemented as a pre-tcell `/dev/tty` open + raw mode + wrapped `ESC[c` `ESC[5n` + read with a 150 ms deadline, it measured worse: `SetReadDeadline` is not honoured on that tty, so startup blocked until a keystroke (first paint ~60 ms -> ~3020 ms) and swallowed it. Deleted; do not re-propose without solving the deadline (raw `poll(2)`/`select(2)`, or the round trip through tcell's own tty after `Init`).
- **Caveat**: a tmux-side patch (make `tty_update_features()`'s `server_redraw_client()` conditional on the answer having added features or the image backend having changed; write-up `graphics-testing/tmux-issue.md`) fixed the harness reproduction but did nothing on the reporter's machine. The harness's fake terminal answers probes differently from a real WezTerm. `terminal-features` for the real termtype does not prevent the wipe either (it only changes ECH vs. line erase). The live mechanism is L19. To pin a trigger on a machine, record it with `graphics-testing/record-live.sh` (every byte tmux writes to the terminal plus tmux `-vv` log: `received primary DA`, `image backend is`, `tty_draw_line`, redraw decisions). The PTY harness lives in `graphics-testing/` with its own `AGENTS.md`; use it rather than re-reading tmux source.

## Lesson 19: passthrough Sixel cannot survive a line repaint, so native is the only correct transport

- **Method**: `graphics-testing/record-live.sh` put `script` between WezTerm and a fresh tmux server (`-vv`) to timestamp real terminal bytes beside tmux's log. The harness had been simulating the wrong half of the conversation.
- **Capture** (passthrough, quick view, image at row 3 col 124 of 243): first displays were repainted 112-222 ms later (flashed then gone, or never seen); warm re-displays 3.5 s later (stayed). tmux logged no redraw/invalidate/feature update, but thousands of `tty_draw_line` calls, appearing as erase-character runs starting mid-row in the left panel and sweeping 120 cells right, straight through the image.
- **Mechanism**: cells under a cursor-relative image are *locked* in tcell (so the app never paints them), hence tmux's grid holds blanks there. tmux redraws by line and emits a blank run as one erase, so any repaint of an overlapped row erases the image. The app repaints those rows constantly (prefetch loading icons and warm tints, title suffix, icons), and a cold image is followed by exactly that churn while neighbours decode. Nothing races; the image is stored where tmux's repaint cannot preserve it.
- **Proof** (`seqprobe -neighbour-row`, one unrelated write onto a row the image covers): bare/native re-emits the image DCS together with the text (`image_draw_line`); passthrough emits only the text and the image is gone.
- **Decision**: native is the only correct transport under a tmux with image support; passthrough is solely a fallback for tmux builds without it. L17's "yazi passthroughs, so passthrough is better" was the wrong inference: yazi is exposed to the same erase but repaints around its image far less than this app's prefetch marks do. L9's conclusion stands with its real explanation. Cost: tmux re-encodes one payload into one strip per cell row (~19 strips, ~400 KB) on every redraw of the region.
- **Setting removed**: with patched tmux + native confirmed on the reporter's machine, `[preview].tmux_sixel_transport` and its M-F3 radio group were deleted: a setting whose only correct value is the default is not a setting. The one legitimate fallback (a tmux built without image support reports terminal sixel capability but silently drops a bare DCS) is detected by asking tmux: `TmuxSupportsNativeSixel` also requires `#{image_support} != "0"`; an empty value (tmux too old to know the format) keeps the L9 `client_termfeatures`-only rule. Do not re-add the knob; extend detection.

## Recommended `~/.tmux.conf`

```tmux
# Required for Kitty under tmux always, and for Sixel when tmux does not report the attached
# terminal as sixel-capable (L2, L9). Off by default.
set -g allow-passthrough on

# tmux >= 3.6 only: raises the ~1MB hardcoded single-sequence limit (L5) so large Sixel
# previews are not silently discarded. Check `tmux -V` first: tmux errors on startup for
# options it does not know, so do not add this line on tmux < 3.6.
# set -g input-buffer-size 10485760
```

`terminal-features ...:sixel` matters for any terminal `auto` routes to Sixel (non-Kitty-capable terminals, or a Kitty-capable one with `image_protocol=sixel`): it makes `TmuxSupportsNativeSixel` true and Sixel go out bare (L9). If tmux's database lacks the terminal, add `set -as terminal-features ',your-terminal:sixel'`.

With `allow-passthrough` and `image_protocol = auto`:

| Terminal | Outside tmux | Inside tmux |
|---|---|---|
| Kitty | Kitty, cursor-relative | Kitty, Unicode placeholder |
| Ghostty | Kitty, cursor-relative | Kitty, Unicode placeholder |
| WezTerm | Kitty, cursor-relative (reliable, L1) | Kitty, cursor-relative by default (L3/L8/L12 race), or placeholder after confirming `terminal_kitty_placeholder = "yes"` in M-F3 (L16; confirming against a build that lacks support hits L7's failure) |
| Other sixel terminals tmux recognises | Sixel | Sixel, bare (native), subject to the L5 version caveat and the L11 gap |
| Sixel terminals tmux does not recognise | Sixel | Sixel, passthrough-wrapped fallback, same caveats |

Forcing `image_protocol=kitty` under tmux on a terminal `client_termtype` does not identify as Kitty-capable (not Kitty, Ghostty or WezTerm) falls back to cursor-relative Kitty: it renders, with the L3/L8 race.

## Debugging notes

- **A real PTY harness beats guessing.** `tcell.SimulationScreen` has no `Tty()`, so anything gated on graphics support is a no-op under it: fine for surrounding logic, useless for wire bytes. A throwaway `github.com/creack/pty` harness spawning `pc` with `TERM`/`TERM_PROGRAM` set and capturing raw output confirmed L1 empirically, including with a real 2.5 s panel-refresh ticker running, which ruled out a wrong theory that periodic refresh disturbed the selection. Prefer recording the real session (L18/L19) over simulating one side.
- **Trust protocol specs and reference source for encoding-shaped data.** Summarisers compress a 280-entry ordered codepoint table into prose. Use kitty's source (`gen/rowcolumn-diacritics.txt`, `tools/utils/images/rowcolumn_diacritics.go`) plus a worked example.
- **A symptom that looks like a race usually is one**: ask where the two clocks are rather than staring at one side (L3's 0,0 was tmux's redraw schedule vs. passthrough's bypass of it).
- **Never run server-mutating tmux commands (`new-session`, `kill-session`) from an agent shell without `$TMUX`.** Without it commands are not scoped to a client, and `kill-session -t _probe` killed the user's real session once. Use read-only `tmux show-options -s` / `-g` to inspect a live server.
- **A second independent implementation failing identically is evidence the bug is not yours** (yazi shared L5's flicker and L11's idle vanish), shifting the search to what is true of the environment.
- **A tool that works where yours does not is a lead to read, not a shrug.** yazi's source (`yazi-emulator/src/mux.rs`, `yazi-adapter/src/drivers/sixel.rs`) turned L6 into a minutes-long code comparison. Caveat from L17-L19: copy yazi's *mechanisms*, not its conclusions; the inference "it passthroughs, so passthrough is better" was wrong. yazi wraps only graphics and capability queries (DA1, XTVERSION, Kitty query), not ordinary TUI output.
- **Tests reading `os.Getenv("TMUX")` need `t.Setenv`.** A `writeKittyDelete` test failed only because the agent's own shell ran inside the user's tmux (`$TMUX` inherited). Pin ambient environment explicitly (`t.Setenv("TMUX", "")` and a non-empty value for both branches).
