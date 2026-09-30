# Find duplicates

Find duplicates (`Ctrl+Alt+F`, Command menu, leader `P`) scans the active panel's directory tree for files with identical content and opens a full-screen results view. It is local-only; remote paths are rejected.

## Scan phases

1. **Walking** collects every regular file (symlinks are skipped).
2. **Size prefilter.** A file whose size is unique in the tree cannot have a duplicate and is never opened.
3. **Confirm** (only when the candidates exceed `dedup.hash_confirm_bytes`) asks before hashing that many bytes.
4. **Hashing** compares same-size files chunk by chunk and drops a file as soon as its prefix differs from all others; files that reach the end are grouped by SHA-256.

Esc cancels the scan.

## Results view

Rows that are copies of the file under the cursor (and collapsed folders containing such copies) are drawn in the hint color and carry a trailing related-copy icon (theme key `icons.dedup.related`). The icon stays visible on kept (green) and marked (yellow) rows, where the row color shows keep/mark state instead of the hint.

In **View: Dirs** the right column is split: the Copies pane on top and a browse panel below it, a real file list. It shows the directory of the row under the cursor (the directory itself for a folder row, or the file's parent with the cursor on the file), following whichever tree pane's cursor moved last (switching panes with Tab does not reload it). Tab cycles focus main tree, Copies, browse panel, back to main. In the browse panel you can move the cursor, open a folder (Enter/Right), go to the parent (Backspace/Left), view a file (F3, returning to the results) or edit it (F4); selection and file operations are not available there. Moving the cursor in either tree pane (not merely focusing it) re-syncs the panel to the new row, discarding manual navigation. Leaving the results view and coming back keeps the browse panel's focus and location. **View: Groups** has no browse panel.

## Leaving and returning

Leaving the results view (Esc, Alt+W again, Enter to jump to a file, "Back to file view", or switching to Jobs/Commands/Messages) keeps the results in memory and shows `Duplicates kept - Alt+W returns`. Kept results live until the next scan or until pc quits. There are two ways back:

- **Alt+W** (`dedup.open`, Display menu > Duplicates, leader `w`) shows the kept results from the browser or from the Jobs/Commands/Messages views. Without kept results it says so.
- **Running Find duplicates again.** When the active panel is at or under the scanned root, the view returns instantly with no rescan. From anywhere else a dialog offers **Show** (kept results), **Rescan** (scan the active panel's directory, replacing the kept results) or **Cancel**.

**Refresh** inside the view rescans the kept root.

## Hash cache

Full-file SHA-256 digests are cached in `<user cache dir>/pc/dedup-hashes.gob` (`~/.cache/pc/dedup-hashes.gob` on Linux, honoring `XDG_CACHE_HOME`). An entry is reused only while the file's path, size and modification time are unchanged, so later scans - including after a restart, a cancelled run or Refresh - skip unchanged files. A fully cached rescan skips the confirm prompt. A cancelled scan keeps the hashes it completed.

Only files hashed to the end (real duplicates) are cached; files ruled out by a differing prefix are cheap to reread. To clear the cache, delete the file.
