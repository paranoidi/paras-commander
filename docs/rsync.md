# Rsync

Rsync syncs the selected entries to a destination with the installed `rsync`, as a background job in the [job queue](jobs.md). Use it where Copy and Move are not enough: copying only what changed, mirroring a directory (deleting what is gone from the source), comparing by checksum, limiting bandwidth, or previewing a sync with a dry run. It needs `rsync` on `PATH`; without it the command shows an error instead of the dialog.

| | |
|---|---|
| Key | unbound by default (assign one in `keybindings.toml`) |
| Menu | File -> Rsync... (`R`) |
| Leader | Esc, `S` |

Rsync works on the selected entries, or the highlighted entry when nothing is selected. The dialog title shows the item count.

Each option has an Alt+letter shortcut, shown underlined: Update U, Mirror M, Move V, Archive A, Checksum compare H, Only newer N, Skip existing S, Copy dir contents D, Compress Z, Dry run R (Alt+O is OK and Alt+C is Cancel).

## The dialog

- **Mode**:
  - **Update** copies new and changed files and leaves everything else in the destination alone.
  - **Mirror** also deletes destination files that are not in the source, so the destination ends up identical. OK is drawn in the destructive style. Check with **Dry run** first.
  - **Move** deletes each source file once it has been transferred. rsync leaves the emptied source directories behind.
- **Destination** starts with the other panel's directory. It is a directory: the sources land inside it, and it is created if missing, but its parent directory must exist (add `--mkpath` to the extra arguments to create the whole path); otherwise the job fails at once. It has path completion, the Bookmarks, History and Pinned pickers, and the shortcuts that insert the active or inactive panel's path, like the [copy and move](fileoperations.md#copy-and-move) dialogs. While the destination is a panel's directory that panel is outlined (in a warning colour for Mirror).
- **Archive** (on by default) preserves permissions, times, owners, symlinks and recurses. Unchecked, rsync still recurses but preserves nothing.
- **Checksum compare** decides what changed by file checksum instead of size and modification time (slower, exact).
- **Only newer** skips files that are newer at the destination.
- **Skip existing** never touches a file that already exists at the destination.
- **Copy dir contents** puts the contents of each source directory into the destination instead of the directory itself (rsync's trailing slash). Without it, syncing `orchard` to `/srv/meadow` gives `/srv/meadow/orchard`. It is available only when every source is a directory.
- **Compress** compresses data in transit; it helps over a slow network, not between local disks.
- **Dry run** changes nothing; it lists what would change (see below).
- **Exclude**: patterns separated by spaces, each passed to rsync as an exclude.
- **Bandwidth limit (KiB/s)**: a whole number, empty for no limit.
- **Extra arguments**: any other rsync options, written like a command line (quotes work). They are placed before the paths. Use this for everything the dialog does not offer, for example hard links, ACLs, `--partial` or filter rules.

Archive, Checksum compare, Copy dir contents, Compress and Skip existing show a short explanation in a tooltip once they have had focus for a short pause.

The last rows show the command that will run (it wraps between arguments, and on a short terminal ends in "..." when it does not all fit), so you can check exactly what the checkboxes and fields mean. Anything wrong with the fields (a bandwidth limit that is not a number, an unclosed quote in the extra arguments, a destination rsync cannot reach) replaces the command with the message, and OK keeps the dialog open.

The options of the last run, apart from the destination, are offered again the next time the dialog opens, until you quit.

## The job

The job shows progress by bytes and files with an ETA and the speed, and the file being transferred. Cancel stops rsync cleanly; Retry runs the same command again, and rsync only transfers what is still missing. An rsync job cannot be paused, soft stopped or undone.

An rsync that finishes with an error fails the job. The job's error is rsync's exit meaning (for example "partial transfer due to error"), and everything rsync printed on its error stream, such as the files it could not transfer or the reason it stopped, is listed in the job's Messages pane. Warnings from a run that succeeded (for example a skipped special file) appear there too. A destination that is a file, or whose parent directory does not exist, fails the job before rsync starts, for a dry run as well as a real run.

Rsync older than 3.1 (for example the one macOS ships) does not report progress: the job shows the file names as they go but no byte progress or ETA.

### Dry run

A dry run job ends with a results summary such as "12 changes". Open it from the jobs view with **Show results** to see the itemized list in a scrollable dialog: the first column is rsync's change code (`>f+++++++++` is a new file, `*deleting` a deletion), then the name. "No changes" means the destination is already up to date. Run the dialog again with Dry run unchecked to do it for real.

## Remote locations

If the source or the destination is on an [SFTP](sftp.md) panel, rsync runs over ssh to that host (using your `~/.ssh/config`). Only one side can be remote. Because rsync starts its own ssh rather than reusing the panel's connection, and has no terminal to ask on, the host needs key or agent authentication and a known host key; a host that wants a password fails with ssh's error. Remote paths with spaces may need rsync 3.2.4 or newer.

## Not included

Pausing, per-file conflict prompts (rsync decides by its own rules; use Dry run to see them), snapshots with `--link-dest`, and rsync daemon (`rsync://`) targets. Extra arguments cover the rest.
