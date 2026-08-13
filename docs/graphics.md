# Terminal image previews

Paras Commander can show inline image previews in quick view, fullscreen (F3), and
carousel. Images are drawn with either **Sixel** or the **Kitty graphics protocol**,
depending on your terminal and configuration.

This page covers what you need for previews to work reliably — especially under
**tmux**.

## Enable images

In `config.toml`:

```toml
[preview]
images = true
image_protocol = "auto"   # recommended: "auto", "sixel", or "kitty"
```

| Key | Default | Meaning |
|---|---|---|
| `images` | `true` | When `false`, image files show format / dimensions / size text only. |
| `image_protocol` | `"auto"` | Which graphics protocol to use (see below). |
| `unicode_placeholder_terminals` | `[]` | Extra terminals (beyond the built-in Kitty/Ghostty) to trust with Kitty Unicode placeholders under tmux — see [Enabling Unicode placeholders for WezTerm](#enabling-unicode-placeholders-for-wezterm-under-tmux) below. |

Leave `image_protocol` at `"auto"` unless you have a reason to force a protocol.
Forcing `"kitty"` on a terminal that only speaks Sixel (or that lacks Kitty Unicode
placeholders under tmux) often produces missing or misplaced images.

## Supported terminals

| Terminal | Outside tmux | Inside tmux (with config below) |
|---|---|---|
| **Kitty** | Kitty protocol, cursor-relative | Kitty protocol, Unicode placeholders (always trusted) |
| **Ghostty** | Kitty protocol, cursor-relative | Kitty protocol, Unicode placeholders (always trusted) |
| **WezTerm** | Kitty protocol, cursor-relative | Kitty protocol, cursor-relative by default; Unicode placeholders only if you opt in via `unicode_placeholder_terminals` (see below) |
| Other Sixel-capable terminals (e.g. foot, many iTerm2 builds) | Sixel | Sixel, native if tmux recognizes the terminal as sixel-capable, else passthrough |

With `image_protocol = "auto"`:

- Outside tmux: Kitty/Ghostty/WezTerm are detected from `TERM_PROGRAM` / `TERM`;
  everything else uses Sixel.
- Inside tmux: those env vars no longer describe the real terminal (`TERM` is
  typically `tmux-256color`, and tmux sets `TERM_PROGRAM` to `tmux`). Paras Commander
  asks tmux for `#{client_termtype}` instead and picks Kitty when that looks like
  Kitty, Ghostty, or WezTerm; otherwise Sixel.
- Kitty protocol under tmux still needs a *separate* check to decide **how** to send
  it: Unicode placeholders (race-free) for a confirmed placeholder-capable terminal, or
  cursor-relative-via-passthrough otherwise. Kitty and Ghostty are always trusted for
  this; WezTerm is not, by default (see next section).

## Required tmux configuration

tmux does not understand the Kitty graphics protocol at all — Kitty images are always
forwarded to the outer terminal with tmux **DCS passthrough**. Without passthrough
enabled, Kitty escape sequences are dropped or mishandled and Kitty previews will not
work.

Sixel is different: when tmux reports (via `#{client_termfeatures}`) that the
currently attached outer terminal supports sixel — true for WezTerm out of the box,
since tmux ships WezTerm in its built-in terminal-features database, so this applies
whenever `image_protocol = "sixel"` is forced or the attached terminal isn't
Kitty-capable at all — Paras Commander sends Sixel **unwrapped**. tmux parses it
natively, keeps track of the image, and
redraws it itself after every tmux-side screen invalidate (status line tick, window
switch, etc.), which is more robust than passthrough: a passthrough-wrapped image is
invisible to tmux, so tmux's own next full-screen redraw paints over it and it never
comes back. When tmux doesn't report sixel support for the attached terminal, Sixel
falls back to passthrough like Kitty.

Add to `~/.tmux.conf`:

```tmux
# Forward Kitty/Sixel graphics to the outer terminal (off by default).
set -g allow-passthrough on

# Keep client TERM / TERM_PROGRAM available in the session environment.
set -ga update-environment TERM
set -ga update-environment TERM_PROGRAM
```

Reload the config (or restart the tmux server) after editing:

```sh
tmux source-file ~/.tmux.conf
```

Confirm passthrough is on for the running server:

```sh
tmux show-options -g | grep allow-passthrough
```

You should see `allow-passthrough on`.

### Enabling Unicode placeholders for WezTerm under tmux

Under tmux, Kitty-protocol images normally use **cursor-relative placement**: Paras
Commander moves the cursor, then writes the image right after. tmux only forwards the
outer terminal's real cursor position on its own redraw cycle, so this has a known,
unfixed race — the image can occasionally land at `0,0` or vanish.

Kitty and Ghostty avoid this entirely: they support an alternative **Unicode
placeholder** mode (ordinary colored text cells that reference the image), which goes
through tmux's normal redraw pipeline instead and has no position race. Paras Commander
always uses it for Kitty/Ghostty.

WezTerm can support Unicode placeholders too, but only in specific builds — it's not a
guarantee for "WezTerm" as a name, and `#{client_termtype}` can't tell a
placeholder-capable build from an older/stock one that will only render garbage
diacritic glyphs if placeholder mode is forced on it. So Paras Commander does **not**
assume WezTerm supports placeholders by default; you opt in explicitly once you've
verified your attached build actually does:

1. Confirm support in your WezTerm build (check your WezTerm release notes/changelog
   for Kitty Unicode-placeholder / `U=1` virtual placement support, or test it with
   `kitten icat` directly against WezTerm under tmux).
2. Add `wezterm` to `unicode_placeholder_terminals` in `config.toml`:

   ```toml
   [preview]
   image_protocol = "auto"
   unicode_placeholder_terminals = ["wezterm"]
   ```

3. Reattach (or open a new pane) so tmux's `#{client_termtype}` is re-queried, then open
   an image preview under tmux and confirm it renders and repositions cleanly (resize
   the pane, switch windows and back).

If images render as scrambled colored text instead of a picture, your attached WezTerm
build doesn't actually support placeholders — remove `wezterm` from the list again.

### Optional: large Sixel previews (tmux 3.6+)

tmux versions through **3.5a** have a hardcoded ~1 MB limit on a single escape
sequence. Oversized Sixel payloads are **silently discarded** (images may flash and
disappear). Kitty payloads are chunked and are not affected by this limit.

**tmux 3.6+** exposes `input-buffer-size`. If `tmux -V` reports 3.6 or newer, you can
raise it:

```tmux
# tmux >= 3.6 only — older tmux will error on this option at startup.
set -g input-buffer-size 10485760
```

Check whether your server supports it (read-only):

```sh
tmux show-options -s | grep input-buffer-size
```

On tmux older than 3.6, Paras Commander shrinks Sixel under tmux (smaller palette and
a size cap) and falls back to a text summary (`… / too large for tmux`) when a preview
would still exceed the safe limit. Upgrading tmux and raising `input-buffer-size`
removes that ceiling.

`terminal-features ...:sixel` (and tmux's built-in per-terminal database, which already
covers WezTerm) is what Paras Commander relies on to send Sixel unwrapped under tmux —
see above. If your terminal isn't recognized, add it explicitly, e.g.
`set -as terminal-features ',your-terminal:sixel'`.

## Protocol choice (`image_protocol`)

| Value | Behavior |
|---|---|
| `"auto"` | Best default. Kitty on Kitty/Ghostty/WezTerm; Sixel elsewhere. Under tmux, Unicode placeholders are used for a confirmed placeholder-capable terminal (Kitty/Ghostty always, WezTerm only via `unicode_placeholder_terminals`); otherwise cursor-relative Kitty. |
| `"sixel"` | Always encode as Sixel. Needs a Sixel-capable outer terminal. |
| `"kitty"` | Always encode with the Kitty protocol. Under tmux, Unicode placeholders are used only when the outer terminal is confirmed placeholder-capable (same rule as `"auto"`); otherwise cursor-relative Kitty is used (can misplace images under tmux on terminals without placeholder support, e.g. an un-opted-in WezTerm). |

Prefer `"auto"`. It already picks Kitty for WezTerm both outside and under tmux;
whether that's cursor-relative or Unicode-placeholder under tmux is controlled by
`unicode_placeholder_terminals` (see [Enabling Unicode placeholders for
WezTerm](#enabling-unicode-placeholders-for-wezterm-under-tmux) above).

## Checklist

1. Terminal supports Sixel and/or Kitty graphics.
2. `[preview].images = true` (default).
3. Prefer `[preview].image_protocol = "auto"`.
4. If you use tmux: `allow-passthrough on`, plus the `update-environment` lines above.
5. For large/detailed photos under tmux: prefer tmux ≥ 3.6 and a higher `input-buffer-size`.
6. On WezTerm under tmux, add `wezterm` to `unicode_placeholder_terminals` only after
   confirming your build supports Kitty Unicode placeholders — otherwise leave it unset.

## Video thumbnails

Video files show text metadata first, then a thumbnail grid below it (default 2×2,
configurable via `video_thumb_cols` / `video_thumb_rows`) when graphics are enabled.
While frames are extracted, a `Generating thumbnails…` line appears under the metadata
so the text does not jump when the grid arrives. Grids are downscaled to
`image_max_edge_px` (default 1024) before the final cell-budget fit. Composed thumbnails
are stored under `$XDG_CACHE_HOME/pc/video-thumbs/` (cap `video_thumb_cache_max_mb`,
default 512). With `prefetch` enabled (default), nearby videos are warmed in the
background before the caret lands on them.

| Condition | Thumbnails |
|---|---|
| `[preview].images = false` | Metadata only |
| No TTY | Metadata only |
| otherwise | Same protocol as still images (`ResolveImageProtocol`: Kitty on Kitty/Ghostty/WezTerm, sixel elsewhere) |

Audio files always show metadata text only (no cover art).

## Related

- Preview settings: [config.md](config.md) — `[preview]`
