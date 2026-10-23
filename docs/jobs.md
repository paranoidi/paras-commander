# Jobs

Copy, move, flatten, extract and delete operations run as background jobs, so
the file manager stays usable while they work. The menu bar shows a compact
summary: an icon with a count for each job state, a progress bar for the
running job and, to its left, a pill with the current transfer speed while a
transfer runs. A job waiting for your answer is shown as "N job(s) waiting".

Open the jobs view with `Alt+J` (`jobs.open`, **Display -> Jobs**, Esc menu
`j`). It has a Queue panel with Type, Status, ETA, Speed and Progress, plus
Details, Errors and Succeeded panels for the selected job. When a job is
waiting for an answer, opening the view selects it and focuses the conflict
panel. Running and queued jobs are listed first in run order (so the queue
move keys match the screen), followed by finished jobs with the most recent
first. Keys are rebindable under `[jobs]` in `keybindings.toml`.

| Key | Action |
|---|---|
| `Ctrl+C` | Cancel the selected job (`jobs.cancel`) |
| `Ctrl+P` | Pause a queued job (`jobs.pause`) |
| `Ctrl+R` | Resume a paused job or retry a failed one (`jobs.resume`) |
| `Ctrl+Up` / `Ctrl+Down` | Move the job in the queue (`jobs.queue-up`, `jobs.queue-down`) |
| `F8` | Clear finished jobs (`jobs.clear-finished`) |
| `+` / `-` / `Ctrl+U` | Raise, lower or clear the transfer rate limit (Queue panel focused) |
| `Tab` | Cycle the panels |
| `Enter` | Maximize the focused Errors or Succeeded panel with wrapped lines (`jobs.maximize-pane`); `Esc`, `q`, `Enter` or `Left` restore it |
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

# Skipped items and the Errors panel

When a copy meets something it cannot copy, it skips that item and carries on
with the rest: a file type that cannot be copied (a socket, FIFO or device), or
a source file that cannot be read or has vanished. Problems on the destination
side (write errors, a full disk) still stop the job. Move jobs always stop at
the first problem.

A job that skipped items finishes as **failed** with `N item(s) failed`. In the
jobs view the right column shows an **Errors** panel (above **Succeeded**) listing
each skipped item as `<source path>: <reason>`. The panel appears only when the
selected job has errors. Tab cycles Queue, blocker (when shown), Details,
Errors (when shown), Succeeded.

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
- Every item that could not be reversed is listed in the undo job's Errors
  panel, and the job finishes as failed with `N item(s) failed`. The original
  job then shows `Undo: undone`; the leftovers are not retried.
- A job that overwrote anything (or, for a move, replaced an identical file and
  deleted its source) cannot be undone; its Details panel shows
  `Undo: irreversible (overwrote files)`. A canceled or failed copy can undo
  the part it did write.
- Not undoable: flatten, delete, extract, and a cross-device move that kept
  both files under a new name. Jobs are not kept across restarts, so neither
  is their undo.

# Retrying a failed job

A copy, move, flatten, delete, or extract job that fails (e.g. `permission
denied` on the destination) stays in the Queue panel as a failed row instead
of disappearing. Select it and press `Ctrl+R` — the footer shows **Retry**
and the `:` leader menu shows "Retry failed job" while a failed job is
selected — to re-run it from scratch under the same job ID. A copy that
failed only because it skipped items re-runs just those items. Any files
already transferred before the failure hit the normal overwrite/skip
conflict prompt on retry.

`Ctrl+R` (**Jobs → Resume / retry job**) is the same key used to resume a
paused job; it resumes when the selected job is paused and retries when it
is failed.

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
| Skip All | L | Skip this and every later conflict in the job |
| Cancel | C | Abort the job |
| Postpone | P | Dismiss the quick dialog without answering (quick dialog only) |

When exactly one side is a directory, the prompt says `Destination is a
directory` or `Destination is a file` and offers Rename (`R`), Rename All
(`A`), Skip, Skip All and Cancel instead of the overwrite buttons. When the
target disk is too full the prompt offers Retry (`R`) and Abort (`B`).

### Answering from the file view

You do not have to open the jobs view. `Ctrl+Q` or `Alt+Q`
(`jobs.answer-blocker`, Esc menu `a`) opens the quick dialog for the job that
has waited longest. Postpone dismisses it and the job keeps waiting. After you
answer, the next waiting job's dialog opens automatically.

## Conflict rules

Advanced... opens a matrix where every condition picks its own action: Ask, Overwrite, Skip or
Keep both. Up/Down move between rows, Left/Right between the choices of a row, Space or Enter picks
one. Tab jumps between the time rows, the size rows, the checkboxes and the buttons.

| Rows | Conditions (relative to the destination file) |
| --- | --- |
| Time | Destination newer, Destination older, Same time (compared to the whole second) |
| Size | Destination smaller, Destination larger, Same size |

Every conflicting file matches exactly one time row and one size row. The action is the time row's
if it is not Ask, else the size row's, else Ask. Ask means "no rule here": if no row decides, the
File exists dialog comes back for that file with "No rule matched." (even when the rules were
applied to all conflicts).

- **Keep both** writes the new file as `name (1).ext` (first free number) and leaves the existing
  file alone.
- **Greyed choices:** the time rows take precedence. A size choice is greyed out when a time row
  holds a different non-Ask action, since a file can be newer and smaller at once. Changing a time
  row resets any size choice it greys out to Ask.
- **Skip identical files (slow)** (Alt+I, off by default) runs first: files of equal size are compared with
  the same chunked hashing as Find Duplicates (`dedup.chunk_bytes`), stopping at the first
  differing chunk. Identical files are not copied; on a move the source is removed, so no
  identical leftovers remain. Different files continue to the rows. The option is not offered when
  extracting an archive (the archive is not comparable to the extracted output).
- **Apply to all conflicts** (Alt+A, on by default) makes the rules the job's policy, so later
  conflicts are decided without asking; uncheck it to apply the rules to the current file only.

The rules replace a plain "match size" switch: use the Same size row (Skip or
Overwrite) together with the time rows to express it.

OK (Alt+O) submits; Cancel (or Esc) goes back to the File exists dialog.

See also [Background scan throttling](background-scan-throttle.md) and [Running as root](elevation.md).
