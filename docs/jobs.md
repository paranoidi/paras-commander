# Transfer rate limit

The jobs view (open with Alt+J) has a Queue panel on the left listing all
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

# Retrying a failed job

A copy, move, flatten, delete, or extract job that fails (e.g. `permission
denied` on the destination) stays in the Queue panel as a failed row instead
of disappearing. Select it and press `Ctrl+R` — the footer shows **Retry**
and the `:` leader menu shows "Retry failed job" while a failed job is
selected — to re-run it from scratch under the same job ID. Any files
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

OK (Alt+O) submits; Cancel (or Esc) goes back to the File exists dialog.
