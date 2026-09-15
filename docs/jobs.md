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
