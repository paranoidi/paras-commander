# Paras Commander

A Midnight Commander–inspired twin-panel file manager for the terminal. This glossary fixes the words used for its concepts in code, docs, issues and tests.

## Language

### Panels

**Primary panel / Secondary panel**:
The two file-browser panels, named by identity rather than position; swapping or stacking the layout never changes which is which.
_Avoid_: left panel, right panel (those name the visual slot only)

**Active panel / Inactive panel**:
The panel holding keyboard focus, and the other one.
_Avoid_: current panel, other panel

**Selection**:
The set of entries a panel has marked, kept by full path so it survives leaving the directory.
_Avoid_: marks, tags

**Selected strip**:
The sub-pane listing selected entries that lie outside the panel's current directory.
_Avoid_: selection panel, basket

**Stash**:
A single per-panel slot where a selection can be parked and later restored.
_Avoid_: clipboard, saved selection

**Listing format**:
Which columns a panel shows beside each name (size, mtime, permissions, meta columns).

**Meta column**:
An extra listing column whose values come from a user-configured per-entry command.
_Avoid_: custom column

**Folder colour**:
The Nemo-compatible tint of a local directory's name and icon, read from the GVfs metadata database into an in-memory snapshot (`internal/foldercolor`).

**Tree layout**:
A panel mode where directories expand and collapse inline within one listing.

**Carousel**:
A panel mode showing parent, current and child columns side by side.
_Avoid_: miller columns, three-pane view

**Zoom**:
Giving the active panel a larger share of the split.
_Avoid_: maximize

**Entry filter**:
A persistent rule (git status or name pattern) that narrows which entries a panel shows; at most one per panel.
_Avoid_: quick filter

**Quick filter**:
Transient type-to-jump narrowing of the file list while typing.
_Avoid_: search, entry filter

**Disk usage scan**:
A background measurement of directory sizes, shown as size bars and used for size sorting.

**Filesystem change report**:
What an operation tells the disk-usage cache it did to a local path: renamed, removed, created, or dirty (changed by an unknown amount, so rescanned). Reported where the change happens, in `internal/ops`, never by the dialogs or jobs that call it.
_Avoid_: fs event, notification (those suggest a filesystem watcher, which this is not)

### Navigation

**History**:
Each panel's own record of visited directories.

**Bookmark**:
A saved, named directory shared with fzf-marks and GTK bookmarks.
_Avoid_: favorite

**Pin list**:
A session-only list of pinned paths, separate from history and bookmarks.
_Avoid_: favorites

**Path picker**:
The fuzzy dialog for choosing a path from history, bookmarks and pins, reachable from any path input.

**Latched sync**:
A mode where the inactive panel follows the directory highlighted in the active panel until toggled off.
_Avoid_: linked panels, mirror

**Driver / Follower**:
In latched sync and quick view, the panel whose cursor leads and the panel that reacts to it.
_Avoid_: master/slave, source/target panel

**Find**:
Recursive name search under a panel's directory, backed by a background index.

**Remote panel**:
A panel browsing an `sftp://` location.

### Preview

**Quick view**:
The inactive panel shows a live preview of whatever the active panel's cursor is on.
_Avoid_: preview pane

**Fullscreen preview**:
A full-screen view of one file's preview (`file.view`).
_Avoid_: viewer

**Directory overlay**:
A temporary listing of a highlighted directory painted in the inactive panel without moving that panel.

**Preview command**:
A user-configured rule that replaces the built-in preview for matching files.
_Avoid_: preview plugin

**Passthrough**:
Wrapping terminal output so tmux forwards it untouched to the outer terminal.

**Outer terminal**:
The real terminal emulator running underneath tmux.
_Avoid_: host terminal

### Jobs

**Job**:
A queued background file operation (copy, move, delete, flatten, extract), or a task job.
_Avoid_: task (for a transfer), transfer (as a general term)

**Task job**:
A job that runs a built-in feature's own function (`jobs.TypeTask`, e.g. a fileindex scan) instead of a file transfer: progress, cancel and failure like any job, but no pre-scan, conflicts, undo, retry or pause; one task per lease name runs at a time.
_Avoid_: background task, worker job

**Job source**:
One top-level path a job was queued with (one per selected entry); a job with several expands into one Queue sub-row per source.
_Avoid_: item (an item is any file the job processes), selection (the panel's set)

**Blocker**:
A decision a running job needs from the user, such as a file conflict or a lack of disk space.
_Avoid_: prompt, conflict dialog

**Conflict rules**:
The per-condition answers (time, size and, with Verify, content Same / Differ) a job applies to file conflicts instead of asking.
_Avoid_: advanced options, match size

**Verify**:
Comparing the contents of a conflicting source and destination before the conflict rules decide; the same pairwise check as panel compare.
_Avoid_: skip identical, checksum

**Delete source**:
A move's conflict answer that keeps the existing destination and removes the file being moved.
_Avoid_: drop source, discard

**Plan**:
The enumeration of a job's sources, which runs alongside the transfer itself.
_Avoid_: index (reserved for Find)

**Dangling directory**:
A directory left empty after a move or delete and offered for removal.

**Flatten**:
Moving the contents of nested directories into a single destination directory.

**Mass rename**:
Renaming a whole selection at once by pattern, regex, external editor or capitalization.

**Self-copy**:
A copy whose destination is its own source, which asks for a new name.

### Commands

**Command**:
A user-launched external process (from the user menu, run-for-each or executing a file) tracked in the Commands view.
_Avoid_: job

**Run-for-each**:
Running one command once per selected entry.
_Avoid_: batch command

**Work pool**:
A named limit on how many commands may run at once.

**User menu**:
The user's own nested menu of commands (`app.user-menu`).
_Avoid_: custom commands

**Subshell**:
The persistent shell behind the panels, toggled with `app.drop-to-shell`, whose working directory stays in sync with the active panel.

**Terminal panel**:
A strip above the footer showing the subshell (or a command's terminal) while the panels stay visible.

### Views and input

**Browser**:
The normal twin-panel view.

**Auxiliary view**:
Any full-screen view other than the browser: Jobs, Commands, Messages, Compare, Dedup.
_Avoid_: screen, overlay view

**Compare**:
A view diffing the contents of the two panels' directory trees.

**Dedup**:
A view finding duplicate files under one directory.
_Avoid_: duplicate finder

**Leader menu**:
The bottom strip of lettered entries opened by `app.leader-menu`; each letter fires an action directly.
_Avoid_: function menu

**Menu bar**:
The top row of pulldown menus (`app.open-menu`).

**Overlay**:
A view- or dialog-specific shortcut table that takes precedence over the global one while that view is focused.
_Avoid_: context map

**Vi-motion mode**:
A mode where h/j/k/l move like arrow keys and leader letters fire directly.

**Toast**:
A short-lived status message shown above the footer; the Messages view keeps their history.
_Avoid_: notification, banner

**Blocked chrome**:
The dimmed look of the panels while a dialog or menu covers them.
