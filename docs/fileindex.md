# fileindex

pc has [fileindex](https://github.com/paranoidi/fileindex) built in. fileindex is a
SQLite index of your files by content hash, with tags. In pc you can browse the
index like a directory tree, show tags as a panel column, add and remove tags,
and run scans and prunes as background jobs.

pc reads and changes an index that the `fileindex` command created. Run
`fileindex init` once before using it from pc; pc never creates or upgrades the
database. The database path and which files a scan includes come from
fileindex's `~/.config/fileindex/config.ini`, so the command and pc index the
same files. `[fileindex]` in pc's `config.toml` can override the database path
and the number of hash workers (see [Config](config.md#fileindex)).

When there is no index, or its version does not match, pc reports it once in
the Messages log and each fileindex action says so instead of running.

## Actions

All actions are in the leader menu's **fileindex** group and in Help (`F1`)
under fileindex. The keys below are the defaults; rebind them in
`keybindings.toml`.

| Key | Action | |
|---|---|---|
| `Alt+Shift+I` | `fileindex.open` | Show the current directory in the index |
| `Alt+Shift+R` | `fileindex.reveal` | From the index, go to the entry's real location |
| `Ctrl+A` | `fileindex.scan` | Scan the selection, else the entry under the cursor, else the directory |
| `Alt+Shift+P` | `fileindex.prune` | Remove what is gone from disk under the current directory |
| `Ctrl+T` | `fileindex.tag-add` | Add tags |
| `Ctrl+W` | `fileindex.tag-remove` | Remove tags |

Under tmux or screen with `Ctrl+A` as the prefix key, use the leader menu
(`S`) or bind `fileindex.scan` to another key.

## Browsing the index

`fileindex.open` shows the current directory as `fileindex:///path/to/dir`,
with the same entry under the cursor. Navigation then works as usual: history,
bookmarks and the path picker accept `fileindex://` paths too. The index root is
`fileindex:///`.

An index view lists what the index recorded, including files that have since
gone from disk. Files show their indexed size and modification time;
directories show neither. `fileindex.reveal` goes back to the real directory
with the entry highlighted, or says the entry is no longer on disk.

In an index view:

- **View and copy** read the real file, so copying out of the index works when
  the file is still on disk.
- **Rename** and **move** inside the index record that the entry moved. They
  do not touch the disk. The rename dialog says so.
- **Delete** removes the entry, or a directory and everything under it, from
  the index only. The delete dialog says so.
- **New directories, copies into the index, and moves out of it** are
  refused: the index is read-only for new content.
- Git status, disk usage, folder colours, the shell, the terminal panel and
  the external editor are not available, as on [SFTP](sftp.md) panels.

## Tags

Tags are shown by the **fileindex tags** column. Switch it on per panel from
the meta-column picker (`panel.meta`, see [Meta columns](meta-columns.md)).
It works on local panels and in index views, and lists a file's or a
directory's tags, comma separated. It can be sorted and filtered like any meta
column.

`fileindex.tag-add` and `fileindex.tag-remove` open a prompt for the
selection, else the entry under the cursor, else the current directory. Type
one or more tags separated by commas or spaces. **Recursive** applies the tags
to a directory, every directory under it and every file in them, instead of
the directory alone. Tags work on entries that are gone from disk too, and the
message after OK counts paths the index does not know. The tags column updates
when the edit finishes.

## Scan and prune

`fileindex.scan` hashes new and changed files under the chosen paths, then
removes from the index whatever under them is gone from disk.
`fileindex.prune` only does the removal, for the current directory. In an
index view both act on the real paths.

Both run as **task** jobs in the [jobs view](jobs.md): the Details panel shows
the files done, bytes hashed and current path, and per-file errors and pruning
removals are listed as warnings. Cancel stops a scan between files and keeps
everything already committed. One fileindex job runs at a time; another
waits in the queue. Task jobs cannot be paused or retried; run the action
again instead.

The index does not yet follow renames, moves and deletes that pc makes on
disk; rescan or prune to bring it up to date.
