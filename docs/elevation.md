# Running as root (elevation)

paras-commander never runs as root itself. When something needs root it asks
`sudo` for that one step.

## Editing with F4 (sudoedit)

When you press **F4** on a local file you cannot write to (or on a new file in
a directory you cannot write to), the file is opened with `sudo -e`
(sudoedit): `sudo` asks for the password on the released terminal and you edit
a temporary copy, which `sudo` copies back when you quit the editor. Your own
editor (`$VISUAL`, then `$EDITOR`, then `vi`) is used via `SUDO_EDITOR`. A line
`pc: <path> is not writable, editing via sudoedit` is printed first. Read-only
filesystems are never elevated.

Switch off with `elevate = false` under `[editor]` in `config.toml`.

## File operations that fail with "permission denied"

If a local **rename, same-filesystem move, delete, mkdir, chmod, chown,
symlink or hardlink** fails with a permission error, a **Permission denied**
dialog offers to retry just that step as root:

- **Elevate** (`Alt+E`, or `Enter` in the password field) runs the step through
  `sudo` with the password you typed (shown as `*`). A wrong password asks
  again.
- **Skip** (`Alt+S`) leaves that item alone and carries on with the next one.
- **Cancel** (`Alt+C`, `Esc`) stops the operation.

If `sudo` needs no password (for example a `NOPASSWD` rule or a cached
credential), the password field is left out and the dialog just asks for
confirmation.

Details:

- Only the failed step is run as root, through a fixed command (`mv -T`,
  `rm -rf`/`rm -f`, `mkdir`, `chmod`, `chown`, `ln -s -T`, `ln -T`), never
  through a shell. These are the GNU coreutils flags, so elevation expects a
  GNU userland.
- Within one copy/move/delete job the password is remembered after the first
  successful elevation, so later items in the same job do not ask again. It is
  kept in memory only for the lifetime of that job; it is never written to disk,
  logs, or the job list. Foreground operations (rename, mkdir, chmod, ...) ask
  each time.
- Things created by an elevated step (a new directory, a symlink) are owned by
  root.
- Read-only filesystems (`EROFS`) are not a permission problem and are never
  offered for elevation. Remote (SFTP) locations are never elevated.
- A job that is waiting on this dialog still holds its place in the queue.

### Not elevated

- The content writes of a **copy**, or of a **move across filesystems**
  (copy + delete), are not elevated. Only the final same-filesystem rename of a
  move is.
- Mass rename and listing directories you cannot read are not elevated.

### Configuration

`[operations] elevate` (default `true`) turns the dialog on or off; see
[config.md](config.md#operations). When off, permission errors are reported as
before.

See also [File operations](fileoperations.md#operation-dialogs) for the dialogs that can ask for elevation and [Jobs](jobs.md) for jobs waiting on this dialog.
