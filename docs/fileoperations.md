# File operations: how a copy reaches the disk

This page explains what paras-commander does between reading a source file
and calling the copy done, why the job speed you see is what it is, and how
that compares to Midnight Commander and yazi. Everything here is tunable under
`[operations]` in `config.toml` (see [Config](config.md#operations)).

The dialogs that start these operations are described in
[Operation dialogs](#operation-dialogs) at the end; the queue they feed is
described in [Jobs](jobs.md).

## The copy path

For a local-to-local file, in order:

1. **Clone** (`cow_file_cloning`, default on): `ioctl(FICLONE)`. On a
   filesystem that supports it (btrfs, XFS with reflinks, ZFS with block
   cloning enabled, same dataset/pool) the file is done instantly with no
   data copied. When the filesystem cannot clone, or the host blocks the
   ioctl (as some containers do), the copy moves on to the next step.
2. **Kernel copy** (`copy_file_range`, default on): `copy_file_range(2)` in
   8 MiB chunks. Cross-pool / cross-filesystem copies end up here on modern
   kernels; the data still moves, but without a round trip through userspace.
3. **Userspace copy**: `read`/`write` with a `copy_buffer_kib` buffer
   (default 256 KiB), optionally hole-preserving (`sparse_file_copy`).

Progress is reported after every chunk on all three paths.

## What "done" means: durability

By default (`sync_after_each_file = true`) a file is not counted as
transferred until it has been `fsync`'d — the data is on the disk, not just
in the kernel's write cache. This matters most for **move**: sources are only
deleted after their copies are durable, so a power cut or a yanked USB
enclosure mid-move cannot leave you with neither copy.

`sync_min_file_kib` skips the fsync for small files (lots of tiny files fsync
slowly); `sync_at_job_end = true` with `sync_after_each_file = false` does one
sync pass at the end of the job instead.

## Write-behind and the speed display

Filesystems accept writes far faster than the disk can absorb them. ZFS
buffers up to `zfs_dirty_data_max` (up to 4 GB) and only starts throttling
`write()` at 60 % of that; Linux's page cache does the same with
`vm.dirty_ratio` (20 % of RAM by default). So a multi-gigabyte file can be
"written" at memory speed and then sit in a single end-of-file `fsync` for
tens of seconds while it actually drains to a slow target — with nothing
left to report. The Speed column and menu-bar pill are sampled on a fixed
clock and feed zero for idle time on purpose (they show what is happening,
not what happened), so during that fsync they decay toward zero and then
jump back when the next file starts filling the cache.

`sync_write_behind_mib` (default 64) bounds that gap: whenever a file is
going to be fsync'd anyway, it is also fsync'd every 64 MiB during the copy.
Reported progress can never run more than 64 MiB ahead of the disk, the
end-of-file fsync is short, and the displayed speed tracks the real write
rate for the whole file. Set it to `0` for a single fsync per file.

Costs and interactions to know about:

- Every fsync ends with a device cache-flush command. Expect a few percent
  of throughput on fast disks, more on USB bridges with slow flush latency;
  raise the value if that shows up.
- ZFS without a separate log device logs large sequential writes by
  reference (`WR_INDIRECT`): the data is written once to its final location,
  so periodic fsync does not double-write it. Snapshots are unaffected — they
  are crash-consistent regardless of userspace syncing.
- A dataset with `sync=disabled` turns fsync into a no-op; you then get the
  single-flush behaviour and its speed-display decay back.
- With `sync_after_each_file = false` no write-behind sync happens at all.
  The kernel's own write throttle then paces `write()` at disk speed once the
  cache is full, so progress is still accurate — but the job finishes with up
  to a cache's worth of data still in flight.

## Compared to other file managers

| | paras-commander | Midnight Commander | yazi |
|---|---|---|---|
| Clone / kernel copy | FICLONE → `copy_file_range` → userspace | FICLONE → userspace `read`/`write` | `std::fs::copy` (`copy_file_range`) |
| Progress source | Bytes written, per chunk | Bytes written, per buffer | Destination file size polled every 3 s |
| fsync | Per file by default, with write-behind every 64 MiB | Never | Never |
| Move = copy + delete source | After the copy is durable | After the copy returns (data may still be in cache) | After the copy returns |

mc and yazi trust the kernel: the file is "done" when `write()` returns, and
their speed never stalls because they never wait for the disk. That is also
why they never show the decay described above — and why a move in either can
delete the source while the destination is still only in RAM. paras-commander
defaults to the stricter semantics; `sync_after_each_file = false` gives you
the mc/yazi behaviour if you prefer it.

## Moving to another device

A move first renames every item it can, which is instant on the same
filesystem. Items that cannot be renamed (another device or another host) are
then copied and their sources deleted. The job counts those items in the
background while the copy starts, so its progress and ETA reflect the real size
of what is being copied; the **Details** panel shows `(scanning…)` until the
count finishes. Sources are deleted only once the count is done.

## Moving onto an existing directory

Move tries a plain rename first. When a moved directory meets a directory of
the same name at the destination, the two are **merged** rather than one
replacing the other: no prompt is shown for the directory itself, items that
exist only in the source are moved in, and sub-directories present on both
sides are merged the same way. You are asked about a conflict only for the
colliding items themselves (file over file, or a file and a directory sharing
a name). Skipping an item keeps it, and its parent directories, in the source.
Once everything is moved the emptied source directory is removed. A symbolic
link is never merged into or out of; it is treated as a single item.

## Files and directories never overwrite each other

Overwrite replaces a file with a file (or a link). It never replaces a
directory, and never replaces a file with a directory: when one side is a
directory (or a link to one) and the other is not, the conflict dialog says
`Destination is a directory` or `Destination is a file` and offers only Rename
(keep both), Skip and Cancel (never Delete source). "Overwrite all" and the advanced conflict rules
are not applied to such items; you are asked each time, while "Skip all" and
"Rename all" apply as usual. This holds for copy, move and flatten.

## Operation dialogs

Every operation works on the selected entries, or the highlighted entry when
nothing is selected. Delete and rename act on the entries that were selected
when the dialog opened, even if you move the cursor afterwards. All keys are
rebindable in `keybindings.toml`.

| Operation | Key | Action id | Menu |
|---|---|---|---|
| Copy | `F5`, `Ctrl+C` | `file.copy` | File -> Copy |
| Move | `F6` | `file.move` | File -> Move |
| Rename | `Shift+F6`, `Ctrl+R` | `file.rename` | File -> Rename |
| Duplicate | `Shift+F5` | `file.duplicate` | Esc menu `p` |
| Mkdir | `F7` | `file.mkdir` | File -> Mkdir |
| Mkdir and open | `Shift+F7` | `file.mkdir-open-in-other` | Esc menu `o` |
| Delete | `F8`, `Delete` | `file.delete` | File -> Delete |
| Symlink | `Ctrl+L` | `file.symlink` | File -> Symlink |
| Hard link | unbound | `file.hardlink` | File -> Link |
| Flatten | `Ctrl+Z` | `file.flatten` | File -> Flatten |
| Extract archive | `Ctrl+X` | `file.extract` | File -> Extract |

Symlinks, hard links, chmod and chown are local-only. All dialogs accept
`Alt+O` (OK) and `Esc` (Cancel).

### Copy and move

The destination field starts with the other panel's path; a leading `~` resolves against your home directory and a relative path against the active panel's directory. It supports path
completion and the Bookmarks, History and Pinned pickers; `Shift+Left` and
`Shift+Right` insert the active or inactive panel's path. While you type, the
panel whose directory matches the destination is outlined (in a warning colour
for move). Move always asks for a destination; renaming in place is Rename.

- **Preserve permissions** (`Alt+R`), **Preserve timestamps** (`Alt+T`) and
  **Dereference symlinks** (`Alt+D`) are copy-only checkboxes. Dereference
  copies what a symlink points to instead of recreating the link; it is
  available only between local locations. Defaults:
  `preserve_permissions`, `preserve_timestamps`, `dereference_symlinks`.
- **Flatten into destination** (`Alt+I`) drops the directory structure and puts
  every item directly into the destination. Without it the structure relative
  to the sources' common parent is kept, and when the selection spans several
  directories the dialog shows the common source and a preview of the result.
- **Advanced** (`Alt+A`) switches the dialog to the Conflict rules screen, the
  same rule matrix the job offers at its first "File exists" prompt, so the
  rules can be set up front (the Delete source choices appear for a move). The
  screen also has, for a move, **Remove empty directories** (`Alt+R`), which
  removes the source directories the move left empty once it finishes, without
  asking, and **Add paused** (`Alt+A`), which queues the job without starting
  it. OK queues the job; Cancel returns to the main screen and keeps what
  was set. Rules left untouched leave conflicts to ask as usual.
- Copying or moving a single item onto itself asks for a new name; copying a
  directory into itself is refused.
- After a copy or move into the other panel's current directory finishes, the
  first transferred entry is highlighted there, unless you moved that panel's
  cursor meanwhile (`focus_other_panel_after_transfer`).
- When a move or delete leaves directories empty, a prompt offers to remove
  them (`remove_dangling_directories`).

Name collisions are resolved in the jobs view or the quick dialog, including
the size and time based Conflict rules, content verification (Same / Differ)
and, for a move, Delete source; see [File conflicts](jobs.md#file-conflicts).

### Duplicate

Copies exactly one file or directory under a new name in the same place.
The name is prefilled with the original name plus `.1`, or the next free
number (`.2`, `.3`, ...) when that name is already listed. The name field behaves like Rename, including Sanitize and Slugify and the
**Focus after duplicate** checkbox; the focus moves to the copy once the job
has created it.

### Rename

Rename works within one directory and refuses path separators. The **Focus
after rename** checkbox (`Alt+A`, default from `rename_focus_after`) selects
and centres the new name; for a directory, **Open in the other panel after
rename** (`Alt+P`) also opens it in the inactive panel. Inside the name field:

| Key | Opens |
|---|---|
| `F2` | Sanitize: removes characters that are awkward in file names |
| `F3` | Slugify: lowercase, hyphen-separated names |
| `F4` | Encoding: repairs names stored in a legacy Japanese encoding, with a preview of the decoded name. Offered only when such an encoding is detected |

Leading or trailing whitespace in the new name triggers a warning. With a
selection, Rename becomes [Mass rename](mass-rename.md).

### Mkdir

The name is prefilled from the highlighted entry (a file drops its last
extension). After Create, the new directory is highlighted. `Shift+F7`
additionally opens the new directory in the inactive panel. With a selection,
radios offer **Create**, **Create and copy** and **Create and move**: the
selected items are queued into the new directory and the selection is cleared.
With two or more selected items, `F7` inside the name field fills in the
longest name part they share. Mkdir also works on SFTP panels.

### Delete

The dialog lists the entries (scrollable when long) with a size summary that
fills in while the disk-usage scan runs. `Alt+Y` and `Alt+N` answer from anywhere in the dialog. Deleting is queued as a job.
Delete jobs start at once and run in parallel with each other and with a
running copy or move, except when their paths overlap a running transfer,
in which case they wait for it. Switch the confirmation off with
`confirm_delete = false`.

### Symlink

`Ctrl+L` asks for the link target and the link path. Hard links (File -> Link) work
the same way.

### Flatten

Needs a selection of directories only. The dialog asks for the destination and
two options: **recursive** (take files from all depths instead of only the
immediate children) and **remove empty directories** afterwards (defaults
`flatten_recursive`, `flatten_remove_empty_dirs`). Files that already exist
at the destination go through the normal conflict prompt. Progress shows a
count of moved items without a total.

### Permission denied

When an operation fails for lack of permission, see [Elevation](elevation.md).
