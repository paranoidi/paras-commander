# Background scan throttling

Copy jobs (including flatten-into-destination copies) start transferring as soon
as the first source item is discovered, instead of waiting for the whole source
tree to be walked first. Move and flatten jobs do not pre-scan: they rename each
source first and only walk and copy a source whose rename is not possible
(different filesystem or host), so move progress counts top-level items and
a flatten job shows its moved-item count without a total.
While the transfer runs, a second background walk counts the remaining files,
directories, and bytes so the job's totals and ETA keep growing.

That counting walk runs unbounded — nothing downstream blocks it — so it reads
directory entries and file metadata at full speed regardless of transfer
pace. On SSD/NVMe/network storage that's essentially free. On a mechanical
HDD, the disk head has to keep jumping between wherever the transfer is
reading/writing and wherever the counting walk currently is, which can
measurably slow the transfer down.

## Adaptive throttle

Rather than guessing based on disk type (unreliable — virtualized, RAID,
dm-crypt, and USB-attached SSD devices routinely misreport as rotational),
paras-commander measures the actual effect: every few seconds, while a
transfer is active, it briefly pauses the counting walk and compares the
job's measured throughput just before vs. during the pause.

- If pausing measurably improves throughput, the counting walk is throttled
  with a duty-cycle pause that grows on each such probe (up to a cap).
- If pausing makes no measurable difference, any existing throttle decays
  back toward zero.

This re-probes throughout the job's lifetime, since contention can change —
a job copying a few huge files behaves differently than one copying millions
of small ones. On non-rotational or otherwise uncontended storage, probes
never show a benefit, so the throttle stays at zero and the counting walk
runs at full speed, same as without this feature.

The counting walk is throttled independently of the actual transfer walk,
which is never slowed by this — it's already self-limiting.

## Keeping the UI responsive

Both background walks also sleep briefly per time slice: after walking for
`scan_yield_work_ms` they sleep `scan_yield_interval_ms`, unconditionally,
not only while a transfer happens to be active. This keeps a large scan from
ever fully saturating the CPU/scheduler at the terminal UI's expense, even
before the first item has been found and the job has started transferring.
Because the sleep follows elapsed time rather than a number of entries, a walk
over metadata the kernel already has cached runs at full disk speed.
The adaptive throttle's pause is added to these sleeps, so even a fully
throttled counting walk keeps making progress.

## Estimated totals

When a copy or move is queued, the job starts with the size the panel already
knows for the selection: file sizes from the listing plus directory sizes from
the disk-usage cache. The percentage, ETA and menu-bar progress bar work right
away, and the **Details** panel marks the total `(estimated)`.

- A copy uses any non-zero estimate. Its total never drops below the estimate
  while the counting walk runs, and switches to the exact counted size when the
  walk finishes.
- A move first renames every source it can, which is instant. It uses the
  estimate only when every source's size is known, and each source moved by
  rename counts its estimated size as done. Sources that cannot be renamed
  (another device or host) are copied, and the move counts just those in the
  background while the copy runs. The total switches to the exact counted size
  when that count finishes, and no source is deleted before it has.

While a copy or move has not finished counting, its **Details** progress line
shows `(scanning…)`, since the totals and percentage are still provisional and
will keep climbing.

## Disabling it

If you suspect the throttle is misfiring (e.g. slower totals updates with no
actual transfer benefit), disable it in `config.toml`:

```toml
[jobs]
scan_disable_adaptive_throttle = true
```

See [Config](config.md#jobs) for the full list of `[jobs]` settings, and [Jobs](jobs.md) for the jobs view and its Details panel.
