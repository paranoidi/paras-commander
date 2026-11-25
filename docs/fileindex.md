# fileindex

pc has [fileindex](https://github.com/paranoidi/fileindex) built in. fileindex is a
SQLite index of your files by content hash, with tags. In pc you can browse the
index like a directory tree, browse just the entries that carry chosen tags,
show tags as a panel column, add and remove tags, and run scans and prunes as
background jobs.

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
rebuilds the search indexes and counts the files still waiting to be hashed
below each directory (what keeps a directory `scanning` until they are
hashed), and can take minutes on a large index; the menu-bar spinner
turns while it runs, and other fileindex work waits until it is done. A database from a newer fileindex
version is reported in the Messages log, and fileindex actions say so instead
of running; it needs a newer pc.

## Actions

`<` opens the **File Index menu** at the bottom of the screen;
the `:` leader menu lists it under Tools with the same `<`. The menu groups
its actions into Browse, Tagging, Tools, Actions and Duplicates groups, with
Tools below Tagging in the same column. Each action's
letter in the menu is the letter of its own shortcut, in lowercase: every
fileindex shortcut is `Alt+Shift` plus that letter. The Duplicates group starts
with [Find duplicates](find-duplicates.md) under `F`, the
uppercase of Find in index's letter, and Browse by tags is `T`, the uppercase of
Tags' letter. All actions are also in Help (`F1`) under
fileindex. The keys below are the defaults; rebind the shortcuts under
`[main]` and the menu letters under `[fileindex_menu]` in `keybindings.toml`.

| Key | Menu | Action |
|---|---|---|
| `M-F` | `f` | [Find in index](#find-in-index) from the current directory, on a local or `fileindex://` panel |
| `M-I` | `i` | Toggle [browsing with fileindex](#browsing-with-fileindex): the real directory plus the index's state per entry |
| `M-B` | `b` | Toggle: show the current directory in the index, or from the index go to the real location; from a [tag view](#browsing-by-tags) it opens the plain index at the entry |
| `M-R` | `r` | Back to the plain filesystem: from the index or a tag view, go to the entry's real location; with browsing with fileindex on, turn it off |
| `M-G` | `T` | [Browse by tags](#browsing-by-tags): pick tags and show the entries that carry them, as a flat list or in their directory structure |
| `M-L` | `l` | Switch a [tag view](#browsing-by-tags) between the flat list and the directory structure (dimmed outside a tag view) |
| `M-W` | `F` | [Find duplicates within index](find-duplicates.md): duplicates from the index for the current directory, also from a normal panel: it never reads the disk, so it only sees indexed files (selected directories on a normal panel are ignored) |
| `M-O` | `o` | [Other copies](#other-copies) of the file under the cursor, from the index |
| `M-D` | `d` | [Copies outside directory](#copies-outside-directory): files under the current directory that also exist elsewhere in the index |
| `M-U` | `u` | [Find unique files](#find-unique-files): a view of the files under the current or selected directories with no copy outside them, to copy, move or delete |
| `M-S` | `s` | Scan the selected entries, else the current directory (the menu says which) |
| `M-A` | `S` | [Fast scan](#scan-and-prune) the selected entries, else the current directory: hash only a sample (first, middle and last 8 KB) of larger files |
| `M-P` | `p` | Remove what is gone from disk under the current directory |
| `M-E` | `e` | [Report differences](#report-differences) between the disk and the index under the directory under the cursor, else the current directory |
| `M-T` | `t` | [Edit tags](#tags) |

## Browsing with fileindex

`M-I` turns **Browse with fileindex** on or off for the active
panel. The panel stays a normal local panel (real paths, every file operation
works on the disk), and the index is merged into it:

- The git status column is replaced by one icon per row saying how the entry
  relates to the index. The column header and the panel title show the
  fileindex icon. In a git repository the git column is not shown while the
  mode is on.
- Entries the index recorded but that are gone from disk appear as extra
  **ghost rows** (icon `missing`), with their indexed size and modification
  time.

`M-R` also turns the mode off.

`[fileindex] start_browse_with` ([config.toml](config.md)) starts both panels
with the mode on.

The mode stays on while you navigate to other local directories and turns off
when the panel leaves local paths (SFTP; on an SFTP panel the action says
fileindex covers local paths only). On a `fileindex://` panel the action goes to
the real location with the mode on. Scans, prunes, tag edits and index
removals refresh the panel, and a running scan updates the rows of the directory
shown about once a second. Changes on disk show up as they happen too: the panel
is a local one, so [directory watching](panels.md#directory-watching) applies
(deleting an indexed file outside pc turns its row into a ghost row; creating it
again clears the ghost). A panel browsing the database directly (`fileindex://`) is
not watched.

| State | Legend | Meaning |
|---|---|---|
| `indexed` | indexed | In the index, the same size and modification time (files) |
| `unhashed` | not hashed yet | A file in the index with the same size and modification time that has not been hashed yet (a scan is still hashing, `hash` is off, or a fast scan hashed only its sample) |
| `changed` | changed | A file whose size or modification time differs from the index |
| `new` | not in index | On disk but not in the index |
| `missing` | only in index | Only in the index (ghost row) |
| `excluded` | excluded | Not in the index, and the scan rules in fileindex's `config.ini` (`exclude_dirs`, `exclude_files`, file patterns) skip it |
| `scanning` | scanning | A scan or prune task job is working on the row, or has walked it and a file in it is still waiting to be hashed |
| `checking` | checking | An indexed directory the background check has not finished with yet |
| `differs` | differs below | An indexed directory with something different below it: a new file a scan would add, a file whose size or modification time changed, or an indexed file gone from disk |
| `unchecked` | browser scan depth exceeded | An indexed directory with nothing different found in the folders the automatic check reached, but with deeper folders it did not look into |
| `tagged` | tagged | Indexed and carrying tags |

When several apply, the first of: scanning, missing, changed, excluded, new,
differs, checking, unhashed, tagged, unchecked, indexed (a file that is both unhashed
and tagged shows `unhashed`). A directory does not count unhashed files as a
difference. Directories are checked in the background
(flat layout only), one after the other, and each shows `checking` until its own
check finishes. After each
listing, every indexed directory is checked down to `browser_scan_max_depth`
levels (default 3). A difference within that depth shows as `differs`; if there
is none but the directory has folders deeper than that, it shows `unchecked`
(browser scan depth exceeded). In `config.toml`, `browser_scan_max_depth`
(label "Browser scan max depth") is `N` for N levels, `-1` for unlimited (the
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
theme (scanning and checking use `[status] scanning`); without Nerd Font icons a letter is shown instead. The fileindex menu shows a legend of these icons at the bottom.

On a ghost row:

- **Delete** removes it from the index only (the delete dialog says so). A
  selection mixing ghost rows and real entries deletes the real ones from disk
  and the ghost rows from the index; the index follows the real deletes too,
  because the operation started from a Browse-with panel (see
  [Operations and the index](#operations-and-the-index)).
- **Copy, move, view and edit** are refused with "Only in the index: name":
  there is no file to read. So are [user menu and Run for each commands](commands.md#local-panels-only)
  that would receive the ghost row (as `%f`, as a Run for each item, or as the
  directory they run in); a command that does not use it runs normally.

A symlink to a file is judged by what it points to: its target's size and
modification time are compared with the index, as a scan records them, and a
dangling link shows `missing` when the index recorded it (a dangling link the
index never saw gets no state). A link to a directory gets no state.

A difference can hide from the panel: new files under dot or git-ignored names are
not listed, and the cause can lie deeper than the automatic check looks. The
[differences report](#report-differences) lists every difference, hidden files
included.

`fileindex.status` shows the same words as the overlay in its column.

## Browsing the index

`M-B` shows the current directory as `fileindex:///path/to/dir`,
with the same entry under the cursor; on an index panel it does what
`M-R` does, so the one shortcut toggles. Navigation then works as usual: history,
bookmarks and the path picker accept `fileindex://` paths too. The index root is
`fileindex:///`. With Nerd Font icons on (`use_nerdfont_icons`), panel titles
and other path displays show `fileindex://` as the database icon followed by
a space.

To see only the entries that carry certain tags, use
[Browsing by tags](#browsing-by-tags).

An index view lists what the index recorded, including files that have since
gone from disk. Files show their indexed size and modification time;
directories show no modification time, and their size once it is requested,
as on a local panel (see Disk usage below). `M-R` goes back to the real directory
with the entry highlighted, or says the entry is no longer on disk.

In an index view:

- **View and copy** read the real file, so copying out of the index works when
  the file is still on disk.
- **Rename** and **move** inside the index record that the entry moved. They
  do not touch the disk. The rename dialog says so.
- **Delete** removes the entry, or a directory and everything under it, from
  the index only: files on disk stay. The delete dialog says so.
- **New directories, symlinks, copies into the index, flatten, unflatten, extracting
  into the index, and moves out of it** are refused: the index is read-only
  for new content, and a flatten there would reach only the files it lists.
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

## Browsing by tags

`M-G` (File Index menu `T`) shows only the entries that carry the tags you
pick, from any panel, as a **tag view** in the active panel. It opens the
**Browse by tags** dialog:

- **Tags:** every tag in the database as checkboxes, which work as in the
  [tag editor](#tags): Space, Enter or a click toggles, typing jumps to the
  best match. Check at least one tag (the dialog warns otherwise). With no tags
  in the database yet it says "No tags yet".
- **Match:** **All tags** keeps entries carrying every checked tag, **Any tag**
  entries carrying at least one.
- **View:** **Flat list** or **Directory structure** (below).

Opened inside a tag view, the dialog starts with that view's tags, Match and
View; elsewhere it starts with the Match and View you last used (All tags and
Flat list the first time). When nothing carries the tags the panel stays where
it is and a message says "No entries tagged beach & holiday".

An entry matches by its own tags only: a file by the tags the Tags column shows
for it (copies of one content all match), a directory by its own. Entries inside
a tagged directory do not match because the directory does. A view always covers
the whole index, wherever the panel was.

**Flat list.** One row per match, from every directory, sorted by path. A row
shows the entry's own name, so two matches called `notes.txt` in different
directories are two rows; the panel applies the `tag-list` [column
profile](columns.md#profiles) (Size, Tags and the **Directory** column, which
tells them apart). A tagged directory is one row: Enter opens it and shows its
whole content, the plain index. Going up from a row, or from anything opened
inside one, leads back up to the list (with the cursor on the row you came
from), not to the directory the row lives in.

**Directory structure.** The index's directory tree, with only the matching
entries and the directories that lead to them. It opens at the deepest
directory that holds every match. A tagged directory, and everything inside
one, shows its whole content.

`M-L` switches the view you are in between the two. From the flat list the
cursor row appears in its real directory; from the structure the list opens with
the cursor on that entry's row (when the entry is itself a match). `M-B` in a
tag view opens the plain index at the entry under the cursor (a second `M-B`
goes to the real directory, as in any index view), and `M-R` shows the entry at
its real location. Titles show the query: with Nerd Font icons the database icon,
then `(beach&holiday)` for a structure view or `@(beach|holiday)` for a flat
one, then the directory. A [panel tab](tabs.md) shows the icon and the tags
instead (`beach|holiday`, or `beach&holiday /summer` in a structure view).

In a tag view:

- **Copy** reads the real files, as in any index view. Copying a directory of a
  structure view that shows only part of its content (one that is neither tagged
  nor inside a tagged directory) copies only the matches, with their
  directories. Rows of a flat list that share a name meet the usual conflict
  handling.
- **Rename, move and delete** change the index only, as in any index view. A
  directory that a structure view shows only in part is refused ("Not tagged
  itself: name"), since the change would also reach the entries the view hides;
  so is the top of a view. Tagged directories and what is inside them, and the
  rows of a flat list, work as usual.
- **Disk usage** of such a partly shown directory counts the matches only.
- **Find in index** and **Jump to directory** search the whole index below the
  panel directory, not only the matches (from the flat list, the whole index),
  and open their result in the plain index.
- **Find duplicates** in a structure directory covers everything under it, not
  only the matches.
- On the flat list there is no real directory, so what would use the panel's
  directory is refused with "This is a list of tagged entries: select something
  first" unless it has something of its own: Scan and Fast scan need selected
  entries, Report differences the directory under the cursor, Find unique files
  selected directories, Edit tags a selection or the entry under the cursor.
  Prune, Copies outside directory and Find duplicates are always refused there.
  Inside a row, or in a structure view, the panel has a directory and they work
  as usual.

A tag view reloads when the index changes (a tag edit, a prune, a delete or
move in the index, the end of a scan), but not at every commit of a running
scan: during one it follows the panel's periodic refresh
([Panels](panels.md#staying-current)) and catches up when the scan ends.
Moving around inside a view is quick: the matches are looked up once and kept
while the database is unchanged.

**Paths.** A tag view is a `fileindex://` path with the query in parentheses
before the `://`, so history, bookmarks, tabs and the path picker keep it, and
you can type one:

| Path | Shows |
|---|---|
| `fileindex(beach&holiday):///home/me` | structure view, entries with **all** of the tags, at `/home/me` |
| `fileindex@(beach\|holiday):///` | flat list, entries with **any** of the tags (`@` marks flat) |
| `fileindex@(beach):///home/me/Trip//2019` | inside the flat row `/home/me/Trip`: the `//` marks where the row ends |

Use `&` or `|` in one query, never both. A tag name containing `%`, `/`, `:`,
`(`, `)`, `&`, `|` or a space is written with those characters as `%XX` (a space
is `%20`); the tags of a path are sorted, so one query has one path. The flat
form with no directory, `fileindex@(beach)://`, is accepted for the list.

## Finding in the index

Find (`C-f`) on an index view opens **Find in index**, which searches
the index under the panel directory instead of walking the disk (files that
have gone from disk are found too). [Jump to directory](navigation.md#jump-to-directory)
(`M-c`) on an index view searches the indexed directories the same way. `M-F` opens it from an ordinary local panel
too, so you can search the index without leaving the real directory: Enter
then shows the result in the real location (falling back to the index view when
the directory is gone from disk). The query runs as you type; results are capped at
`[ui.find].max_results` and the title shows `N+` when more matched. A finished query
without results turns the query red.

Two query modes (radios; `M-m` switches between them):

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
selection;
`F3` opens the real file in the viewer (closing it returns to the
dialog as it was) and `F4` in the external editor; `Shift+Left` and
`Shift+Right` show a result in a panel without closing; `Alt+Insert`
pins it. The key that opened the dialog hides it with its query and marks kept; the panel shows a
yellow search icon at its bottom right until the same key on that panel and
directory brings it back. Opening it anywhere else asks whether to restore the
parked search or start a new one there. OK without marks shows the result under the cursor in the panel: a
directory itself, a file in its directory with the file highlighted. The
marked summary counts files by their indexed size; directories add no size.
Find's advanced criteria are not available: the index keeps no times, owners
or modes to filter on.

## Other copies

`M-O` shows where else the file under the cursor exists according to the index, in a full-screen
view. It works on local and index panels, on files only.

**The view.** A list titled "Other copies of name (N)" is always on top (on the left when side by side), one
path per other copy. A path too long for the list is shortened; the one under the cursor then also shows
in full on the bottom border, across both panels. The other half shows a
real file panel on the directory of the copy under the cursor, with the file highlighted, so you can see
what else is there. Moving the cursor in the list takes the panel to that copy's directory; what you
do inside the panel (browse into a folder, move its cursor) stays until the list cursor moves again.
Started from an index view, the panel shows the index view of the directory, otherwise the real
directory (the index view of it when it is gone from disk).

The view opens stacked; [`[ui] other_copies_orientation`](config.md#ui) changes that to side by side, or
to follow the file panels, in which case `C-Space` in the view switches them too. When the terminal has too
few rows for stacked, it follows the file panels.

`Tab` switches between the list and the panel; `Shift+Tab` switches back. With the panel
focused it works like a normal file panel (select, rename, create folders, sort, quick filter, view,
edit). `Esc`, or `Left` in the list, closes the view and its panel and returns to the file panels, which
are as they were.

**Keys in the list.** Typing jumps to matching names. `Insert` or `Space` marks a copy and moves down;
`C-u` unmarks all. Enter opens the copy with the default application.

| Key | Action |
|---|---|
| `F5` | Copy to the other panel, through the usual copy dialog. It acts on the marked copies, else the copy under the cursor; the destination is the file panel that is not active |
| `F6` | Move to the other panel; the copies leave the list |
| `F8` | Delete the marked copies, else the copy under the cursor, after the usual confirmation; they leave the list |
| `F3` / `F4` | View or edit the copy |
| `S-Left` / `S-Right` | Show the copy in the left or right file panel, with the file highlighted; from the panel, its cursor entry |
| `C-Space` | Switch this view between stacked and side by side until it closes |
| `F9` / `F1` | The view's menu and its Help |

The mouse works as well: a click selects a row of the list or an entry of the panel and gives it
focus, and the wheel scrolls.

- Copies are files with the same content hash. JPEGs differing only in metadata
  count as copies, as in the Copies column below.
- A file that is only partly hashed (a [fast scan](#scan-and-prune) hashed its
  sample, or a scan has not reached it yet) has no known copies until it is fully hashed:
  the action says "Not fully hashed yet". A file the index does not
  know says "Not in the index", and one with no other copy "No other copies".
- An empty file has no copies to list, since all empty files have the same content: the action
  says "Empty file, copies not tracked".
- The answer is what the last scans recorded: the index may be stale, so a
  listed copy can be gone from disk (it then opens in the index view) and a
  new copy is missing until scanned.


## Copies outside directory

`M-D` answers "is it safe to delete these because they exist
elsewhere?". It opens the [Find duplicates](find-duplicates.md) results view for the active
panel's directory, listing only the files under it that have a copy somewhere else in the
index; each group shows the copies inside and outside the directory. Groups entirely inside
the directory are what plain Find duplicates is for, and groups entirely outside it are not
shown. The view title reads "Copies outside", the pane header shows the directory, and an empty result
says "No copies outside" the directory. It works on local and `fileindex://` panels; the index
root itself has nothing outside it.

- Deleting from this view works as in Find duplicates: mark copies, confirm, and the delete
  job deletes the files from disk and removes their index entries. Refresh, and the refresh after hashing potential
  duplicates, keep the view in this mode.
- Folder duplicates are not offered: only files with a copy elsewhere are listed, so a folder
  cannot be judged to match another as a whole.
- A match where a file is only partly hashed (a [fast scan](#scan-and-prune)) shows as a
  potential duplicate, as in Find duplicates.
- Copies are matched as in [Other copies](#other-copies); the answer is what the last scans
  recorded. A JPEG copy that differs only in metadata is listed with the group's smallest size.

## Find unique files

`M-U` answers "what would I lose if I deleted this?". It opens a full-screen view of the files with
no copy outside the directory, as a tree, for acting on them: mark what matters and copy it to a
backup. It works on local and `fileindex://` panels and reads only the index. What it checks depends
on the panel selection:

| Selection | Checked |
|---|---|
| None | The current directory |
| Directories | Those directories, as one scope: a copy in another selected directory is not outside, so the list is what deleting all of them would lose |
| Only files | The current directory; the selection is ignored and kept |
| Files and directories | Nothing: an error toast says to select one kind, and the view does not open |

The index root has nothing outside it, so it is refused. A selected directory the index does not
know is named in a warning when the list loads.

**The view.** The tree takes the active panel's place, with a file count and size on each row.
Directories start collapsed and a directory holding only one directory is shown as one row. The
other panel stays beside it: `Tab` focuses it, and it works like a normal file panel there (browse,
select, rename, create folders, sort, quick filter, view, edit) to choose where files go. `F5` and `F6`
from it still copy or move the tree's files into it; `F8` does nothing there, so a delete always
starts from the tree. It is the real panel, so it stays where you left it when you close the view. `Esc` or `M-U` closes
the view and keeps the results and marks; `M-U` on the same scope brings them back. A different scope
starts over. Refresh (`C-y`) reads the index again, keeping marks and open directories that
still apply.

**Keys in the tree.** `Right` and `Left` open and close directories, or step in and to the parent;
`M-up` and `M-down` jump between directories. `Insert` or `Space` marks a file, or every file in a
directory, including inside collapsed ones. Typing jumps to matching names. Enter opens a file with
the default application.
The panel's show-hidden-files key hides or shows files under dot-prefixed names (also in the view's
Actions menu); hiding one also drops its mark. They are shown at first, so a hidden unique file is not missed; hide
them to leave out directories like `.git`.

**Operations.** They act on the marked files, else the file under the cursor, else every file in the
directory under the cursor.

| Key | Action |
|---|---|
| `F5` | Copy to the other panel, through the usual copy dialog. Files from several directories can keep their layout below their common directory, or be flattened |
| `F6` | Move to the other panel; the files leave the list |
| `F8` | Delete, after a confirmation that warns the index knows no other copy; the files leave the list. If that would leave directories empty, a second question lists them and offers to remove them too, parents included at any depth, up to and including the directories checked and never above them (Yes is the default; Cancel deletes nothing and keeps the marks) |
| `M-H` | Fully hash what the flagged files need, in a background job; the list refreshes when it ends. Greyed out in the menus when every listed file is settled |
| `F3` / `F4` | View (leader `v`) or edit the file |
| `S-Left` / `S-Right` | Show the row in the left or right panel |
| `Alt+Insert` | Pin the row |

A copy does not make a file drop off the list by itself: the index records new files only when it
scans them. Scan the destination, then refresh, and files now copied elsewhere go away. That also
works as a check that the backup holds what you copied.

**What the list means.**

- A file is hidden only when a fully hashed copy outside is confirmed. Copies are matched as in
  [Other copies](#other-copies), so a JPEG copy differing only in metadata counts as a copy.
- Files whose copies cannot be ruled out are still listed, with the "not hashed yet" icon, and the
  title counts them (`12, 3 not fully hashed`). That covers files not fully hashed, and files with
  a [potential duplicate](find-duplicates.md) outside. `M-H` settles them.
- Empty files are not listed: they have no content to lose, and their copies are not tracked.
- Copies inside the scope do not count, so all copies of a file that exists only inside it are listed.
- The answer is what the last scans recorded. A copy deleted from disk still counts until it is
  [pruned](#scan-and-prune), and a copy never scanned is unknown, so scan and prune the places you
  rely on before deleting anything based on this list.

## Columns

The index adds columns to the [Columns dialog](columns.md), in its **File Index** section, listed only
while the index database exists and opens. They work on local panels and in
index views, and sort and filter like any meta column:

| Column | Name | Shows |
|---|---|---|
| **Status** | `fileindex.status` | how the entry compares with the index: `indexed` (a file with the same size and modification time as when scanned, or a known directory), `changed` (a file whose size or modification time differs), `unhashed` (in the index but not hashed yet), `new` (not in the index), `excluded` (not in the index and skipped by the scan rules) or `missing` (in an index view, gone from disk) |
| **Tags** | `fileindex.tags` | a file's or a directory's tags, comma separated |
| **Copies** | `fileindex.copies` | how many indexed files have the same content, this one included; JPEGs compare their metadata-free hash, as [find duplicates](find-duplicates.md) does. An empty file shows `-`: all empty files have the same content, so the count would mean nothing. Counts above 1 use the warn [alert color](meta-columns.md#alert-colors), counts above 2 the critical one |
| **First seen** | `fileindex.first-seen` | when the file or directory was first indexed at this path, in local time |
| **Last seen** | `fileindex.last-seen` | when a scan last saw the file or directory, in local time |
| **Tree files** | `fileindex.tree-files` | for a directory, how many indexed files are under it |
| **Hash** | `fileindex.hash` | the first 12 hex digits of the file's SHA-256 |
| **Directory** | `fileindex.directory` | the directory the entry is in, with `~` for home; shortened to the column width with the last names kept (`~/P…/Holiday/Trip`). It needs no index data, so it works on local panels too |

Tree files adds up the whole subtree, which at the index root is the whole
index. A directory's indexed size shows in the size column of an index view
(see Browsing the index).

A panel with no meta columns applies the [column profile](columns.md#profiles)
named by `[fileindex] default_profile` ([config.toml](config.md); by default
`index`, which is Size and the tags column) when it enters an index view (a
`fileindex://` panel or Browse with fileindex). In a flat [tag view](#browsing-by-tags)
it applies the `tag-list` profile (Size, Tags and Directory) instead, and
moving between a flat tag view and another index view switches between the two.
When it leaves the index views, its previous columns come back, unless you
changed the columns in between. A `default_profile` of `""` applies none of
them.

## Tags

`M-T` opens the tag editor for the selection, else the entry under
the cursor, else the current directory. It lists every tag in the database as
checkboxes in two columns, with the tags the entries already carry checked. A
tag that only some of the selected entries carry shows as mixed (`[-]`).

Nothing is written until OK; Cancel or Esc discards the edits. To see what carries
a tag, use [Browse by tags](#browsing-by-tags).

- **Checkboxes:** the list comes first and has focus when the dialog opens.
  Space, Enter or a click toggles a tag. A mixed tag cycles through set on all,
  removed from all, and mixed, which leaves each entry as it was.
- **Leaving the list:** Tab, or Down on the last row.
- **Jump:** typing in the list jumps to the best fuzzy match, and the typed
  text, up to 14 characters, shows as `> text` at the left of the dialog's top
  border. It is yellow, or red when no tag matches. With several matches, Up and Down
  cycle through them. Backspace trims the text and Esc clears it. Toggling a tag
  clears it too, so you can type the next search straight away.
- **New tags:** the input below the list. Type one or more tags separated by
  commas or spaces and press Enter. They appear checked in the list and the
  dialog stays open for more. OK also takes whatever is still in the input. With
  no tags in the database yet, the dialog opens here.
- **Recursive** shows only when the entries include a directory. It applies the
  edits to a directory, every directory under it and every file in them, instead
  of the directory alone. Only the tags you changed are applied, so a tag the
  directory already has does not spread down the tree until you remove it, then
  add it again with Recursive on.

Tags work on entries that are gone from disk too. Adding tags to a path the
index does not know queues a tag scan of it (a task job, see below) that applies
the tags once the walk has recorded the path; a path the scan rules skip stays
untagged and is listed as a warning on the job. Removing tags only counts such
paths in the message after OK.

A tag scan never reads whole files, whatever `hash` says: like a
[fast scan](#scan-and-prune) it reads only a sample of each larger file (its
first, middle and last 8 KB), so tagging a directory of large videos as
watched takes seconds. A file's tag belongs to its content once a plain scan
has fully hashed it, and then follows the file wherever it goes. Until then the
sample stands in: if the file is moved, renamed or deleted first, the tag is kept
by its size and sample, and returns when a scan with hashing or a fast scan finds a
file with the same size and sample. Adding tags to indexed files that have
neither a full hash nor a sample queues the same tag scan for them, so their tags
are kept the same way. A file still being written, or cut short, has another
sample than the finished file, so it does not take the tag. The tags column updates when the edit
finishes. A tag no entry uses any more stays in the list.

A directory has no content of its own, so its tags are kept by its files. When a
tagged directory leaves the index (a prune finds it gone, or it is deleted or
moved across file systems in pc), the index keeps a print of its 64 largest files.
A later scan or prune that finds a directory holding the same files at another
path, hashed or sampled, gives it the tags back, and the job lists each such
directory as "Restored tag". A tagged directory renamed outside pc and scanned
only at its new path gets its tags there at once ("Copied tag"); the old path keeps
them until a prune. Every matching directory gets the tags, so copies share them,
as with files. To match, the files must cover at least 90% of the bytes on both
sides, at least half of the bytes must match by content rather than by name and
size, and the directory must hold at least 1 MiB or three matching files, so a
directory of one small common file never takes a tag from another. Tagging a
directory queues the tag scan for its files when any of them has neither a full
hash nor a sample, recursive or not, so its print can be matched. A directory with
no files keeps no print, and its tags go with it. Prints of directories that never
return are kept until `fileindex tag-gc`.

## Scan and prune

A user menu command can queue a scan by itself once it has run: see
[Updating the file index](commands.md#updating-the-file-index).

Queuing a scan clears the panel's selection.

`M-S` works in three stages on the chosen paths. First it walks
them and records new and changed files by name: they are searchable (Find in
index, the `fileindex://` view) as soon as each batch is saved, before any
hashing, and show the `unhashed` state. Then it removes from the index
whatever under them is gone from disk. Last it hashes the files that need it,
including unhashed ones left by an earlier scan. With `hash = false` under
`[fileindex]` the scan stops after the removal, so files stay unhashed (no hash
or copies column, left out of duplicate detection) until a scan with hashing
runs.
`M-A` is the same scan with its last stage reduced: it hashes
only a sample of files larger than 24 KB (their first, middle and last 8 KB), so a
big tree is done in a fraction of the time, even with `hash = false`. Those files keep the `unhashed`
state, count only as potential duplicates in [Find duplicates](find-duplicates.md)
and stay out of the hash and copies columns until a plain scan hashes
them fully; running a fast scan again skips them.
`M-P` only does the removal, for the current directory. In an
index view all of them act on the real paths.

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
of the row. A scan hashes all chosen entries together. A file keeps the icon
until it is hashed and a directory until every file below it is hashed, so a
directory clears once its own files are done, even while the scan goes on with
others. A file that is new or changed on disk keeps it as well, until the scan has
recorded it. A scan that does not hash everything (`hash = false` under
`[fileindex]`, a fast scan, a scan that tags) leaves some files unhashed: their
rows keep the icon until the job ends and then show `unhashed`. A directory the
scan is not working on keeps its usual state. Cancelling the scan returns the
rows to what the database holds.

## Operations and the index

The index follows the renames, moves and deletes that pc makes on disk, from
any panel, undo included. An index view (`fileindex://`) is the other way round:
its operations change the index and never the disk. Changes made outside pc wait
for a scan; a deleted file then shows as a ghost row until a prune.

Following moves or removes records that already exist. It never adds any: new
files, copies and extracted files wait for the next scan. A file moved by copy and delete (across
file systems) has no record until a scan of the destination, which brings its tags back.
Panels showing the index refresh when the records change.

The [Find duplicates](find-duplicates.md) and Copies outside views are the
exception: deleting there removes the file from disk and its record from the
index, whichever panel the view was opened from. A file that is already gone
from disk only loses its record.

## Report differences

`M-E` compares the disk with the index under a
directory and lists every difference: the directory under the cursor, else the
current directory, on a local or `fileindex://` panel. It runs as a task job in
the [jobs view](jobs.md), with a progress count and cancel, beside a running scan
(it only reads). The "fileindex differences queued" message shows only if the
report has no result after 2 seconds. When it ends within 2 seconds of the
request and you are still looking at the file list of the same panel with no
dialog open, a picker titled "Differences under NAME (N)" opens by itself;
otherwise a message says "N differences under NAME: see Jobs view". Either way
the result stays on the job: select it in the jobs view and use **Jobs → Show
results** to open the picker again. The picker lists the
paths, filtered as you type, each with its state:

| State | Meaning |
|---|---|
| `new` | on disk, included by the scan rules, not in the index |
| `changed` | in the index with another size or modification time |
| `missing` | in the index, gone from disk |

Enter shows the path in the active panel, in its directory with the file
highlighted (the real directory, or the index view when the report was started
there). The report looks at the whole subtree, hidden files included, so it explains
a `differs` icon the panel does not: it uses the same check as the icon. A record
the scan rules would skip but that is still on disk (below an excluded directory,
or in a directory pc cannot read) is not a difference. "No differences"
and "NAME is not in the index" are shown as messages instead of a picker, and
always show, whatever you are doing when the report ends. A picker opened from
the jobs view returns there on Esc; choosing a path switches to the file list.

The picker's list works like Find's, with the same keys (`keybindings.toml`,
`[dialog.find]` and `[main]`):

| Key | Effect |
|---|---|
| `Insert` | Mark the path and move down; OK then adds the marked paths to the active panel's selection instead of showing the cursor path |
| `Alt+Insert` | Pin or unpin the path |
| `Shift+Left` / `Shift+Right` | Show the path in the left/top / right/bottom panel, keeping the picker open |
| `F3` | View the file; closing the viewer returns to the picker |
| `F4` | Edit the file, keeping the picker open |

View and edit refuse a file that is missing from disk.

