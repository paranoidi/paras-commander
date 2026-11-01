# fileindex

pc has [fileindex](https://github.com/paranoidi/fileindex) built in. fileindex is a
SQLite index of your files by content hash, with tags. In pc you can browse the
index like a directory tree, show tags as a panel column, add and remove tags,
and run scans and prunes as background jobs.

pc shares its index with the `fileindex` command. Set the database location
with `database` under `[fileindex]` in pc's `config.toml`; left empty, pc uses
the path from fileindex's `~/.config/fileindex/config.ini`. Which files a scan
includes always comes from that `config.ini`, so the command and pc index the
same files. `workers` sets the number of hash workers and
`scan_commit_interval_ms` how often a scan saves its progress to the database
and `hash` whether a scan hashes the files it indexes (see [Config](config.md#fileindex)).

When there is no database at that location, the first fileindex action (or
the tags column, or a `fileindex://` path) asks whether to create an empty one.
**Yes** creates it and carries on with the action; **No** leaves everything as
it was, and the next action asks again.

A database from an older fileindex version gets the same kind of question:
**Yes** upgrades it (what `fileindex init` does) and carries on. Upgrading
rebuilds the search indexes and can take minutes on a large index; the menu-bar spinner
turns while it runs, and other fileindex work waits until it is done. A database from a newer fileindex
version is reported in the Messages log, and fileindex actions say so instead
of running; it needs a newer pc.

## Actions

`<` (`fileindex.menu`) opens the **File Index menu** at the bottom of the screen;
the `:` leader menu lists it under Tools with the same `<`. The menu groups
its actions into Browse, Tagging and Actions columns. Each action's
letter in the menu is the letter of its own shortcut, in lowercase: every
fileindex shortcut is `Alt+Shift` plus that letter. All actions are also in Help (`F1`) under
fileindex. The keys below are the defaults; rebind the shortcuts under
`[main]` and the menu letters under `[fileindex_menu]` in `keybindings.toml`.

| Key | Menu | Action | |
|---|---|---|---|
| `M-I` | `i` | `fileindex.browse-with` | Toggle [browsing with fileindex](#browsing-with-fileindex): the real directory plus the index's state per entry |
| `M-B` | `b` | `fileindex.browse` | Toggle: show the current directory in the index, or from the index go to the real location |
| `M-R` | `r` | `fileindex.reveal` | From the index, go to the entry's real location |
| `M-S` | `s` | `fileindex.scan` | Scan the selected entries, else the current directory (the menu says which) |
| `M-P` | `p` | `fileindex.prune` | Remove what is gone from disk under the current directory |
| `M-T` | `t` | `fileindex.tag-add` | Add tags |
| `M-U` | `u` | `fileindex.tag-remove` | Remove tags |

## Browsing with fileindex

`fileindex.browse-with` turns **Browse with fileindex** on or off for the active
panel. The panel stays a normal local panel (real paths, every file operation
works on the disk), and the index is merged into it:

- The git status column is replaced by one icon per row saying how the entry
  relates to the index. The column header and the panel title show the
  fileindex icon. In a git repository the git column is not shown while the
  mode is on.
- Entries the index recorded but that are gone from disk appear as extra
  **ghost rows** (icon `missing`), with their indexed size and modification
  time.

The mode stays on while you navigate to other local directories and turns off
when the panel leaves local paths (SFTP; on an SFTP panel the action says
fileindex covers local paths only). On a `fileindex://` panel the action goes to
the real location with the mode on. Scans, prunes, tag edits and index
removals refresh the panel.

| State | Legend | Meaning |
|---|---|---|
| `indexed` | indexed | In the index, the same size and modification time (files) |
| `unhashed` | not hashed yet | A file in the index with the same size and modification time that has not been hashed yet (a scan is still hashing, or `hash` is off) |
| `changed` | changed | A file whose size or modification time differs from the index |
| `new` | not in index | On disk but not in the index |
| `missing` | only in index | Only in the index (ghost row) |
| `excluded` | excluded | Not in the index, and the scan rules in fileindex's `config.ini` (`exclude_dirs`, `exclude_files`, file patterns) skip it |
| `scanning` | scanning | A scan or prune task job is working on the row |
| `differs` | differs below | An indexed directory with something different below it: a new, changed (size or modification time) or missing file |
| `unchecked` | scan depth exceeded | An indexed directory with nothing different found in the folders the automatic check reached, but with deeper folders it did not look into |
| `tagged` | tagged | Indexed and carrying tags |

When several apply, the first of: scanning, missing, changed, excluded, new,
differs, unhashed, tagged, unchecked, indexed (a file that is both unhashed
and tagged shows `unhashed`). A directory does not count unhashed files as a
difference. Directories are checked in the background
(flat layout only; sizes are compared, not modification times). After each
listing, every indexed directory is checked down to `folder_scan_max_depth`
levels (default 3). A difference within that depth shows as `differs`; if there
is none but the directory has folders deeper than that, it shows `unchecked`
(scan depth exceeded). In `config.toml`, `folder_scan_max_depth`
(label "Folder scan max depth") is `N` for N levels, `-1` for unlimited (the
automatic check then never shows `unchecked`) and `0` to turn the automatic
check off; see [config](config.md#fileindex).

Asking for directory sizes checks without a depth limit: the directory-size
action on the cursor directory, the whole-panel disk usage scan, and selecting
directories (each selected directory once per listing). The result replaces
`unchecked` or `indexed` with `differs`, or turns `unchecked` into `indexed`
when nothing differs, so a difference below the automatic depth shows only
after such a request. A tagged directory keeps `tagged` instead of showing
`unchecked`. The glyphs are under
`[icons.fileindex_state]` and the colors under `[panel.fileindex]` in the
theme (scanning uses `[status] scanning`); without Nerd Font icons a letter is shown instead. The fileindex menu
(`fileindex.menu`) shows a legend of these icons at the bottom.

On a ghost row:

- **Delete** removes it from the index only (the delete dialog says so). A
  selection mixing ghost rows and real entries deletes the real ones from disk
  and the ghost rows from the index.
- **Copy, move, view and edit** are refused with "Only in the index: name":
  there is no file to read.

`fileindex.status` shows the same words as the overlay in its column.

## Browsing the index

`fileindex.browse` shows the current directory as `fileindex:///path/to/dir`,
with the same entry under the cursor; on an index panel it does what
`fileindex.reveal` does, so the one shortcut toggles. Navigation then works as usual: history,
bookmarks and the path picker accept `fileindex://` paths too. The index root is
`fileindex:///`. With Nerd Font icons on (`use_nerdfont_icons`), panel titles
and other path displays show `fileindex://` as the database icon followed by
a space.

An index view lists what the index recorded, including files that have since
gone from disk. Files show their indexed size and modification time;
directories show no modification time, and their size once it is requested,
as on a local panel (see Disk usage below). `fileindex.reveal` goes back to the real directory
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
- **Disk usage** works as on a local panel ([disk-usage.md](disk-usage.md)):
  the directory scan, sizing the directory under the cursor, selected
  directories, expanded tree directories and the delete summary all fill the
  size column. The totals are the indexed sizes of the files under each
  directory, read from the index without touching the disk, so they include
  files gone from disk and match the file rows. A scan, prune, move or delete
  in the index drops the totals; request them again to see the new ones.
- Git status and folder colours are not available, as on [SFTP](sftp.md)
  panels.

## Finding in the index

`panel.find-dialog` on an index view opens **Find in index**, which searches
the index under the panel directory instead of walking the disk (files that
have gone from disk are found too). `panel.jump-dialog` opens the same dialog
for directories only. The query runs as you type; results are capped at
`[ui.find].max_results` and the title shows `N+` when more matched. A finished query
without results turns the query red.

Two query modes (radios; `find.cycle-match-mode` switches between them):

- **Pattern** matches file and directory names, ignoring case. Plain text finds
  names containing it (`holiday`). With `*` (any run) or `?` (one character)
  the whole name must match the pattern (`*.jpg`, `report-??.pdf`).
- **Boolean** matches file names word by word (files only; Only directories is
  off). Words are whole words or prefixes with `*` (`holi*`), `"quoted
  phrases"`, and `AND`, `OR`, `NOT` combine them (`beach AND NOT 2019`). A
  syntax error is shown in the dialog.

Other options: **Only directories** / **Only files**, **Include hidden**
(names with a dot-prefixed path component are left out by default), the depth
box (1-9 levels below the panel directory, empty or 0 for unlimited), and
**Search only from selected directories** when directories are selected in the
panel.

Keys work as in Find: mark results and press OK to add them to the panel
selection (`find.select-all`, `find.unselect-all`, `find.select-group`,
`find.unselect-group`, `find.select-parent-dirs`, `panel.select-toggle`);
`find.view` opens the real file in the viewer; `ui.open-primary` and
`ui.open-secondary` show a result in a panel without closing; `panel.pin-toggle`
pins it. OK without marks shows the result under the cursor in the panel: a
directory itself, a file in its directory with the file highlighted. The
marked summary counts files by their indexed size; directories add no size.
Find's advanced criteria are not available: the index keeps no times, owners
or modes to filter on.

## Columns

The index adds columns to the meta-column picker (`panel.meta`, see
[Meta columns](meta-columns.md)), in its **File Index** section, listed only
while the index database exists and opens. They work on local panels and in
index views, and sort and filter like any meta column:

| Column | Name | Shows |
|---|---|---|
| **Status** | `fileindex.status` | how the entry compares with the index: `indexed` (a file with the same size and modification time as when scanned, or a known directory), `changed` (a file whose size or modification time differs), `unhashed` (in the index but not hashed yet), `new` (not in the index), `excluded` (not in the index and skipped by the scan rules) or `missing` (in an index view, gone from disk) |
| **Tags** | `fileindex.tags` | a file's or a directory's tags, comma separated |
| **Copies** | `fileindex.copies` | how many indexed files have the same content, this one included; JPEGs compare their metadata-free hash, as [find duplicates](find-duplicates.md) does |
| **First seen** | `fileindex.first-seen` | when the file or directory was first indexed at this path, in local time |
| **Last seen** | `fileindex.last-seen` | when a scan last saw the file or directory, in local time |
| **Tree files** | `fileindex.tree-files` | for a directory, how many indexed files are under it |
| **Hash** | `fileindex.hash` | the first 12 hex digits of the file's SHA-256 |

Tree files adds up the whole subtree, which at the index root is the whole
index. A directory's indexed size shows in the size column of an index view
(see Browsing the index).

A panel with no meta columns switches on the columns in
`[fileindex] default_columns` ([config.toml](config.md); the tags column by
default) when it enters an index view, and switches them off again when it
leaves.

## Tags

`fileindex.tag-add` and `fileindex.tag-remove` open a prompt for the
selection, else the entry under the cursor, else the current directory. Type
one or more tags separated by commas or spaces. **Recursive** applies the tags
to a directory, every directory under it and every file in them, instead of
the directory alone. Tags work on entries that are gone from disk too, and the
message after OK counts paths the index does not know. The tags column updates
when the edit finishes.

## Scan and prune

`fileindex.scan` works in three stages on the chosen paths. First it walks
them and records new and changed files by name: they are searchable (Find in
index, the `fileindex://` view) as soon as each batch is saved, before any
hashing, and show the `unhashed` state. Then it removes from the index
whatever under them is gone from disk. Last it hashes the files that need it,
including unhashed ones left by an earlier scan. With `hash = false` under
`[fileindex]` the scan stops after the removal, so files stay unhashed (no hash
or copies column, left out of duplicate detection) until a scan with hashing
runs.
`fileindex.prune` only does the removal, for the current directory. In an
index view both act on the real paths.

Both run as **task** jobs in the [jobs view](jobs.md): the Details panel shows
the files done, bytes hashed, current path and, while a scan hashes, how many
hash workers it runs. The Details panel lists the steps (walk filesystem, pruning, hash new files) with
a status icon each, so the running one is visible. The stage also shows in the progress: while walking, the
files done is the count saved so far with an unknown total (`?`) and the
current path is the folder being walked; while removing, the total is still
unknown and the path is the folder being checked; while hashing, it is
`done / files to hash` with the workers and the file being hashed; per-file errors and pruning removals are listed as
warnings. With `parallel_workers = auto` the worker count grows while more
workers make hashing faster. A scan saves what it hashed every
`scan_commit_interval_ms`. Cancel stops a scan between files and keeps
everything already committed. One fileindex job runs at a time; another
waits in the queue. Task jobs cannot be paused or retried; run the action
again instead.

While a scan or prune works on an entry, its panel row shows the `scanning`
state icon: in the state column while browsing with fileindex, else at the end
of the row. A scan hashes all chosen entries together and clears each entry's
icon as soon as its files are committed.

The index does not yet follow renames, moves and deletes that pc makes on
disk; rescan or prune to bring it up to date.
