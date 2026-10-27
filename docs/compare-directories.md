# Compare directories

Compare shows how two directory trees differ: files that exist on only one side, identical files, files that moved to a different path, and files with different content. Files at the same path are compared byte for byte and the comparison stops at the first difference; files without a partner are hashed with SHA-256, so renamed or relocated copies are recognised. Local directories only.

![Compare results with every class](assets/compare.png)

## Open

`Ctrl+Alt+C` (`panel.compare-panels`, leader `C`, Command menu, Compare panels) compares the left panel's directory with the right panel's. The directories must not contain one another. It is also reachable from [Find duplicates](find-duplicates.md) (`Ctrl+Alt+C`), where it compares the selected file's directory with the directory under the Copies cursor and returns to the duplicates view on `Esc`. Keys are rebindable in `keybindings.toml` (`[compare]` for the keys below).

## Results

Two columns, one per side, list the path of each file (only the differing part when both roots share a prefix). A status icon and the bottom-border legend say what the row is:

| Class | Meaning |
|---|---|
| Identical | same path and content |
| Relocated | same content, different path |
| Only on one side | exists in one tree only |
| Content differs | same path, different content |

Files are compared in parallel (`hash_concurrency`); rows show Pending or Hashing until settled. Same-path files of different size are marked as different without being read. Same-path files of equal size are read side by side, `read_buffer_kib` at a time, and stop at the first differing block, the same check the [conflict rules](jobs.md#verify-both-to-see-if-identical) use. The Hashed count in the border counts both files of a pair. Zero-byte files are hidden by default (`N empty hidden` in the border).

## Keys

| Key | Action |
|---|---|
| `Left` / `Right`, `Tab` | choose the focused column |
| `Up` / `Down`, `PgUp` / `PgDn` | move through rows |
| `Enter` | go to the file in the browser and close |
| `F3` | `compare.cycle-filter`: Category dialog, show any mix of the classes |
| `Shift+F3` | `compare.reset-filter` |
| `Alt+F` | `compare.filter-dialog`: filter by name, see [Panel filter](panel-filter.md#compare-view) |
| `Alt+E` | `compare.toggle-empty` |
| `Alt+R` | `compare.refresh`: rescan, keeping cursor and filters |
| `F5` | `compare.merge`: merge dialog, see [Merge](#merge) |
| `Alt+N` | `compare.relocated-name`: choose the focused column's name for a relocated file when merging; press again to drop the choice |
| `F8` | delete the focused column's file(s) |
| `Esc` | `compare.close` |

## Selecting in results

Selection acts on the focused column's panel: `Insert` / `Space` toggle the row's path, `Alt+K` selects a range, `*` inverts, `Ctrl+U` clears. `+` and `-` open the category dialog in select or unselect mode, choosing every visible row of the ticked classes (including one-sided classes). Selections persist in the panels after leaving the view and feed copy, move and delete as usual ([Selections](selections.md)).

Because delete in the view acts on the whole panel selection, opening Compare (from the panels or from Find duplicates) while either panel has selected paths first asks what to do with the selection: **Drop** clears it, **Stash** (the default) moves it into each panel's stash (merging with a stash already there) so `panel.stash-toggle` brings it back later, and **Cancel** leaves the selection and does not open Compare. Deleted files drop out of the results without a rescan.

## Merge

`F5` opens the merge dialog. Pick the destination side (left or right); the other side is the source. Merge acts on the selected rows of either panel, or on every visible row when nothing is selected, and runs as jobs.

- **Transfer**: copy files missing from the destination, and files whose content differs (overwriting asks through the usual [conflict](jobs.md) prompt).
- **Operation**: copy, or move (delete the source after transfer).
- **Relocated files**: a relocated file already exists in the destination under a different path. *Keep destination names* leaves it alone; *Rename to source names* renames the destination's copy to the source's path inside the destination tree (no data is copied, missing directories are created).

To decide per file, put the cursor on a relocated row, focus the column whose name should win and press `Alt+N` (`compare.relocated-name`, Actions menu, Name from this column). The row's icon turns into an arrow pointing away from the winning column (`→` left name, `←` right name) and the legend says which name merge uses; pressing it again in the same column returns the row to the dialog's choice. Per-file choices override the dialog and survive a refresh.

The preview line counts the files to copy or move and the files to rename.

## Options

`[compare]` in [config.md](config.md#compare): `hash_concurrency`, `read_buffer_kib`, `stay_on_volume_default` (`chunk_bytes` applies to find duplicates only).
