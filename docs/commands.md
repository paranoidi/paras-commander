# Commands, user menu and run for each

Paras Commander can run external commands on the files you select. Three entry points share one engine: **Run for each** (type a command, run it once per selected item), the **user menu** (F2, your own commands from `menu.toml`) and **Enter on an executable file**. Output is captured and kept in the **Commands view**, so nothing disappears when the command ends.

For an interactive shell instead, see [Subshell](subshell.md).

## Macros

Command lines may contain these placeholders. Do not put quotes around them: path and name values are quoted for you, so spaces and quotes in file names are safe.

| Macro | Expands to |
|---|---|
| `%f` | Name of the highlighted file on the active panel (in run for each: the absolute path of the item being processed) |
| `%F` | Name of the highlighted file on the other panel |
| `%d` | Directory of the highlighted row on the active panel |
| `%D` | Directory of the highlighted row on the other panel |
| `%t` | Tagged (selected) files of the active panel, one quoted argument each |
| `%T` | Tagged files of the other panel |
| `%%` | A literal `%` |

A line containing shell operators (`|`, `>>`, `&&`) runs through `sh -c`; any other line is split into arguments directly.

## Run for each

Runs one command once per selected item (or the highlighted item when nothing is selected).

- **Open**: `Ctrl+Alt+R` (`file.run-for-each`), menu **Command -> Run for each...** (`f`), or Esc menu `R`.
- The command must contain `%f`. The field validates as you type: an invalid command shows a red message, a valid one shows `-> <expanded command>` for the first item.
- **Run in each selected directory** (`Alt+R`): runs the command with each selected directory as its working directory instead of the active panel's directory. Only directories may be selected in this mode, and `%f` becomes optional.
- **Allocate pseudo-TTY (interactive)** (`Alt+T`): runs each item on a live terminal shown in the bottom [terminal panel](subshell.md#terminal-panel), for tools that need a real terminal. Only one foreground batch uses the panel at a time; others wait. The Commands view records the final screen of each run. Linux only; elsewhere it falls back to normal capture.
- **Worker pool**: when `pools.toml` defines pools, radios let the batch share a limited number of parallel slots; see [Worker pools](#worker-pools).
- **History** (`F3`, `file.run-for-each.history`): fuzzy picker over the command lines you ran earlier in this session.

The selection is cleared when the batch starts. A failed batch shows a summary toast and opens the Commands view.

## Commands view

Open with `Alt+C` (`commands.open`), **Display -> Commands** or Esc menu `E`. The left list holds one row per run, newest at the top, with its phase and exit marker; opening the view selects the newest run; the command is shown as written, followed by one line per macro it used. Stdout and Stderr appear in two panes, wrapped to the pane width.

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Cycle list, stdout, stderr |
| `Up` `Down` `PgUp` `PgDn` `Home` `End` | Move through the run list as drawn (`Up` toward newer runs, `Home` the top row), or scroll the focused output pane. Panes follow the tail until you scroll up; reaching the bottom follows again |
| `F8` | Terminate the selected running command (`commands.terminate`) |
| `Shift+F8` | Kill it (`commands.kill`) |
| a printable character | Type-to-jump filter over the command lines |
| `Left` / `Esc` | Close the view (`commands.close`; Esc first closes an open filter) |

Quitting with commands still running asks for confirmation, like unfinished jobs.

## User menu

`F2` (`app.user-menu`, also `<`; menu **Command -> User menu**, Esc menu `X`) opens a strip of your own commands. Press the highlighted letter to run an entry; `Left` or `Backspace` leaves a submenu, `Esc` closes the menu. `F9` inside the menu, or `Shift+F2` (`app.user-menu-edit`) anywhere, opens `menu.toml` in `$EDITOR` and reloads it. A missing file is created with a commented reference of every field.

A `menu.toml` in the active directory (name from `local_names`, default `menu.toml`) takes precedence over the global one; see [Config](config.md#user_menu). The same trust rule as [Meta columns](meta-columns.md#open) applies: a group- or world-writable `menu.toml` (other than one writable only by your private group) or one owned by another user is skipped with a warning, and the user menu then shows that warning instead of opening the editor.

### Entry modes

| Field | Behaviour |
|---|---|
| (none) | Output is captured, the Commands view opens and gets a row |
| `dialog = true` | Output is shown in a dialog that opens at once and fills while the command runs. **Background** (`Esc`, `Alt+B`) leaves it running in the Commands view, **Cancel** (`Alt+C`) kills it |
| `background = true` | Runs silently; you are notified only on failure or stderr output; the panel refreshes when done |
| `interactive = true` | Suspends the TUI and gives the command the terminal (vim, lazygit, htop) |
| `detach = true` | Starts the process and forgets it (GUI helpers such as `xdg-open`) |
| `run_for_each = ["files", "dirs"]` | Runs once per selected item of the listed kinds; the command must contain `%f` |
| `pty = true` | With `run_for_each`: run each item on a pseudo-TTY |
| `pool = "name"` | Waits for a slot in a [worker pool](#worker-pools) |
| `keep_open = true` | Keeps the menu open after launching (needs `background`, `detach` or `interactive`) |

Other fields: `title` (required), `command` (required unless the table is a submenu), `key` (single activation letter, otherwise derived from the title), `default = true`, `toast` (message after success), `shell = true` (force `sh -c`), `when` (visibility), `shell_patterns`, `dialog_width` and `dialog_height` (`"80%"` or a number). `interactive` cannot be combined with `detach`, `background`, `dialog` or `run_for_each`.

`when` takes the same predicate language as [Predicates](predicates.md): a bare pattern such as `"*.png"` matches the highlighted file name, and a list means "any of".

Entries are uniquely named TOML tables in the order you write them. A table that contains child tables instead of `command` is a submenu, nested as deep as you like. Load errors (unknown field, bad `when`, `run_for_each` without `%f`, unknown `pool`) are shown as a critical message.

```toml
[shell_here]
title = "Open shell script"
command = "bash %f"
when = "*.sh"

[convert]
title = "Convert to PNG"
command = "magick %f %f.png"
run_for_each = ["files"]
when = ["*.jpg", "*.webp"]
pool = "cpu"
background = true
toast = "Converted"

[tools]
title = "Tools"

[tools.disk_use]
title = "Disk usage"
command = "du -sh %t"
dialog = true

[tools.editor]
title = "Edit in vim"
command = "vim %f"
interactive = true

[open_gui]
title = "Open with default app"
command = "xdg-open %f"
detach = true
```

### Output dialog paths

When a `dialog = true` command finishes, paths printed in its output that exist (relative ones are taken from the directory the command ran in) are highlighted. `Up`/`Down` jump between lines with paths, `Left`/`Right` step between paths, and `Insert` or `Space` (`command-output.toggle-selection`) toggles the path's selection in the active panel. The select group, unselect group and invert selection keys of the panels work on all detected paths.

### Selection after a command

A command clears the tags it consumed when it starts: `%t` clears the active panel, `%T` the other panel, and a command using neither leaves selections alone. `interactive` and `detach` entries clear only after a successful start. `run_for_each` entries and the Run for each dialog clear the active panel.

## Worker pools

Pools limit how many commands run at once. Define them in `pools.toml` (path from `[pools]` in `config.toml`, default `<config dir>/pools.toml`):

```toml
[[pools]]
name = "cpu"
max_parallel = 4

[[pools]]
name = "network"
max_parallel = 1
```

`max_parallel` is clamped to 1-64 and names must be unique. A user-menu entry with `pool = "cpu"` (default or `background` mode) waits for a free slot before it runs; the same pool caps how many items of one `run_for_each` batch run at once, while without a pool they run one after another. A pooled run shows as queued until it gets a slot. An unknown pool name is reported when the menu loads. Copy and move jobs do not use pools.

## Enter on an executable and background launches

With `run_executables_on_enter` (default on), `Enter` on a local executable file (ELF or shebang script) runs it in the active panel's directory and records it in the Commands view. `[[panels.execute_rules]]` decide the mode per file: a rule with `background = true` (built in for `*.AppImage`) keeps you in the file view and only notifies on launch failure, a non-zero exit or stderr output; without a matching rule the Commands view opens and follows the run. Remote panels never execute locally. `F4` always edits the file.

## Options

- `[user_menu]`, `[pools]`, `[commands]` (`force_color`) and `[panels]` (`run_executables_on_enter`, `execute_rules`): [Config](config.md#user_menu).
- Keys: [Keybindings](keybindings.md); everything above is rebindable in `keybindings.toml` (`[commands]` for the view keys, `[dialog.user_menu]` for `usermenu.back`, `[dialog.run_for_each]` for history, `[dialog.command_output]` for toggle-selection).
