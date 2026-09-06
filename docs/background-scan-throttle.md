# Background scan throttling

Copy/move/flatten jobs start transferring as soon as the first source item is
discovered, instead of waiting for the whole source tree to be walked first.
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

Both background walks also sleep briefly on a fixed cadence
(`scan_yield_interval_ms` every `scan_yield_every_n` entries) unconditionally,
not only while a transfer happens to be active. This keeps a large scan from
ever fully saturating the CPU/scheduler at the terminal UI's expense, even
before the first item has been found and the job has started transferring.

While a copy/move/flatten job's background counting walk hasn't finished
enumerating the source tree yet, its entry in the jobs view **Details**
panel shows a `(scanning…)` marker next to the progress line, since the
totals and percentage shown are still provisional and will keep climbing.

## Disabling it

If you suspect the throttle is misfiring (e.g. slower totals updates with no
actual transfer benefit), disable it in `config.toml`:

```toml
[jobs]
scan_disable_adaptive_throttle = true
```

See [config.md](config.md) for the full list of `[jobs]` settings.
