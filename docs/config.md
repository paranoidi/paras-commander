# config.toml reference

Paras Commander reads its general settings from `config.toml`. The file is
optional — any key you omit falls back to a built-in default, and any
out-of-range value you do set is clamped back into a valid range rather than
causing a startup error. An unknown key, however, is rejected at load time
(useful for catching typos).

## Location

`config.toml` lives at `$XDG_CONFIG_HOME/pc/config.toml`, or
`~/.config/pc/config.toml` if `XDG_CONFIG_HOME` is unset.

## Generating a fresh example

Run:

```sh
pc --config-stub
```

This writes a `config.toml` containing every key at its current built-in
default. It's the fastest way to get a copy of every available key, correctly
named and up to date with the installed binary — treat it as more
authoritative than any example in this document if the two ever disagree.

## Related but separate files

These live alongside `config.toml` in the same config directory but are
**not** part of it — each is its own TOML file with its own keys:

| File | Purpose | Referenced from config.toml via |
|---|---|---|
| `keybindings.toml` | Keyboard shortcuts. **Warning:** it has its own `[jobs]` and `[dialog.input]` tables that are unrelated to config.toml's `[jobs]` table — same table name, different file, different purpose. | — |
| `menu.toml` | User menu (F2) command definitions. | `[user_menu].file` |
| `meta.toml` | Meta column command definitions. | `[meta].file` |
| `pools.toml` | Worker pool definitions. | `[pools].file` |
| `themes/*.toml` | Color themes. | top-level `theme` key |

## Contents

- [General (root keys)](#general-root-keys)
- [\[panels\]](#panels)
- [\[disk_usage\]](#disk_usage)
- [\[carousel\]](#carousel)
- [\[ui\]](#ui)
- [\[ui.zoom\]](#uizoom)
- [\[ui.scroll\]](#uiscroll)
- [\[ui.find\]](#uifind)
- [\[ui.status\]](#uistatus)
- [\[filter\]](#filter)
- [\[jobs\]](#jobs)
- [\[operations\]](#operations)
- [\[logging\]](#logging)
- [\[bookmarks\]](#bookmarks)
- [\[user_menu\]](#user_menu)
- [\[preview\]](#preview)
- [\[sftp\]](#sftp)
- [\[shell\]](#shell)
- [\[meta\]](#meta)
- [\[pools\]](#pools)
- [\[compare\]](#compare)
- [\[dedup\]](#dedup)

## General (root keys)

Keys at the top level of the file, outside any `[table]`.

| Key | Type | Default | Description |
|---|---|---|---|
| `theme` | string | `"default"` | Name of the active color theme (see `themes/`). |

## `[panels]`

File panel browsing, sorting, and listing.

| Key | Type | Default | Description |
|---|---|---|---|
| `show_hidden` | bool | `false` | Show dotfiles and other hidden entries in panel listings on startup. |
| `respect_gitignore` | bool | `true` | Hide files matched by `.gitignore` in panel listings. |
| `default_sort` | string | `"name"` | Default panel sort order: `"name"`, `"extension"`, `"size"`, or `"mtime"`. |
| `default_listing_format` | string | `"brief"` | Default listing column layout: `"mtime"` (modified time), `"perm"` (permissions), or `"brief"` (minimal columns). |
| `sort_reverse` | bool | `false` | Reverse the default sort order. |
| `directories_first` | bool | `true` | List directories before files regardless of sort order. |
| `refresh_interval_ms` | int | `2500` | How often panels re-read their directory from disk in the background. `0` disables automatic refresh; non-zero values are clamped to 200–60000. |
| `open_files_externally` | bool | `true` | Open non-executable files with the OS-associated external application on Enter. |
| `run_executables_on_enter` | bool | `true` | Run executable files directly when pressing Enter on them. |

## `[disk_usage]`

Disk-usage view and background walk.

| Key | Type | Default | Description |
|---|---|---|---|
| `idle_size_sort` | bool | `true` | While a disk-usage scan is running, re-sort by size once the cursor has been idle for a moment instead of resorting on every update. |
| `idle_sort_delay_ms` | int | `500` | How long the cursor must be idle before the disk-usage idle re-sort (above) triggers. |
| `descend_into_mount_points` | bool | `false` | Let a disk-usage scan cross into other mounted filesystems instead of stopping at mount boundaries. |
| `walk_concurrency` | int | `4` | Number of subdirectory branches a disk-usage scan walks concurrently (minimum 1). Lower this on slow HDDs/NAS, raise it on fast local SSDs. |

## `[carousel]`

Carousel (multi-column) panel layout.

| Key | Type | Default | Description |
|---|---|---|---|
| `split` | array of strings | `["<33%", "<33%", "*"]` | Column widths for carousel view (parent \| center \| child), one token per column: a fixed cell count (`"16"`), a percent of remaining width (`"20%"`), the flex remainder (`"*"`), or a fit-to-content cap — `"<16"` sizes the column to the longest filename currently listed in it, capped at 16 characters, and `"<33%"` does the same capped at 33% of the panel's interior width. Fit-to-content tokens (`<N` / `<N%`) are only valid for the parent (index 0) and center (index 1) columns — the child column can show an embedded file preview instead of a listing, so it has no "longest filename" to fit to, and a `<` token there is rejected like any other malformed entry. Must be exactly 3 entries. |
| `show_size` | array of bools | `[false, true, true]` | Whether to show the size column in each of the 3 carousel columns. |

## `[ui]`

General interface layout, timing, and rendering behavior.

| Key | Type | Default | Description |
|---|---|---|---|
| `show_menu_bar` | bool | `true` | Show the top menu bar. |
| `show_file_icons` | bool | `true` | Show file-type icons/glyphs in panel listings. |
| `shrunken_shows_name_only` | bool | `true` | When a panel becomes too narrow for its listing columns, show only the name column instead of truncating everything. |
| `screen_render_hash_cache` | bool | `true` | Skip re-drawing the terminal screen when nothing actually changed since the last frame — reduces flicker and I/O over slow connections. |
| `key_repeat_debounce_ms` | int | `35` | Coalesces rapid repeated key presses (file-list cursor steps, quick-view/carousel preview reloads, F3 style-picker highlighting) so they don't reload on every single step. `0` disables debouncing; values above 10000 are clamped. |
| `path_picker_validate_delay_ms` | int | `200` | Delay after the path-picker filter changes before checking whether the typed path exists on disk. |
| `selections_panel_max_rows` | int | `0` (→ 5) | Maximum visible rows in the cross-directory selections strip. `0` means use the built-in default of 5. |

## `[ui.zoom]`

Active/inactive panel width zoom and twin-panel orientation.

| Key | Type | Default | Description |
|---|---|---|---|
| `active_panel` | bool | `true` | Widen the active panel and shrink the inactive one, sized by `active_percent` / `inactive_percent`. |
| `disabled_above_width` | int | `140` | When greater than 0 and the terminal is at least this many cells wide, panel zoom is suppressed in favor of an even 50/50 split. `0` disables this width-based gate. |
| `disabled_above_height` | int | `45` | Same as above but for terminal height, applied when panels are stacked top/bottom. `0` disables this gate. |
| `orientation` | string | `"side_by_side"` | Twin-panel layout: `"side_by_side"` or `"stacked"`. |
| `active_percent` | int | `70` | Width share given to the active panel when `active_panel` is enabled. Must sum to 100 with `inactive_percent`. |
| `inactive_percent` | int | `30` | Width share given to the inactive panel when `active_panel` is enabled. |

## `[ui.scroll]`

File-list scroll policy and scrollbar display.

| Key | Type | Default | Description |
|---|---|---|---|
| `mode` | string | `"edge"` | File-list scroll policy: `"minimal"`, `"center"`, or `"edge"`. |
| `edge_margin` | int | `5` | In `"edge"` scroll mode, how many rows of buffer to keep above/below the cursor before scrolling (clamped to 0–50). |
| `scrollbar` | string | `"thumb"` | Vertical scroll indicator style for panel lists: `"none"`, `"thumb"`, or `"bar"`. |
| `scrollbar_inactive` | bool | `false` | Also show the scroll indicator on the inactive panel. |

## `[ui.find]`

Find dialog ranking and result timing.

| Key | Type | Default | Description |
|---|---|---|---|
| `query_debounce_ms` | int | `150` | Delay after the last keystroke in the find dialog before re-ranking results. `0` re-ranks on every keystroke. |
| `max_results` | int | `200` | Maximum number of ranked results shown in the find dialog (the full index is still searched; only the displayed top-N is capped). |
| `list_nav_idle_ms` | int | `400` | How long the find result list must be idle (no arrow-key movement) before a background rank update is applied to the view. |

## `[ui.status]`

Transient status banners and the Messages view log.

| Key | Type | Default | Description |
|---|---|---|---|
| `message_ttl_seconds` | float | `4.5` | How long transient status banners stay visible before clearing automatically. `0` keeps a message until it's replaced or explicitly cleared. |
| `log_max_entries` | int | `500` | Maximum number of status/toast lines retained for the Messages view (oldest entries are dropped first). |

## `[filter]`

Panel quick-filter behavior.

| Key | Type | Default | Description |
|---|---|---|---|
| `mode` | string | `"fuzzy"` | Quick-filter matching mode. Currently only `"fuzzy"` is supported. |
| `syntax` | string | `"subset-fzf"` | Quick-filter query syntax. Currently only `"subset-fzf"` is supported. |
| `match_path_segments` | bool | `false` | Match filter terms against full path segments instead of just the file name. |
| `cycle_matches` | string | `"visual"` | How Up/Down move among quick-filter matches: `"visual"` (panel row order) or `"ranked"` (best fuzzy match first). |
| `case_insensitive` | bool | `true` | Match panel quick-filter and find queries case-insensitively. |

## `[jobs]`

Background file-operation jobs: display, timing, and throttling.

| Key | Type | Default | Description |
|---|---|---|---|
| `show_finished` | bool | `true` | Keep finished jobs visible in the jobs view instead of removing them immediately. |
| `keep_finished` | int | `20` | Maximum number of finished jobs to retain in the jobs view. |
| `autoshow_on_error` | bool | `true` | Automatically open the jobs view when a job hits an error. |
| `autoshow_on_start` | bool | `false` | Automatically open the jobs view whenever a new job starts. |
| `progress_ui_wake_debounce_ms` | int | `150` | Minimum spacing between UI refreshes triggered by worker progress events (clamped 50–5000). |
| `blocker_dialog_next_debounce_ms` | int | `200` | Delay before automatically opening the next queued blocker dialog (e.g. overwrite prompt) after answering one. `0` opens immediately (clamped 0–5000). |
| `worker_progress_min_bytes` | int | `524288` (512 KiB) | Minimum bytes copied between worker progress events (clamped 64 KiB–64 MiB). |
| `worker_progress_min_interval_ms` | int | `200` | Minimum milliseconds between worker progress events while copying (clamped 50–5000). |
| `throughput_chart_window_sec` | int | `45` | Time span shown by the job details throughput chart, in seconds (clamped 20–120). |
| `throughput_chart_column_ms` | int | `400` | Milliseconds represented by each throughput chart column, and the chart's redraw interval (clamped 80–2000). |
| `throughput_chart_enabled` | bool | `true` | Show the throughput strip and chart in job details. |
| `free_space_on_progress_wake` | bool | `true` | Refresh both panels' free-space display whenever a job progress update wakes the UI. |
| `free_space_poll_interval_secs` | int | `5` | How often to refresh panel free space while any job is running. `0` disables polling (max 3600). |
| `scan_yield_interval_ms` | int | `50` | Cooperative sleep interval during a pre-copy directory scan while a transfer job is active, so the UI stays responsive (clamped 50–5000). |
| `scan_yield_every_n` | int | `64` | Number of walked entries between cooperative yields during pre-scan (max 4096). |
| `scan_nice_increment` | int | `10` | Linux `nice` increment applied to pre-scan while a transfer job is active, so scanning doesn't starve the copy (0–19). |
| `scan_progress_min_interval_ms` | int | `200` | Throttle for scan-progress UI updates during pre-scan (clamped 50–5000). |

## `[operations]`

Copy/move file-transfer behavior.

| Key | Type | Default | Description |
|---|---|---|---|
| `confirm_delete` | bool | `true` | Ask for confirmation before deleting files or directories. |
| `delete_mode` | string | `"permanent"` | How delete removes files. Currently only `"permanent"` is supported. |
| `preserve_permissions` | bool | `true` | Preserve source file permissions on copy. |
| `preserve_timestamps` | bool | `true` | Preserve source file modification times on copy. |
| `copy_buffer_kib` | int | `256` | Read/write buffer size, in KiB, used for userspace file copies. |
| `sync_after_each_file` | bool | `true` | fsync each copied file before closing it. Durable, but slow when copying many small files. |
| `disk_space_check_min_file_bytes` | int64 | `52428800` (50 MiB) | Only run the mid-copy free-space check for files at least this large. `0` checks before every file. |
| `cow_file_cloning` | bool | `true` | Use Linux copy-on-write file cloning (`FICLONE`) when the filesystem supports it, avoiding a full data copy. |
| `copy_file_range` | bool | `true` | Try Linux `copy_file_range(2)` (after CoW cloning) before falling back to a userspace read/write copy loop. |
| `sparse_file_copy` | bool | `false` | Preserve holes in sparse files on Linux via `SEEK_DATA`/`SEEK_HOLE` instead of writing zeroed regions. |
| `preallocate_destination` | bool | `false` | Reserve destination disk space up front (fallocate/truncate) before copying, when the source size is known. |
| `preallocate_min_file_bytes` | int64 | `1048576` (1 MiB) | Only preallocate for files at least this large. `0` preallocates for every file. |
| `sync_at_job_end` | bool | `false` | fsync all copied local files once at the end of the job, instead of per-file (only relevant when `sync_after_each_file` is `false`). |
| `sync_min_file_kib` | int | `0` | Skip fsync for copied files smaller than this size. `0` means no minimum (all files are synced). |
| `flatten_default_location` | string | `"active"` | Default destination panel prefilled in the flatten dialog: `"active"` or `"inactive"`. |
| `flatten_recursive` | bool | `false` | Default state of the flatten dialog's "recursive" checkbox. |
| `flatten_remove_empty_dirs` | bool | `true` | Default state of the flatten dialog's "remove empty directories" checkbox. |
| `rename_focus_after` | bool | `false` | Default state of the rename dialog's "focus after rename" checkbox. |
| `remove_dangling_directories` | bool | `true` | After a move or delete job finishes, prompt (default answer: Yes) to remove directories that were left empty by the operation. |

## `[logging]`

Application log file.

| Key | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Write application logs to disk. |
| `level` | string | `"info"` | Minimum log level: `"debug"`, `"info"`, `"warn"`, or `"error"`. |
| `path` | string | `""` | Log file path. Empty uses the built-in default location. |

## `[bookmarks]`

Directory bookmarks (fzf-marks compatible).

| Key | Type | Default | Description |
|---|---|---|---|
| `file` | string | `""` | Path to the marks file. Empty uses the `FZF_MARKS_FILE` environment variable, falling back to `~/.fzf-marks`. |

## `[user_menu]`

F2 user menu command definitions.

| Key | Type | Default | Description |
|---|---|---|---|
| `file` | string | `""` | Path to the global `menu.toml`. Empty uses `<config dir>/menu.toml`. |
| `local_names` | array of strings | `["menu.toml"]` | Basenames probed in the active panel directory (for a per-directory menu) before falling back to the global file. |

## `[preview]`

File preview pane (F3 / quick view / carousel).

| Key | Type | Default | Description |
|---|---|---|---|
| `mode` | string | `"internal"` | Preview engine: `"internal"` (built-in syntax highlighting) or `"external"` (runs `command` as a subprocess). |
| `style` | string | `"catppuccin-frappe"` | Syntax highlighting theme name used in internal mode. |
| `line_numbers` | bool | `true` | Show a line-number gutter in internal mode. |
| `command` | string | `"bat -n --paging=never --color=always --wrap=auto --terminal-width=%w %f"` | External preview command template, used when `mode` is `"external"`. Use `%f` once for the file path and `%w` for the terminal width; if `%f` is omitted, the path is appended. |

## `[sftp]`

SSH/SFTP remote panel connections.

| Key | Type | Default | Description |
|---|---|---|---|
| `known_hosts_file` | string | `""` | Path to the OpenSSH `known_hosts` file. Empty uses `~/.ssh/known_hosts`. |
| `ssh_config_file` | string | `""` | Path to the OpenSSH client config file. Empty uses `~/.ssh/config`. |
| `idle_timeout_secs` | int | `60` | How long an unused pooled SFTP connection stays open before closing (clamped 15–3600). |
| `dial_timeout_secs` | int | `30` | Limit on TCP connect + SSH handshake time (clamped 5–300). |
| `list_timeout_secs` | int | `60` | Limit on a remote directory listing request (clamped 5–300). |

## `[shell]`

Drop-to-shell (suspend the TUI, run an interactive shell, resume).

| Key | Type | Default | Description |
|---|---|---|---|
| `command` | string | `""` | Optional shell command/argv to run instead of an interactive shell. Empty uses `$SHELL`, falling back to `bash`. Setting this forces the one-shot shell even when `persistent` is `true` (a custom command is incompatible with the persistent session). |
| `sync_cwd_on_return` | bool | `true` | Navigate the active panel to the shell's working directory after returning from the shell. |
| `persistent` | bool | `true` | Keep one Midnight-Commander-style shell session alive across repeated shell toggles, instead of starting a new shell each time. Linux only; falls back to the one-shot shell elsewhere or if the persistent session can't start. |
| `terminal_panel_height` | int | `10` | Row count of the embedded terminal panel's content area, excluding the separator row (minimum 3). |

## `[meta]`

Meta column command definitions (custom computed panel columns).

| Key | Type | Default | Description |
|---|---|---|---|
| `file` | string | `""` | Path to the global `meta.toml`. Empty uses `<config dir>/meta.toml`. |
| `local_names` | array of strings | `["meta.toml"]` | Basenames probed in the active panel directory (for a per-directory meta file) before falling back to the global file. |
| `default_entry_workers` | int | `2` | Number of concurrent background workers used per meta column entry that doesn't specify its own worker count (clamped 1–64). |

## `[pools]`

Worker pool definitions (used by "run for each" operations).

| Key | Type | Default | Description |
|---|---|---|---|
| `file` | string | `""` | Path to the global `pools.toml`. Empty uses `<config dir>/pools.toml`. |

## `[compare]`

Twin-panel directory compare (content hash diff).

| Key | Type | Default | Description |
|---|---|---|---|
| `hash_concurrency` | int | `4` | Number of files hashed in parallel during a compare. |
| `read_buffer_kib` | int | `256` | Per-worker read buffer size, in KiB, used while hashing. |
| `max_hash_bytes` | int64 | `0` | Cap on bytes read per file when hashing. `0` means unbounded (whole file is hashed). |
| `stay_on_volume_default` | bool | `true` | Default state of the "stay on volume" option when opening compare. |

## `[dedup]`

Find-duplicates scan (within a single directory).

| Key | Type | Default | Description |
|---|---|---|---|
| `hash_confirm_bytes` | int64 | `34359738368` (32 GiB) | Pause and ask for confirmation before hashing if the total size of hash candidates exceeds this. `0` disables the confirmation gate. |
| `file_progress_bytes` | int64 | `268435456` (256 MiB) | Show a per-file progress bar in the scan dialog for files at or above this size. `0` disables the per-file bar. |
| `chunk_bytes` | int64 | `33554432` (32 MiB) | Compare same-size files this many bytes at a time, stopping as soon as content diverges. `0` disables chunked comparison. |
