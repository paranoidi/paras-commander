# Subshell and terminal panel

`Ctrl+O` (`app.drop-to-shell`, menu **Display -> Open shell**, Esc menu `O`) switches between the file manager and one long-lived interactive shell (`$SHELL`, falling back to `/bin/sh`). Command history, environment variables and background jobs of that shell survive every toggle, and the shell opens in the active panel's directory. The same shell can also be shown as a strip below the panels.

## Toggling

Press `Ctrl+O` again inside the shell to return. If the shell is busy with a full-screen program (for example `htop`), it is still running when you come back.

- **Panel to shell**: when the shell's directory differs from the active panel, a `cd` is injected for you. Anything you had half-typed at the prompt is kept. If a foreground command is running in the shell, no `cd` is sent. Remote (SFTP) panel paths are never injected.
- **Shell to panel** (`sync_cwd_on_return`, default on): after leaving the shell, the active panel navigates to the shell's directory.
- The shell is started on the first `Ctrl+O` and ends when you `exit` it; the next toggle starts a fresh one.

If the persistent shell is switched off, unsupported (anything but Linux), or `command` is set, `Ctrl+O` suspends the TUI and runs a one-shot shell until it exits. This mode is not available on remote panels. Shell toggling is blocked while a modal dialog is open.

## Sending file names to the shell

`Alt+Enter` or `Shift+Enter` (`app.shell-insert-paths`) types the selected files' absolute paths, quoted and separated by spaces, into the shell's command line without pressing Enter, then switches to the shell. With nothing selected the highlighted file is used. You review or complete the command and run it yourself. A busy shell shows a warning and you stay in the file manager. It needs the persistent shell and a local panel.

## Terminal panel

The same shell can be shown as a strip above the footer, so you can see command output while browsing.

| Key | Action |
|---|---|
| `Ctrl+Alt+P` | Show or hide the panel (`terminal.toggle-panel`). Showing does not take focus |
| `Alt+P` | Focus the panel; opens it if hidden; leaves it if already focused (`terminal.focus`) |
| `Ctrl+K` / `Ctrl+J` | Grow / shrink the panel, only while it has focus (`terminal.grow`, `terminal.shrink`) |
| `Ctrl+O` | Drop into the full-screen shell, also from inside the panel |

While the panel has focus, all other keys go to the shell, including `F10` and `F1`, so full-screen tools work. The footer function-key hints are blanked meanwhile. Hiding the panel does not stop the shell; reopening shows its current content instantly. When focus leaves the panel, the active panel follows the shell's directory. Full-screen views (jobs, commands, ...) reclaim the strip and it returns when you go back; on very small terminals it is omitted.

The panel has no scrollback and no truecolor support.

The panel is also used by [Run for each](commands.md#run-for-each) with a pseudo-TTY: while such a batch is running it owns the panel and the panel keys above refuse with a warning.

## Shell notes

- zsh: `setopt HIST_IGNORE_SPACE` keeps the injected `cd` lines out of history.
- bash 5.1 and later: with an array-style `PROMPT_COMMAND`, the directory hook is appended to the string form.

## Options

`[shell]` in [Config](config.md#shell): `persistent`, `sync_cwd_on_return`, `command`, `terminal_panel_height` (default 10 rows, minimum 3). Panel colors use the `terminal.text` theme key ([Themes](themes.md)). Keys are rebindable in `keybindings.toml`; the panel keys live under `[terminal]`.
