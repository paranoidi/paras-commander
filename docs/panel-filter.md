# Panel filter

Alt+F opens the Filter dialog, which narrows the active panel to matching entries. OK with an empty pattern and no type checked clears the filter.

## Pattern

Pick how the pattern is read: Shell patterns (`*.log`), Regular expression, or Simple (plain substring). "Case sensitive" applies to every mode (regex included). With meta columns present, "Include meta" also matches column values and "Only meta" matches them exclusively.

## File types

Videos, Pictures, Archives, Audio and Documents are checkboxes beside the mode radios. Checking several shows files of any checked type. What counts as each type is configurable, see `[filter.types]` in [config.md](config.md).

## Combining

An entry is shown when it matches the checked types **and** the pattern; an empty pattern means types only. Directories are filtered the same way as files, except that in tree view a matching entry keeps its parent folders visible.

"Files only" and "Directories only" are checkboxes: checking one unchecks the other. While "Directories only" is selected, the type checkboxes are shown disabled: they cannot be toggled and the checked types are ignored. They keep their state and apply again when "Directories only" is cleared.

## Invert

"Invert" shows the entries that do not match. Files only / Directories only still apply afterwards, so Invert plus Files only lists the files that do not match.

## Keys

Alt+letter toggles the underlined option: Shell patterns S, Regular expression R, Simple L, Videos V, Pictures P, Archives A, Audio U, Documents T, Files only Y, Directories only D, Invert I, Case sensitive E, Include meta M, Only meta N. Tab jumps between the mode radios, type checkboxes, pattern, options and buttons.

The panel border shows the active filter, for example `Filter: not report Videos, Audio`. A live count of matches is shown beside the Pattern label.

## Compare view

In the Compare panels view Alt+F (`compare.filter-dialog` in `[compare]`, leader letter `f`) opens the same dialog and filters the result rows by file name (the basename of the row's path). It stacks with the category filter (F3) and the empty-file toggle, and merge (F5) only sees the rows that remain. Rows are always files, so the meta and Files only / Directories only options are not shown. The border shows the pattern after the category, for example `All · Filter: *.jpg`. OK with an empty pattern, or Shift+F3 (reset filter), clears it.

## Find duplicates view

In the Find duplicates results view Alt+F (`dedup.filter-dialog` in `[dedup]`, leader letter `f`) opens the same dialog and filters whole duplicate groups: a group is shown when any of its copies has a file name that matches (types and pattern), and Invert shows the groups where no copy matches. A shown group keeps all its copies, in both the groups tree and the directory tree. Rows are always files, so the meta and Files only / Directories only options are not shown. The border shows the pattern, for example `View: Dirs · Filter: *.jpg`. OK with an empty pattern clears it, and a rescan resets it.

Marks on files in groups the filter hides are kept and still counted: deleting acts on every marked file, shown or not.

## Select group

The Select group (`+`) and Unselect group (`-`) dialogs, in the panel and in Find results, have the same mode radios and the same Videos, Pictures, Archives, Audio and Documents checkboxes (same `[filter.types]` config), and every option shared with Filter uses the same Alt letter. Checked types are OR-ed and AND-ed with the pattern; an empty pattern with types checked selects every entry of those types, and an empty pattern with no types selects nothing. Types always match the file name, even with "Match full path". As in Filter, "Directories only" disables the type checkboxes and ignores the types. There is no Invert. Match full path (Find results and the command output dialog) is Alt+H. In a finished user-menu output dialog the same dialogs act on the paths detected in the output (see the user menu section of [config.md](config.md)).
