# Mass rename

Renames the selected entries (or the entry under the cursor) in one go. The dialog shows a
live before/after preview with the changed text highlighted; nothing is renamed until you
confirm. Swaps and chains (a to b, b to a) are handled safely.

Open it with `Shift+F6` or `Ctrl+R` (`file.rename`) while entries are selected
in the current directory; without a selection the same key opens the
single-entry [Rename](fileoperations.md#rename) dialog. Rebind it in
`keybindings.toml`. If any new name is a duplicate, collides with an existing
file or is illegal, the affected rows are flagged and OK stays blocked until
the conflicts are gone.

## Modes

- **Simple (replace text)**: replaces every occurrence of Find with Replace.
- **Regular expression**: Find is a Go (RE2) pattern, Replace is a template using `$1`,
  `${1}`, `\1` or named groups `${name}`; `${0}` is the whole match.
- **External $EDITOR**: opens the names in your editor, one per line, in the same order as the preview. Keep the line count unchanged; OK stays disabled until you have saved a result, and `Enter` on the mode reopens the editor.
- **Capitalize**: capitalizes words. **Capitalize each word** capitalizes every word, and **Treat punctuation as separators** (which turns the former on) also starts a new word after punctuation. By default only the first character of the name is capitalized, and only when it is a letter (a name starting with a digit, space or symbol is left unchanged).

**Show only modified** hides unchanged rows in the preview. **Case insensitive** applies to Simple and Regular expression matching. **Trim whitespace** strips leading and trailing whitespace from the new names.

**Ignore extension** (on by default, `Alt+X`) limits Simple, Regular expression and Capitalize
to the file name without its extension, so `.` to a space turns `walnut.pear.txt` into
`walnut pear.txt`. The extension is the part after the last dot; dotfiles such as `.bashrc` and
directories have none. External $EDITOR ignores this option. The setting is saved with patterns.

## Zero-padding numbers

In Regular expression mode, `${N:W}` expands group `N` (a number or a group name) left-padded
with `0` to `W` characters. Groups already `W` characters or longer are left unchanged, an
unmatched group counts as empty, and `W` is capped at 255.

| Pattern  | Replace   | Before        | After          |
|----------|-----------|---------------|----------------|
| `\d+`    | `${0:3}`  | `walnut 1.txt`  | `walnut 001.txt` |
| `\d+`    | `${0:3}`  | `walnut 10.txt` | `walnut 010.txt` |
| `E(\d+)` | `E${1:2}` | `maple E5.txt`  | `maple E05.txt`  |

Every number matched by the pattern is padded, so `maple1pear2` with `\d+` becomes
`maple001pear002`. Use a more specific pattern such as `E(\d+)` to pad only one number.

To pad only the last number, use `(\d+)(\D*)$` with `${1:3}${2}`: `walnut 7 pear 5.txt` becomes
`walnut 7 pear 005.txt`. The built-in patterns *Pad last number to 2 digits* and *Pad last
number to 3 digits* (F2 Load pattern) do exactly this.

## Apply, saved patterns and history

- **Apply** (`Alt+A`) renames now and keeps the dialog open: the panels refresh, the batch is re-selected under its new names and the Find and Replace fields are cleared, ready for the next rule. OK renames and closes.
- Patterns (mode, Find, Replace and options) can be saved and reused. In the dialog: `F2` Load pattern, `F5` Save pattern (asks for a name and description; `F5` again offers to overwrite an existing pattern), `F3` History (patterns used in this session only), `F8` deletes the selected saved pattern in the pickers.
- Saved patterns live in `patterns.toml` (`[mass_rename] file`, default `<config dir>/patterns.toml`). Until the file exists, Load shows the two built-in pad patterns above. See [Config](config.md#mass_rename).
- The dialog keys are under `[dialog.mass_rename]` in `keybindings.toml`.
