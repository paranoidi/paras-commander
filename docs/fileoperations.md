# File operations: how a copy reaches the disk

This page explains what paras-commander does between reading a source file
and calling the copy done, why the job speed you see is what it is, and how
that compares to Midnight Commander and yazi. Everything here is tunable under
`[operations]` in `config.toml` (see `config.md`).

## The copy path

For a local-to-local file, in order:

1. **Clone** (`cow_file_cloning`, default on): `ioctl(FICLONE)`. On a
   filesystem that supports it (btrfs, XFS with reflinks, ZFS with block
   cloning enabled, same dataset/pool) the file is done instantly with no
   data copied.
2. **Kernel copy** (`copy_file_range`, default on): `copy_file_range(2)` in
   8 MiB chunks. Cross-pool / cross-filesystem copies end up here on modern
   kernels; the data still moves, but without a round trip through userspace.
3. **Userspace copy**: `read`/`write` with a `copy_buffer_kib` buffer
   (default 256 KiB), optionally hole-preserving (`sparse_file_copy`).

Progress is reported after every chunk on all three paths.

## What "done" means: durability

By default (`sync_after_each_file = true`) a file is not counted as
transferred until it has been `fsync`'d — the data is on the disk, not just
in the kernel's write cache. This matters most for **move**: sources are only
deleted after their copies are durable, so a power cut or a yanked USB
enclosure mid-move cannot leave you with neither copy.

`sync_min_file_kib` skips the fsync for small files (lots of tiny files fsync
slowly); `sync_at_job_end = true` with `sync_after_each_file = false` does one
sync pass at the end of the job instead.

## Write-behind and the speed display

Filesystems accept writes far faster than the disk can absorb them. ZFS
buffers up to `zfs_dirty_data_max` (up to 4 GB) and only starts throttling
`write()` at 60 % of that; Linux's page cache does the same with
`vm.dirty_ratio` (20 % of RAM by default). So a multi-gigabyte file can be
"written" at memory speed and then sit in a single end-of-file `fsync` for
tens of seconds while it actually drains to a slow target — with nothing
left to report. The Speed column and menu-bar pill are sampled on a fixed
clock and feed zero for idle time on purpose (they show what is happening,
not what happened), so during that fsync they decay toward zero and then
jump back when the next file starts filling the cache.

`sync_write_behind_mib` (default 64) bounds that gap: whenever a file is
going to be fsync'd anyway, it is also fsync'd every 64 MiB during the copy.
Reported progress can never run more than 64 MiB ahead of the disk, the
end-of-file fsync is short, and the displayed speed tracks the real write
rate for the whole file. Set it to `0` for a single fsync per file.

Costs and interactions to know about:

- Every fsync ends with a device cache-flush command. Expect a few percent
  of throughput on fast disks, more on USB bridges with slow flush latency;
  raise the value if that shows up.
- ZFS without a separate log device logs large sequential writes by
  reference (`WR_INDIRECT`): the data is written once to its final location,
  so periodic fsync does not double-write it. Snapshots are unaffected — they
  are crash-consistent regardless of userspace syncing.
- A dataset with `sync=disabled` turns fsync into a no-op; you then get the
  single-flush behaviour and its speed-display decay back.
- With `sync_after_each_file = false` no write-behind sync happens at all.
  The kernel's own write throttle then paces `write()` at disk speed once the
  cache is full, so progress is still accurate — but the job finishes with up
  to a cache's worth of data still in flight.

## Compared to other file managers

| | paras-commander | Midnight Commander | yazi |
|---|---|---|---|
| Clone / kernel copy | FICLONE → `copy_file_range` → userspace | FICLONE → userspace `read`/`write` | `std::fs::copy` (`copy_file_range`) |
| Progress source | Bytes written, per chunk | Bytes written, per buffer | Destination file size polled every 3 s |
| fsync | Per file by default, with write-behind every 64 MiB | Never | Never |
| Move = copy + delete source | After the copy is durable | After the copy returns (data may still be in cache) | After the copy returns |

mc and yazi trust the kernel: the file is "done" when `write()` returns, and
their speed never stalls because they never wait for the disk. That is also
why they never show the decay described above — and why a move in either can
delete the source while the destination is still only in RAM. paras-commander
defaults to the stricter semantics; `sync_after_each_file = false` gives you
the mc/yazi behaviour if you prefer it.
