# Panel filter

M-f (leader `F`, rebindable in `keybindings.toml`) opens the Filter dialog, which narrows the active panel to matching entries. To mark entries instead of hiding the rest, use [Select group](selections.md#select-and-unselect-group).

The buttons are OK and Cancel, plus **Clear filter** (M-f) while a filter is active. Clear filter removes the filter and closes the dialog; in a panel it removes whatever filter is active, including a git status filter. OK with an empty pattern and no type checked also clears it.

While a filter set with this dialog is active, reopening the dialog shows that filter's settings (pattern, mode, types and options), ready to adjust. Otherwise the pattern opens prefilled with the extension of the file under the cursor (e.g. `*.stl`), shown dimmed: Enter uses it, typing replaces it, editing keys keep it. Directories and files without an extension open empty. In Compare and Find duplicates the cursor row is used the same way. Each panel, Compare and Find duplicates remember their own filter.

## Pattern

Pick how the pattern is read: Simple / auto shell (the default), Shell patterns (`*.log`), or Regular expression. Simple / auto shell matches the text anywhere in the name (`notes` finds `old-notes.txt`); as soon as the pattern contains `*` or `?` it is read as a shell pattern instead, which must match the whole name (`*.log`). `[` alone does not switch, so `[1080p]` still finds names containing it. "Case sensitive" applies to every mode (regex included). With meta columns present, "Include meta" also matches column values and "Only meta" matches them exclusively.

## File types

Videos, Pictures, Archives, Audio and Documents are checkboxes beside the mode radios. Checking several shows files of any checked type. What counts as each type is configurable, see `[filter.types]` in [config.md](config.md).

## Combining

An entry is shown when it matches the checked types **and** the pattern; an empty pattern means types only. Directories are filtered the same way as files, except that in tree view a matching entry keeps its parent folders visible.

"Files only" and "Directories only" are checkboxes: checking one unchecks the other. While "Directories only" is selected, the type checkboxes are shown disabled: they cannot be focused or toggled and the checked types are ignored. They keep their state and apply again when "Directories only" is cleared.

## Invert

"Invert" shows the entries that do not match. Files only / Directories only still apply afterwards, so Invert plus Files only lists the files that do not match.

## Keys

Alt+letter toggles the underlined option: Simple / auto shell L, Shell patterns S, Regular expression R, Videos V, Pictures P, Archives A, Audio U, Documents T, Files only Y, Directories only D, Invert I, Case sensitive E, Include meta M, Only meta N; Alt+F presses Clear filter when it is shown (otherwise it moves a word forward in the pattern). Tab jumps between the mode radios, type checkboxes, pattern, options and buttons.

The panel border shows the active filter, for example `Filter: not report Videos, Audio`. A live count of matches is shown beside the Pattern label.

## Saved patterns and history

A pattern together with its mode, file types and options can be saved and reused, in the Filter dialog and in the Select and Unselect group dialogs:

- `F5` saves it: enter a name and an optional description. Saving under an existing name replaces that pattern. `F5` again in the name prompt lists the saved patterns to overwrite; the overwritten pattern keeps its description unless you typed a new one.
- `F2` lists the saved patterns; type to narrow the list. `Enter` fills the dialog from the selected pattern, then OK applies it.
- `F3` lists the patterns applied this session in that dialog, most recent first. Filter and Select group keep separate lists, which last until pc exits.
- `F8` in either list deletes the selected entry.

Both dialogs share one list of saved patterns, so a pattern saved in Filter can be loaded in Select group and the other way round. The lists open on the pattern you last loaded or saved. A loaded pattern keeps only the options the dialog shows:

- Invert applies in Filter only, Match full path in Select group only (where that checkbox is shown).
- In the Compare and Find duplicates views, Filter ignores Files only, Directories only and the meta options.
- Without meta columns, the meta options are ignored.

Saved patterns live in `filters.toml` in the config directory ([`[filter] file`](config.md#filter)). The keys can be rebound under `[dialog.pattern]` in `keybindings.toml`.

## Compare view

In the Compare panels view M-f (leader letter `f`) opens the same dialog and filters the result rows by file name (the basename of the row's path). It stacks with the category filter (F3) and the empty-file toggle, and merge (F5) only sees the rows that remain. Rows are always files, so the meta and Files only / Directories only options are not shown. The border shows the pattern after the category, for example `All · Filter: *.jpg`. Clear filter, OK with an empty pattern, or S-F3 (reset filter) removes it; the category filter stays.

## Find duplicates view

In the Find duplicates results view M-f (leader letter `f`) opens the same dialog and filters whole duplicate groups: a group is shown when any of its copies has a file name that matches (types and pattern), and Invert shows the groups where no copy matches. A shown group keeps all its copies, in both the groups tree and the directory tree. Rows are always files, so the meta and Files only / Directories only options are not shown. The border shows the pattern, for example `View: Dirs · Filter: *.jpg`. Clear filter or OK with an empty pattern removes it, and a rescan resets it.

Marks on files in groups the filter hides are kept and still counted: deleting acts on every marked file, shown or not.

## Select group

The Select group (see [Selections](selections.md#select-and-unselect-group)) (`+`) and Unselect group (`-`) dialogs, in the panel, in Find results and in Find duplicates results, have the same mode radios and the same Videos, Pictures, Archives, Audio and Documents checkboxes (same `[filter.types]` config), and every option shared with Filter uses the same Alt letter. Checked types are OR-ed and AND-ed with the pattern; an empty pattern with types checked selects every entry of those types, and an empty pattern with no types selects nothing. Types always match the file name, even with "Match full path". As in Filter, "Directories only" disables the type checkboxes and ignores the types. There is no Invert. Match full path (Find results, Find duplicates results and the command output dialog) is M-h. In a finished user-menu output dialog the same dialogs act on the paths detected in the output (see the user menu section of [config.md](config.md)).
