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

Leave `image_protocol` at `"auto"` unless you have a reason to force a protocol.
Forcing `"kitty"` on a terminal that only speaks Sixel (or that lacks Kitty Unicode
placeholders under tmux) often produces missing or misplaced images.

## Supported terminals

| Terminal | Outside tmux | Inside tmux (with config below) |
|---|---|---|
| **Kitty** | Kitty protocol | Kitty with Unicode placeholders |
| **Ghostty** | Kitty protocol | Kitty with Unicode placeholders |
| **WezTerm** | Sixel | Sixel (passthrough) |
| Other Sixel-capable terminals (e.g. foot, many iTerm2 builds) | Sixel | Sixel (passthrough) |

With `image_protocol = "auto"`:

- Outside tmux: Kitty/Ghostty are detected from `TERM_PROGRAM` / `TERM`; everything
  else uses Sixel.
- Inside tmux: those env vars no longer describe the real terminal (`TERM` is
  typically `tmux-256color`, and tmux sets `TERM_PROGRAM` to `tmux`). Paras Commander
  asks tmux for `#{client_termtype}` instead and picks Kitty only when that looks like
  Kitty or Ghostty; otherwise Sixel.

## Required tmux configuration

tmux does **not** render Kitty or Sixel graphics itself for Paras Commander. Both
protocols are forwarded to the outer terminal with tmux **DCS passthrough**. Without
passthrough enabled, image escape sequences are dropped or mishandled and previews
will not work.

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

`terminal-features ...:sixel` is **not** required for Paras Commander. It only matters
for tools that use tmux’s native Sixel path; this app always uses passthrough.

## Protocol choice (`image_protocol`)

| Value | Behavior |
|---|---|
| `"auto"` | Best default. Kitty on Kitty/Ghostty; Sixel elsewhere (including WezTerm under tmux). |
| `"sixel"` | Always encode as Sixel. Needs a Sixel-capable outer terminal. |
| `"kitty"` | Always encode with the Kitty protocol. Under tmux, Unicode placeholders are used only when the outer terminal is Kitty or Ghostty; otherwise cursor-relative Kitty is used (can misplace images under tmux on terminals without placeholder support, e.g. WezTerm). |

Prefer `"auto"`. Explicit `"kitty"` under tmux+WezTerm is unsupported-quality territory;
`"auto"` selects Sixel there instead.

## Checklist

1. Terminal supports Sixel and/or Kitty graphics.
2. `[preview].images = true` (default).
3. Prefer `[preview].image_protocol = "auto"`.
4. If you use tmux: `allow-passthrough on`, plus the `update-environment` lines above.
5. For large/detailed photos under tmux: prefer tmux ≥ 3.6 and a higher `input-buffer-size`.

## Video thumbnails

Video files show text metadata first, then a thumbnail grid below it (default 2×2,
configurable via `video_thumb_cols` / `video_thumb_rows`) when graphics are enabled.
While frames are extracted, a `Generating thumbnails…` line appears under the metadata
so the text does not jump when the grid arrives.

| Condition | Thumbnails |
|---|---|
| `[preview].images = false` | Metadata only |
| No TTY | Metadata only |
| otherwise | Same protocol as still images (`ResolveImageProtocol`: Kitty on Kitty/Ghostty, sixel elsewhere including WezTerm) |

Audio files always show metadata text only (no cover art).

## Related

- Preview settings: [config.md](config.md) — `[preview]`
