# Status commands

Status commands are small shell commands whose output is shown at the top-left of the menu bar, for example the load average or memory use. Any number of them can be shown at once, or none.

```
 0.52 0.48 0.40 · 5.2G/31G · File  Command  Display  Options
```

## Choosing what is shown

Open **Display → Status commands** (`ui.status-commands`) to get a checkbox list of every available status command. Check the ones to show and confirm with OK. The choice is saved to the `show` line of `status.toml` and takes effect immediately. Commands that stay checked keep their order; newly checked ones are added after them.

## Built-in commands

| Name | Label | Shows | Interval |
|---|---|---|---|
| `loadavg` | Load average | 1, 5 and 15 minute load average from `/proc/loadavg` | 3 s |
| `memory` | Memory used | used / total RAM from `/proc/meminfo`, e.g. `5.2G/31G` | 5 s |
| `battery` | Battery | charge of the first battery, e.g. `87%`; hidden on machines without one | 30 s |

## status.toml

Status commands live in `status.toml` in the config directory (`pc --config-stub` writes a commented example). The file is optional: without it the built-ins are available and none is shown.

```toml
show = ["loadavg", "clock"]

[[command]]
name = "clock"
label = "Clock"
command = "date +%H:%M"
interval_ms = 10000
max_width = 5
```

- `show` lists the names drawn in the menu bar, in order. The Status commands dialog rewrites this line and leaves the rest of the file untouched. Unknown names are ignored.
- Each `[[command]]` adds a command. Using a built-in's name replaces that built-in (give all fields again).
- `command` runs via `sh -c`, with no `%f`-style macro expansion. Only the first line of its output is shown, ellipsized to `max_width` columns.
- Empty output, a non-zero exit or a run longer than 10 s hides that segment until the next run.

Field reference: [`status.toml`](config.md#statustoml).

## Appearance

The commands' output should not contain a separator: the menu bar draws one after each segment. The separator glyph is `[icons.menu.status] separator` and its color `menu.status.separator` (dark grey by default); the text uses `menu.status`. See [Themes](themes.md).
