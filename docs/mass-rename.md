# Mass rename

Renames the selected entries (or the entry under the cursor) in one go. The dialog shows a
live before/after preview with the changed text highlighted; nothing is renamed until you
confirm. Swaps and chains (a to b, b to a) are handled safely.

## Modes

- **Simple (replace text)**: replaces every occurrence of Find with Replace.
- **Regular expression**: Find is a Go (RE2) pattern, Replace is a template using `$1`,
  `${1}`, `\1` or named groups `${name}`; `${0}` is the whole match.
- **External $EDITOR**: opens the names in your editor, one per line.
- **Capitalize**: capitalizes words, optionally treating punctuation as word separators.

**Trim whitespace** strips leading and trailing whitespace from the new names.

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
