# Panels

Each panel lists one directory. This page covers what a listing shows, how to change its layout, sort and hidden-file behaviour, and the git integration. Selecting files is in [Selections](selections.md), computed columns in [Meta columns](meta-columns.md), sizes in [Disk usage](disk-usage.md), and moving between directories in [Navigation](navigation.md).

![Panel with git status column](assets/panels.png)

Directories load in the background. A slow directory (network mount, SFTP) keeps showing the old listing and the target row gets a working icon until the new one arrives.

## Listing formats and columns

Press `M-t` to cycle the active panel through the three layouts, or use Left/Right menu, Listing format.

| Format | Columns |
|---|---|
| `brief` | name, meta columns, size |
| `mtime` | name, meta columns, size, modified time |
| `perm` | as `mtime` plus permissions |

`default_listing_format` in [config.md](config.md#panels) picks the startup format. A narrow panel can show only the name column (`[ui].shrunken_shows_name_only`, see [config.md](config.md#ui)). `M-z` zooms the active panel for the session (unless its file name column is already wide, see [`[ui.zoom]`](config.md#uizoom)), and `S-Tab` hides the inactive one. See [Layout](layout.md).

### Long names

A name too long for its column is shortened word by word (words are split at `.`, `-`, `_` and spaces) rather than cut in the middle:

1. The longest words are abbreviated first, down to five letters (`Lantern` → `Lant` happens only later; a word loses at least three letters at once so it never looks like a typo).
2. Words in the middle are then dropped as one gap shown as `…`, growing outward from the centre. The first and last words stay.
3. The remaining words are abbreviated further, down to four letters, then the last word is dropped and the first word shortened.

A short file extension (up to five letters, such as `.odt`) is always kept, and words containing digits (`2019`, `v3`, `0042`) are never abbreviated, only dropped whole. Each step only removes text, so dragging a panel narrower never brings back a part that a wider panel had hidden.

The letter where a word was abbreviated, and the `…`, are drawn in the row colour's counterpart shade (blue ↔ bright blue, white ↔ bright white; bold blue, which most terminals already draw bright, marks with plain blue; other colours are lightened or darkened), so the cut is visible without spending a character:

```
Garden.Lantern.Whisper.2019.Pebble.Draft.odt
Garden.Lantern.Whisper…Pebble.Draft.odt
Garden.Lantern…Draft.odt
Garden…Draft.odt
Garden….odt
```

## Sorting

`M-s` (leader `s`, Left/Right menu, Sort...) opens the Sort dialog for the active panel: sort key (Name, Extension, Size, Modified, and any active [meta column](meta-columns.md)), reverse, directories first, and optional size ordering while a [disk usage](disk-usage.md) scan runs. The header arrow shows the sorted column and points toward larger values. `default_sort`, `sort_reverse` and `directories_first` set the startup behaviour. Cycling the sort key and reversing the sort have no default key and can be bound in `keybindings.toml`.

## Hidden and ignored files

`M-.` (leader `.`, Left/Right menu, Toggle hidden) shows or hides dotfiles together with files ignored by git. The setting is global for both panels. With `respect_gitignore` on (default) and hidden files off, entries matched by `.gitignore` and `.git/info/exclude` are hidden inside a git work tree; outside one only dotfiles are affected. Copy, move, delete and scans always see the real filesystem. A "Gitignore" indicator on the panel's bottom border shows when ignore filtering is active.

## Tree view

Left/Right menu, Tree view (unbound by default) turns the panel into an expandable tree; a ` Tree ` chip appears on the border. Directories load lazily as they expand. Tree view and the carousel view (`M-v`) are mutually exclusive.

| Key | Action |
|---|---|
| `M-→` / `M-←` | Expand / collapse. Pressing expand again on an open directory opens one more level below it. Collapse on a file or closed row jumps to the parent. |
| `C-M-→` / `M-S-→` | Expand all, one level per press / fully |
| `C-M-←` / `M-S-←` | Collapse all, one level per press / fully |
| `M-↑` / `M-↓` | Previous / next sibling directory |

Expand-all is capped (a depth limit for the shallow variant and a total row limit) and shows a `Depth: N/5` toast. The expanded set and cursor are remembered per directory for the session. Enter and Right still change into a directory.

When you return to a directory, the folders that were expanded there are fetched together with its listing so the tree appears at once. On a slow filesystem that wait is capped by `[panels].tree_restore_timeout_ms` (default 1500 ms, counted from the start of the load, so the directory's own listing time counts): folders that were not fetched in time stay collapsed, along with any remembered folders inside them, and a `Slow filesystem, reverted to collapsed state` message appears. The collapsed state is what the directory remembers from then on. Refreshing the directory you are in never collapses its tree.

## Git

Inside a local git work tree a two-cell status column appears before the name (not on SFTP panels or in name-only layout). Letters follow `eza`: staged in the first cell, unstaged in the second.

| Letter | Meaning |
|---|---|
| `N` | new / untracked |
| `M` | modified |
| `D` | deleted |
| `R` | renamed |
| `T` | type change |
| `I` | ignored |
| `U` | conflicted |
| `-` | unchanged (also shown while the status loads) |

Directories show a summary of their contents. Status is read in the background with `git status`.

- `M-+` and `M--` stage or unstage the selected entries, or the cursor entry (then moving the cursor to the next entry). Untrack (no default key) runs `git rm -r --cached` the same way: the entries leave the index but stay on disk.
- `M-g` opens the git menu. Its Filter group limits the panel to Staged, Unstaged, Tracked or Untracked entries. Its Actions group runs **Stage** (`S`), **Unstage** (`U`) and **Untrack** (`R`), the uppercase twins of the matching filter letters; they are the same actions as above, and with chord hints on (F3) the rows show their shortcuts. The filter is dropped when you leave the repository. Its **Commit** row runs `git commit` in your editor (the TUI is suspended meanwhile) when something is staged. **Push** (`P`) and **Pull** (`p`), matching lazygit, run `git push` / `git pull` in the background; credentials must come from an SSH agent or credential helper, since there is no terminal to prompt in.
- Viewing (`F3`) or quick-viewing a modified file shows its diff; see [Quick view and preview](quick-view-preview.md). `C-M-j` / `C-M-k` jump between changes.

## Row indicators

Symbols at the end of a name:

- A new-file mark on entries created by the last copy, move or flatten into this directory; the previous batch is shown in a second color and all earlier ones are dimmed. The marks clear when you leave the directory or refresh the panel.
- An open-folder symbol on a directory the other panel currently shows.
- A name-match mark on rows whose name also exists in the other panel's directory (`other_panel_match_marks` in [config.md](config.md#ui)); the name under the cursor is highlighted in both panels.
- Job, queued, pinned and no-permission marks; see [Jobs](jobs.md).

After a copy or move into the other panel's directory, that panel's cursor jumps to the first transferred entry unless you already moved it (`focus_other_panel_after_transfer`, [config.md](config.md#operations)).

## Opening files

- `Enter` / `Right` enters a directory. On a local executable (`+x` with a shebang or ELF header) it runs the file when `run_executables_on_enter` is on; the output appears in the Commands view, or in the background for rules in `panels.execute_rules` (AppImages by default). Other files open with the desktop default application when `open_files_externally` is on; when `xdg-open` is not installed they open in the full-screen preview.
- `F4` (also `M-e`) edits the file in `$EDITOR`.
- `F3` opens the file preview.

## Staying current

Panels re-read their directory every `refresh_interval_ms` (default 2500, `0` disables). Changes are applied without losing the cursor, selection or scroll position. A slow directory is polled less often. `C-y` forces a full reload and also recalculates [meta columns](meta-columns.md).

### Directory watching

On Linux, local panels also re-read as soon as the system reports a change in the directory they show (and in expanded [tree view](#tree-view) directories), so files created, deleted, renamed or changed by other programs appear within a fraction of a second instead of after the next refresh tick. This includes panels browsing with [fileindex](fileindex.md#browsing-with-fileindex), where a file leaving the disk turns into a ghost row and back. The periodic refresh keeps running regardless, and `[panels] watch_directories = false` turns watching off.

Limits: changes made on another machine to an NFS or SMB mount are not seen by the system, so those directories rely on the periodic refresh; SFTP panels, panels browsing the index directly (`fileindex://`) and every non-Linux platform use the periodic refresh alone. Very large trees are watched only in part (the panel directory first); the periodic refresh covers the rest.

Each panel remembers the focused entry per directory, so going back (see [Navigation](navigation.md)) lands on the item you left.

`M-y` latches the active panel as driver: moving the cursor onto a directory shows it in the other panel. `M-o` / `M-i` open the cursor directory or the current path in the other panel; `C-M-o` opens the cursor directory there and moves focus to that panel.
