# Chooser mode

Chooser mode lets an editor use `pc` as a file picker. Pressing Enter on a regular file writes its absolute path to a file and exits, so the editor can open it.

## Usage

```sh
pc --chooser-file=/path/to/output-file [--select=PATH] [--no-carousel]
```

- `--chooser-file` : when you press `Enter` on a regular file, `pc` writes its absolute path (with a trailing newline) to this file and exits immediately, without asking for confirmation. Directories open as usual; only files complete the pick.
- `--select` : open at a file or directory and highlight it (for example the active buffer of the editor). For a file, `pc` opens its parent directory with the file selected. If the path does not exist yet, the parent directory opens and the basename is selected if it is listed. `--select` requires `--chooser-file`.
- Carousel view (parent | current | child columns, see [Layout](layout.md#carousel-view)) is on by default for the left panel. `--no-carousel` switches to a plain single list; it requires `--chooser-file`.
- Quick View ([Quick view](quick-view-preview.md)) is on by default, so the other panel previews the highlighted file.
- Quitting without selecting leaves the output file untouched. Remove it before each invocation so you can tell "nothing chosen" from an earlier result.

Chooser mode takes no path arguments (use `--select`) and cannot be combined with `--carousel`.

## Helix example

Requires a Helix build with command extensions:

```toml
[keys.normal]
C-p = [
  ':sh rm -f /tmp/pc-chooser',
  ':insert-output pc -select "%{buffer_name}" -chooser-file=/tmp/pc-chooser',
  ':sh printf "\x1b[?1049h\x1b[?2004h" > /dev/tty',
  ':open %sh{cat /tmp/pc-chooser}',
  ':redraw',
]
```

The `printf` restores the terminal modes Helix expects after `pc` exits. The idea is borrowed from the yazi file manager.
