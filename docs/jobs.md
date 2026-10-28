# Jobs

Copy, move, flatten, extract and delete operations run as background jobs, so
the file manager stays usable while they work. The menu bar shows a compact
summary: an icon with a count for each job state, a progress bar for the
running job and, to its left, a pill with the current transfer speed while a
transfer runs. A job waiting for your answer is shown as "N job(s) waiting".

Open the jobs view with `Alt+J` (`jobs.open`, **Display -> Jobs**, Esc menu
`j`). It has a Queue panel with Type, Status, ETA, Speed and Progress, plus
Details and Operations panels for the selected job. When a job is
waiting for an answer, opening the view selects it and focuses the conflict
panel. Running and queued jobs are listed first in run order (so the queue
move keys match the screen), followed by finished jobs with the most recent
first. Keys are rebindable under `[jobs]` in `keybindings.toml`.

Operations spans the full width at the bottom of the screen, so long paths
fit; the Queue and Details share the rows above it, as tall as Details needs.
`Ctrl+Space` (`jobs.toggle-operations-layout`, **Jobs -> Operations layout**,
Esc menu `o`) moves Operations under Details in the right column, giving the
Queue the full height; press it again to move it back. The toggle lasts for
the session; set `operations_layout = "side"` under `[jobs]` in `config.toml`
to start with Operations in the right column. A terminal too short for the
bottom layout uses the side layout.

A job queued from several selected entries shows an expander icon after its
type. `Alt+Right` (`jobs.expand`) lists each of its sources in a tree under
the job, each with its own status icon (queued, running, completed, failed,
canceled) and done / failed / skipped counts; `Alt+Left` (`jobs.collapse`)
folds them away. With the cursor on a source, the Operations panel shows only
that source's rows and names the source on its top border. The source being
worked on is drawn in the job's status color and followed by the file it is
on.

While such a job runs, the file panels mark only the item it is working on
with the job icon, on both the source and the destination side. Items still to
come show the queued icon on their source, and finished items show no job
icon.

A copy or move starts with the size the panel already shows for the selection,
so its percentage and ETA work immediately. The Details panel marks such a
total `(estimated)` until the exact size is counted (see
[Background scan throttling](background-scan-throttle.md#estimated-totals)).

| Key | Action |
|---|---|
| `Ctrl+C` | Cancel the selected job (`jobs.cancel`) |
| `Ctrl+P` | Pause a queued job (`jobs.pause`) |
| `Ctrl+R` | Resume a paused job or retry a failed or canceled one (`jobs.resume`) |
| `Ctrl+Up` / `Ctrl+Down` | Move the job in the queue (`jobs.queue-up`, `jobs.queue-down`) |
| `F8` | Clear finished jobs (`jobs.clear-finished`) |
| `Alt+Right` / `Alt+Left` | Expand / collapse a multi-source job into its sources (`jobs.expand`, `jobs.collapse`) |
| `F3` / `Shift+F3` | Filter the Operations panel by result / reset its filters (`jobs.result-filter`, `jobs.reset-filter`) |
| `+` / `-` / `Ctrl+U` | Raise, lower or clear the transfer rate limit (Queue panel focused) |
| `Tab` | Cycle the panels |
| `Ctrl+Space` | Move the Operations panel between the right column and full width at the bottom (`jobs.toggle-operations-layout`) |
| `Enter` | Maximize the focused Operations panel with wrapped lines (`jobs.maximize-pane`); `Esc`, `Enter` or `Left` restore it |
| `Left` | Back to the file view (`jobs.close`) |

Jobs copy and move one at a time; deletes start immediately. Add a transfer
paused from the copy or move dialog (**Add paused**) and start it later with
`Ctrl+R`. See [File operations](fileoperations.md#operation-dialogs).

# Transfer rate limit

The jobs view has a Queue panel on the left listing all
background copy/move jobs. While that panel is focused, you can cap how fast
transfers run. Delete jobs start immediately and run in parallel with
transfers instead of waiting behind a running copy/move.

The limit is global — it applies to all transfers together, not to a single
job — and it's session-only: it always starts back at Unlimited the next time
you launch paras-commander.

## Adjusting the limit

With the Queue panel focused:

- `+` or `-` from Unlimited both enable the limit, starting at whatever the
  currently running job is actually transferring at, rounded to the nearest
  10MB/s, or 100MB/s if nothing is running.
- `+` raises the limit by 10MB/s. `-` lowers it by 10MB/s, down to a floor of
  10MB/s — it never drops all the way back to Unlimited on its own.
- `Ctrl+U`, or **Jobs → Clear rate limit** in the menu, clears the limit back
  to Unlimited.

## Where it's shown

The current limit appears on the right side of the Queue panel's border
(e.g. `40MB/s`). Nothing is shown there when the limit is Unlimited.

# The Operations panel

The Operations panel lists every item the selected job has processed, newest at
the top. Set `operations_order = "newest_last"` under `[jobs]` in `config.toml`
to list them the other way round, oldest first with the newest at the bottom
like a log. While the job is running, the item it is working on right now is
the newest row, marked with the hard-disk icon (the one panel rows show while a
job works on them); it is hidden while a fuzzy query or result filter narrows
the list, or when a source row other than the active one is selected. Each
processed row starts with a result icon:

- green: copied, moved, deleted or extracted;
- yellow: skipped, followed by why (`exists` when you chose Skip at a "file
  exists" prompt, `identical` when a verified rule skipped a destination with
  the same content, `source deleted` when a move kept the destination and
  deleted its source, `permission denied` when you skipped a permission
  prompt);
- red: failed, followed by the reason.

A row whose conflict was resolved with Delete source has its own icon instead
of the skip icon; it still counts as skipped.

The panel follows new rows as they arrive. Once you scroll away from the newest
row in a focused Operations panel, the rows on screen stay put while the job
adds more; scrolling back to the newest row (`Home`, or `End` with
`newest_last`) follows again. Moving focus to another panel jumps back to the
newest row.

Files and symlinks are listed one per row. A directory gets a row only when it
was skipped as a whole; a move lists each moved top-level item. When a retried
item succeeds, its earlier failed row disappears.

The Details panel above shows the totals on its `Files:` line, for example
`1234 done, 2 failed, 5 skipped`, with the failed count in red and the skipped
count in yellow. A canceled job adds the files it left undone in yellow, for
example `120 done, 380 canceled`. Like the other counts it counts files, not
folders, so it can differ from the `Progress:` line, whose item count includes
folders. It shows `canceled: incomplete scan` when the files were never fully
counted (a copy canceled while still counting, and moves, which don't count
their files), and `canceled: unsupported` for flattens.

Filter the list while the panel is focused:

- Type to fuzzy-match the names, with the same matching as the file list's
  quick filter. This also works in the maximized panel, where every letter
  (including `q`) goes to the filter. Rows that don't match are hidden. `Backspace` edits the query,
  `Ctrl+L` or `Ctrl+Backspace` clears it, and `Esc` closes the filter. `Esc`
  again leaves the jobs view.
- `F3` opens the **Result** dialog: tick **Success**, **Failed**, **Skipped**,
  or **All**.
- `Shift+F3` clears both filters.

The query is shown as `> report` at the top left of the panel's border, in
place of its title, like the file list's quick filter. The result filter is
shown on the bottom border, for example `Failed`, plus `filtering…` while a
large list is still being searched. They stay in place while you move between jobs and are cleared when
you leave the jobs view. `Up`, `Down`, `PgUp`, `PgDn`, `Home` and `End` scroll
the panel. The icons and colors come from the theme (`[icons.jobs.operations]`
and `[jobs.operations]`).

While a job runs, the panel updates at most four times a second, so new rows
don't scroll past faster than you can read them (`operations_refresh_interval_ms`
under `[jobs]` in `config.toml`, `0` updates on every frame). Scrolling, typing
a filter and switching jobs update it immediately.

## Very large jobs

A job keeps the newest 50,000 rows in memory (`operations_memory_rows` under
`[jobs]` in `config.toml`; `0` keeps everything in memory). Past that, its list
is written to `~/.cache/pc/jobs/` (or `$XDG_CACHE_HOME/pc/jobs/`) so a job of
millions of files stays light. Scrolling into older rows briefly shows `…`
while they are read back. These files hold the file names the job processed;
they are readable only by you, and are deleted when the job is cleared from the
jobs view and when paras-commander exits. If the cache cannot be written, the
oldest rows are dropped instead and the Details panel says
`log incomplete, N dropped`.

# Items that cannot be copied

When a copy meets something it cannot copy, it records that item as failed and
carries on with the rest: a file type that cannot be copied (a socket, FIFO or device), or
a source file that cannot be read or has vanished. Problems on the destination
side (write errors, a full disk) still stop the job. Move jobs always stop at
the first problem.

A job with such items finishes as **failed** with `N item(s) failed`. Each
such item is a red row in the Operations panel, followed by its reason, and
the Details panel counts them. Press `F3` and keep only **Failed** to list just
those. Tab cycles Queue, blocker
(when shown), Details, Operations.

With the Details panel focused, `+` and `-` shorten or lengthen the transfer
graph's column time (the tickrate, and so how fast the Speed column reacts)
by 50 ms. The change lasts for the session; set `throughput_chart_column_ms`
in `[jobs]` to make it permanent.

# Undoing a job

A finished copy, move, rename or mass rename can be reversed from the jobs view
with **Jobs → Undo job** (`jobs.undo`). Renames and mass renames done in the
file view are listed as finished `rename` jobs, one job per rename (one per
Apply in the mass rename dialog), so they can be undone the same way. Undo asks
for confirmation (No is the default), then runs as its own `undo` job in the
queue.

- A copy undo removes what the copy created: files only if they still have the
  size and modification time they had when copied, directories only if they
  are empty. Anything you changed or added since is left alone.
- A move or rename undo moves each item back, and never overwrites: if the
  original name is taken again, that item is skipped. Several renamed files
  (including swaps such as `a` and `b` exchanging names) are reversed together.
- Every item that could not be reversed is listed as a failed row in the undo
  job's Operations panel, and the job finishes as failed with `N item(s) failed`. The original
  job then shows `Undo: undone`; the leftovers are not retried.
- A job that overwrote anything (or, for a move, kept a destination and
  deleted its source with Delete source) cannot be undone; its Details panel shows
  `Undo: irreversible (overwrote files)`. A canceled or failed copy can undo
  the part it did write.
- Not undoable: flatten, delete, extract, and a cross-device move that kept
  both files under a new name. Jobs are not kept across restarts, so neither
  is their undo.

# Retrying a failed or canceled job

A copy, move, flatten, delete, or extract job that fails (e.g. `permission
denied` on the destination) stays in the Queue panel as a failed row instead
of disappearing; a canceled one stays as a canceled row. Select it and press `Ctrl+R` — the footer shows **Retry**
and the `:` leader menu shows "Retry job" while a failed or canceled job is
selected — to re-run it from scratch under the same job ID. A copy that
failed only because it skipped items re-runs just those items. Any files
already transferred before the failure hit the normal overwrite/skip
conflict prompt on retry.

`Ctrl+R` (**Jobs → Resume / retry job**) is the same key used to resume a
paused job; it resumes when the selected job is paused and retries when it
is failed or canceled.

## File list indicators

While a job is waiting or running, every file-list row it touches (a source, a
destination, or anything inside those directories) gets small icons after its
name, in this order:

1. **Job icon** — shown only while a job is actually working on the row
   (scanning or transferring); hidden while the job is queued or paused. Green
   when the row is on the job's writing side (a destination), yellow when it is
   being read (a source), red while the job is waiting for you to resolve a
   conflict.
2. **Clock** — shown only while the job is still queued and has not started.
3. **Operation icon** — a red *move* glyph on the source rows of moves (and
   flatten; destination rows show none), a red *delete* glyph for deletes.
   Copies and extracts show none.

Source rows keep their listing entries for the whole job (queued move/delete
show icons rather than vanishing early). Destination names appear in a panel
as soon as that panel's directory listing refreshes during an active write —
progress and job-start wakes re-list panels whose cwd is inside the job's
destination tree — and then carry the green job icon while the transfer is
active. The same icons appear in the carousel view and in the bottom-border
name hint for the highlighted row.

A panel whose current directory is inside a job's write (destination) tree
also shows the job icon on the **physical-right** corner of its bottom border.

# File conflicts

When a copy, move, flatten or extract finds that the destination already exists, the job waits
for a decision. It is asked in the quick blocker dialog or in the conflict panel of the jobs view.

| Button | Alt | Effect |
| --- | --- | --- |
| Overwrite | O | Replace this file |
| Overwrite All | A | Replace this and every later conflict in the job |
| Advanced... | D | Open the "Conflict rules" dialog |
| Skip | S | Keep the existing file |
| Skip All | K | Skip this and every later conflict in the job |
| Delete source | E | Move only: keep the existing file and delete the one being moved |
| Cancel | C | Abort the job |
| Postpone | P | Dismiss the quick dialog without answering (quick dialog only) |

When exactly one side is a directory, the prompt says `Destination is a
directory` or `Destination is a file` and offers Rename (`R`), Rename All
(`A`), Skip, Skip All and Cancel instead of the overwrite buttons (and no
Delete source). When the
target disk is too full the prompt offers Retry (`R`) and Abort (`B`).

### Answering from the file view

You do not have to open the jobs view. `Ctrl+Q` or `Alt+Q`
(`jobs.answer-blocker`, Esc menu `a`) opens the quick dialog for the job that
has waited longest. Postpone dismisses it and the job keeps waiting. After you
answer, the next waiting job's dialog opens automatically.

## Conflict rules

Advanced... opens a matrix where every condition picks its own action: Ask, Overwrite, Skip or
Keep both. Up/Down move between rows, Left/Right between the choices of a row, Space or Enter picks
one. Tab jumps between the time rows, the size rows, the Verify section, Apply to all and the
buttons.

| Rows | Conditions (relative to the destination file) |
| --- | --- |
| Time | Destination newer, Destination older, Same time (compared to the whole second) |
| Size | Destination smaller, Destination larger, Same size |
| Verify | Same, Differ (contents, only with **Verify both to see if identical** ticked) |

Every conflicting file matches exactly one time row and one size row. The action is the time row's
if it is not Ask, else the size row's, else Ask. Ask means "no rule here": if no row decides, the
File exists dialog comes back for that file with "No rule matched." (even when the rules were
applied to all conflicts).

- **Keep both** writes the new file as `name (1).ext` (first free number) and leaves the existing
  file alone.
- **Greyed choices:** the time rows take precedence. A size choice is greyed out when a time row
  holds a different non-Ask action, since a file can be newer and smaller at once. Changing a time
  row resets any size choice it greys out to Ask.
- **Apply to all conflicts** (Alt+A, on by default) makes the rules the job's policy, so later
  conflicts are decided without asking; uncheck it to apply the rules to the current file only.

### Verify both to see if identical

Ticking **Verify both to see if identical** (Alt+V, off by default) compares the contents of the
two files before any other row is looked at, and the **Same** and **Differ** rows decide first:

| Row | Choices |
| --- | --- |
| Same | Ask, Overwrite, Skip, Delete source |
| Differ | Rules, Ask, Overwrite, Skip, Keep both, Delete source |

- **Rules** (Differ only, the default) hands a differing file to the time and size rows. Any other
  Differ choice decides every differing file itself, so the time and size rows are greyed out.
- **Same → Ask** asks with "Contents identical. No rule matched."; it does not fall back to the
  time and size rows.
- **Skip** keeps both files where they are. An identical file is listed as `identical` in the
  Operations panel; on a move its source stays.
- **Delete source** is offered for moves only: the existing file is kept and the file being moved
  is deleted. The job then cannot be undone.
- Files of different size differ without being read. Files of equal size are read side by side
  and compared byte for byte, stopping at the first difference: the same check as
  [Compare directories](compare-directories.md). A symlink
  always counts as differing. The Details panel shows a `Verifying:` line with the bytes read so
  far while a large file is checked.
- Verify is not offered when extracting an archive (the archive is not comparable to the
  extracted output).

The rules replace a plain "match size" switch: use the Same size row (Skip or
Overwrite) together with the time rows to express it.

OK (Alt+O) submits; Cancel (or Esc) goes back to the File exists dialog.

See also [Background scan throttling](background-scan-throttle.md) and [Running as root](elevation.md).
