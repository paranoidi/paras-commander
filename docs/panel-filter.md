# Panel filter

Alt+F opens the Filter dialog, which narrows the active panel to matching entries. OK with an empty pattern and no type checked clears the filter.

## Pattern

Pick how the pattern is read: Shell patterns (`*.log`), Regular expression, or Simple (plain substring). "Case sensitive" applies to shell and simple patterns. With meta columns present, "Include meta" also matches column values and "Only meta" matches them exclusively.

## File types

Videos, Pictures, Archives, Audio and Documents are checkboxes beside the mode radios. Checking several shows files of any checked type. What counts as each type is configurable, see `[filter.types]` in [config.md](config.md).

## Combining

An entry is shown when it matches the checked types **and** the pattern; an empty pattern means types only. Directories stay visible unless "Files only" or "Directories only" is set.

## Invert

"Invert" shows the entries that do not match. Files only / Directories only still apply afterwards, so Invert plus Files only lists the files that do not match.

## Keys

Alt+letter toggles the underlined option: Shell patterns S, Regular expression R, Simple L, Videos V, Pictures P, Archives A, Audio U, Documents T, Files only F, Directories only D, Invert I, Case sensitive E, Include meta M, Only meta N. Tab jumps between the mode radios, type checkboxes, pattern, options and buttons.

The panel border shows the active filter, for example `Filter: not report Videos, Audio`. A live count of matches is shown beside the Pattern label.
