# Path completion

Every path input in paras-commander — the copy/move destination, the flatten
destination, path fields in dialogs like symlink/hardlink/rename/extract, and
the fuzzy history/bookmarks path picker's query — shares one filesystem
completion dropdown.

## How it works

Typing the final segment of a path (the part after the last `/`) looks up the
matching entries in that segment's parent directory and ranks them:

1. **Exact prefix matches** (case-sensitive) come first, sorted by name.
2. **Fuzzy matches** (case-insensitive, not already a prefix match) follow,
   best score first, then by name.

A bare trailing `/` (an empty final segment) lists the whole directory,
sorted by name, without opening the dropdown automatically — the dropdown
only auto-opens while you are actively typing a segment.

The copy/move destination completes **directories only** when the source is a
directory or a multi-selection, since those can't land on a file. The
flatten destination always completes directories only. Copying or moving a
single file still offers files, so you can overwrite one by name.

### Single candidate: ghost text instead of a dropdown

When there is exactly **one** candidate and its name has what you typed as a
prefix, no dropdown appears at all. Instead the remaining letters of that
name are shown dimmed, inline, right after the caret ("ghost text") — the
input still shows exactly what you typed, plus that dimmed suggestion tail.
**Tab** accepts it (appending a trailing `/` for a directory), exactly as it
would accept a dropdown row. Up/Down/Enter/Esc are not affected by the ghost
text and pass straight through to the dialog — Enter confirms the dialog as
normal.

A single **fuzzy** (non-prefix) match cannot be shown as a suffix after the
caret, so that case still shows the one-row dropdown described below.

With two or more candidates, suggestions appear only in the dropdown, never
inline in the field.

## Keys

While the input has candidates:

- **Tab** — with exactly one candidate (dropdown or ghost text), accepts it
  immediately. With more than one, opens the dropdown (if closed) or cycles
  to the next row (wrapping around) if it is already open. Accepting a
  directory leaves the caret after its `/` with nothing suggested yet: type
  a letter to get suggestions, or press Tab again to list the directory.
- **Up / Down** — while the dropdown is open, moves the selection (no wrap).
- **Enter** — while the dropdown is open, accepts the selected candidate and
  closes the dropdown; the *next* Enter confirms the dialog as usual. While
  only ghost text is showing (no dropdown), Enter is not intercepted at all
  and confirms the dialog immediately.
- **Esc** — while the dropdown is open, closes the dropdown only; the dialog
  itself stays open. Ghost text is not closable with Esc (there is no
  dropdown to close); Esc falls through to the dialog.

Accepting a directory appends a trailing `/`.

## Appearance

The dropdown shows up to 5 rows below the input (or above it, if it would run
off the bottom of the screen), with the matched letters highlighted and the
selected row shown with a highlighted background across its full width. When
there are more than 5 candidates, a vertical scrollbar appears in the
dropdown's own rightmost column (the panel's configured scrollbar style).
Colors come from the `[dialog.completion]` theme section (`item`,
`item.selected`, `match`, `match.selected`) in the active `themes/*.toml`
file.
