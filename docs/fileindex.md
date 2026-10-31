# fileindex

pc has [fileindex](https://github.com/paranoidi/fileindex) built in. fileindex is a
SQLite index of your files by content hash, with tags. In pc you can browse the
index like a directory tree, show tags as a panel column, add and remove tags,
and run scans and prunes as background jobs.

pc shares its index with the `fileindex` command. Set the database location
with `database` under `[fileindex]` in pc's `config.toml`; left empty, pc uses
the path from fileindex's `~/.config/fileindex/config.ini`. Which files a scan
includes always comes from that `config.ini`, so the command and pc index the
same files. `workers` sets the number of hash workers (see
[Config](config.md#fileindex)).

When there is no database at that location, the first fileindex action (or
the tags column, or a `fileindex://` path) asks whether to create an empty one.
**Yes** creates it and carries on with the action; **No** leaves everything as
it was, and the next action asks again.

A database from an older fileindex version gets the same kind of question:
**Yes** upgrades it (what `fileindex init` does) and carries on. Upgrading
rebuilds the search indexes and can take minutes on a large index; other
fileindex work waits until it is done. A database from a newer fileindex
version is reported in the Messages log, and fileindex actions say so instead
of running; it needs a newer pc.

## Actions

`<` (`fileindex.menu`) opens the fileindex menu at the bottom of the screen;
the `:` leader menu lists it under Tools with the same `<`. The menu groups
its actions into Browse, Tagging and Actions columns. Each action's
letter in the menu is the letter of its own shortcut, in lowercase: every
fileindex shortcut is `Alt+Shift` plus that letter. All actions are also in Help (`F1`) under
fileindex. The keys below are the defaults; rebind the shortcuts under
`[main]` and the menu letters under `[fileindex_menu]` in `keybindings.toml`.

| Key | Menu | Action | |
|---|---|---|---|
| `Alt+Shift+I` | `i` | `fileindex.open` | Show the current directory in the index |
| `Alt+Shift+R` | `r` | `fileindex.reveal` | From the index, go to the entry's real location |
| `Alt+Shift+S` | `s` | `fileindex.scan` | Scan the selected entries, else the current directory (the menu says which) |
| `Alt+Shift+P` | `p` | `fileindex.prune` | Remove what is gone from disk under the current directory |
| `Alt+Shift+T` | `t` | `fileindex.tag-add` | Add tags |
| `Alt+Shift+U` | `u` | `fileindex.tag-remove` | Remove tags |

## Browsing the index

`fileindex.open` shows the current directory as `fileindex:///path/to/dir`,
with the same entry under the cursor. Navigation then works as usual: history,
bookmarks and the path picker accept `fileindex://` paths too. The index root is
`fileindex:///`. With Nerd Font icons on (`use_nerdfont_icons`), panel titles
and other path displays show `fileindex://` as the database icon followed by
`://`.

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
- **Edit**, **run** (Enter on an executable), **drop to shell**, **send paths
  to shell** and the **terminal panel** work on the real files and
  directories the entries name. Edits change the real file; the index picks
  the change up on the next scan. When an indexed directory is gone from
  disk, the shell and terminal panel start in its nearest existing parent.
  The panel does not follow the shell's directory back into the index view.
- **Find duplicates** compares the stored SHA-256 hashes in the index
  (metadata-free for JPEGs) and reads nothing from disk (see
  [find-duplicates.md](find-duplicates.md)).
- Git status, disk usage and folder colours are not available, as on
  [SFTP](sftp.md) panels.

## Tags

Tags are shown by the **fileindex tags** column. A panel with no meta columns
switches it on when it enters an index view and off again when it leaves
(`[fileindex] default_columns` in [config.toml](config.md) picks those
columns). Switch it on or off per panel
from the meta-column picker (`panel.meta`, see [Meta columns](meta-columns.md)).
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

While a scan or prune works on an entry, its panel row carries the database
icon (`[icons] fileindex`, the HDD job icon without Nerd Font icons). A scan
hashes all chosen entries together and clears each entry's icon as soon as
its files are committed.

The index does not yet follow renames, moves and deletes that pc makes on
disk; rescan or prune to bring it up to date.
