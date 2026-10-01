# Find duplicates

Find duplicates (`Ctrl+Alt+F`, Command menu, leader `P`) scans the active panel's directory tree for files with identical content and opens a full-screen results view. It is local-only; remote paths are rejected.

When the active panel has selected directories, a dialog first asks whether to search the current directory (**Directory**) or only the selected directories (**Selected**, the default). Selected-only scans walk just those directory trees (files selected alongside them are ignored), rooted at their closest common parent, and Refresh rescans the same selection.

## Scan phases

1. **Walking** collects every regular file (symlinks are skipped).
2. **Size prefilter.** A file whose size is unique in the tree cannot have a duplicate and is never opened.
3. **Confirm** (only when the candidates exceed `dedup.hash_confirm_bytes`) asks before hashing that many bytes.
4. **Hashing** compares same-size files chunk by chunk and drops a file as soon as its prefix differs from all others; files that reach the end are grouped by SHA-256.

Esc cancels the scan.

## Results view

Rows that are copies of the file under the cursor (and collapsed folders containing such copies) are drawn in the hint color and carry a trailing related-copy icon (theme key `icons.dedup.related`). The icon stays visible on kept (green) and marked (yellow) rows, where the row color shows keep/mark state instead of the hint.

In **View: Dirs** the right column is split: the Copies pane on top and a browse panel below it, a real file list. It shows the directory of the row under the cursor (the directory itself for a folder row, or the file's parent with the cursor on the file), following whichever tree pane's cursor moved last (switching panes with Tab does not reload it). Tab cycles focus main tree, Copies, browse panel, back to main. Shift+Tab (`dedup.prev-pane`) cycles the other way. In the browse panel you can move the cursor, open a folder (Enter/Right), go to the parent (Backspace/Left), view a file (F3, returning to the results) or edit it (F4); selection and file operations are not available there. Moving the cursor in either tree pane (not merely focusing it) re-syncs the panel to the new row, discarding manual navigation; a single press updates them at once, but while a navigation key is held (key repeat) the reload and the Copies pane wait until it is released and the related-row hints are hidden (`key_repeat_debounce_ms`), like panel sync in the main file list. Leaving the results view and coming back keeps the browse panel's focus and location. **View: Groups** has no browse panel.

Shift+Left / Shift+Right (`ui.open-primary` / `ui.open-secondary`, the same actions the Find and Pin dialogs use) point the primary or secondary file panel at the row under the cursor: a folder opens that folder, a file opens its parent with the file selected. This works in both tree panes and in the browse panel, and the results view stays open.

Space or Ctrl+K (`dedup.mark-keep`, rebindable under `[dedup]` in `keybindings.toml`) marks the file under the cursor to keep; Insert on a kept file shows a toast instead of selecting it.

Right / Enter on a **file** row (tree panes and the browse panel) opens it with the default opener, like the main file list (honours `open_files_externally`); folder and group rows keep their expand/enter behaviour. Alt+x (`panel.external-browser`, the main list's binding, rebindable in `keybindings.toml`) opens the folder of the row in the desktop file manager: a file's containing folder, a folder row itself, or the browse panel's current folder.

Alt+Ctrl+Left / Alt+Ctrl+Right (`panel.tree-collapse-all` / `panel.tree-expand-all-shallow`, the same keys as the main file list) collapse or expand the focused tree pane by one level per press. Alt+Shift+Left / Alt+Shift+Right (`dedup.collapse-all` / `dedup.expand-all`, matching the main list's full collapse/expand) collapse or expand every folder in the pane. Expanding (both forms) is capped like the main file list: at most 5 levels deep (folders at the cutoff stay collapsed, with an info toast), and fewer levels if the result would exceed 20,000 rows.

**Type to jump:** typing a printable key that has no binding in the focused tree pane (not the browse panel) starts the same fuzzy quick filter as the main file list. `> query` shows in the pane's top border, the cursor jumps to the best match (the Copies pane follows), and matched characters are highlighted. Up/Down cycle through matches (`filter.cycle_matches`, `filter.case_insensitive`), Backspace edits the query, Ctrl+L / Ctrl+Backspace clear it, Esc closes it. Enter and Insert close the filter and then do their normal action; any other key closes it too. Each pane has its own filter, and switching panes clears it.

## Leaving and returning

Leaving the results view (Esc, Alt+W again, Enter to jump to a file, "Back to file view", or switching to Jobs/Commands/Messages) keeps the results in memory and shows `Duplicates kept - Alt+W returns`. Kept results live until the next scan or until pc quits. There are two ways back:

- **Alt+W** (`dedup.open`, Display menu > Duplicates, leader `w`) shows the kept results from the browser or from the Jobs/Commands/Messages views. Without kept results it says so.
- **Running Find duplicates again.** When the active panel is exactly the scanned root, a dialog offers **Show** (kept results), **Rescan** (scan again, replacing the kept results) or **Cancel**. In any other directory a fresh scan of that directory starts directly, replacing the kept results.

Whenever kept results are shown again (Alt+W or **Show**), the view opens immediately while the files are re-checked in the background (the title shows `N%`); files that vanished meanwhile are then dropped (with their marks), groups left with fewer than two members disappear, and if nothing remains the view closes with `Duplicates: all files gone`.

**Refresh** inside the view rescans the kept root.

## Hash cache

Full-file SHA-256 digests are cached in `<user cache dir>/pc/dedup-hashes.gob` (`~/.cache/pc/dedup-hashes.gob` on Linux, honoring `XDG_CACHE_HOME`). An entry is reused only while the file's path, size and modification time are unchanged, so later scans - including after a restart, a cancelled run or Refresh - skip unchanged files. A fully cached rescan skips the confirm prompt. A cancelled scan keeps the hashes it completed.

Only files hashed to the end (real duplicates) are cached; files ruled out by a differing prefix are cheap to reread. To clear the cache, delete the file.
