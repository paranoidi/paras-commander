# Extensions are Go packages compiled into pc, registered from init

Status: proposed (not implemented).

An extension is a Go package under `internal/ext/<name>` that registers itself with `internal/extension` from `init()` and is blank-imported by `cmd/pc`, the same way `fsbackend/file` registers the local backend. An extension contributes actions, a filesystem backend, panel columns and background jobs, and reaches the app only through an `extension.Host`. The first extension is fileindex (a SQLite content-hash index of files, instances, dirs and tags), imported as a Go module.

## Context

- fileindex is a separate Go project with its own CLI. Users want to browse its index as a filesystem, read and write its data (scan, prune, tags, index-only moves), see tags as panel columns, and watch long operations such as scan in the jobs view.
- ADR 0022 proposes sandboxed Lua addons. Lua is not in use yet, and a Lua addon cannot open SQLite (no `io`, no native loading), so fileindex would have to shell out per call or go through a new Host round trip per listing.
- The release binary is `CGO_ENABLED=0`. That rules out Go's `plugin` package (cgo, identical toolchain and dependency versions, Linux/macOS only).
- Alternatives considered:
  - **`hashicorp/go-plugin` (subprocess over gRPC)**: loadable at runtime and crash-isolated, but pulls in grpc/protobuf and costs an RPC per `List`. Too heavy for a single extension.
  - **Long-lived subprocess over stdio JSON lines** (`fileindex serve`): stdlib-only and version-decoupled, but needs a protocol and process lifecycle before the first feature works. It can be added later as an adapter behind the same Go interfaces.
  - **CLI invocation per call** (meta-column and user-menu commands): works today for tag columns and tag actions, but a process per row is too slow for listing, and the CLI has no machine-readable output.
- Today every extension point pc would need is static: `keymap.DefaultActionSpecs()`, `keymap.KnownActions`, `leaderMenuGroupActions`, the `dispatch` switch, the meta-column runner (shell commands only) and the job types (file transfers only).
- `keymap.Build` fails startup when two actions share a chord. A third-party default key must not be able to do that.

## Decision

- **Shape.** `internal/extension` owns the registry and the interfaces: `Register(Extension)` from `init()`, and an `Extension` that returns its actions, an optional `fsbackend.Backend`, and column providers. It imports neither `internal/app` nor `internal/ui`; `internal/app` implements `extension.Host` with forwarders (ADR 0001). Extensions are always compiled in, with no build tag.
- **Registration timing.** All registration finishes before `keymap.LoadFromPaths`, so `keybindings.toml` accepts extension action ids and validation knows them. Registering from `init()` guarantees this.
- **Actions.** Ids are `<extension>.<action>` (`fileindex.scan`, `fileindex.tag-add`). Each is a `keymap.ActionSpec` appended to the spec list, so F1 help lists it with `Section` = the extension's title. Each extension gets one leader-menu group with its title. Extension actions are allowed in `[main]` only; overlay tables keep their fixed allowed-id lists (ADR 0006).
- **Default keys never conflict.** Extension default chords and leader letters form their own layer, applied after the built-in defaults and the user's `keybindings.toml`. A chord or leader letter already bound in that scope is dropped and reported once in the Messages view; the action stays bindable. Extension defaults never take a key from a built-in action or a user binding, and `keymap.Build` never sees an extension conflict.
- **Running actions.** `dispatch` falls back to the extension registry for an unknown id. The handler runs on a goroutine with an immutable snapshot (both panel paths, cursor entry, selection), never live `Model`. Effects (navigate, message, refresh columns, open a prompt, submit a job) are queued to the event loop as a wake payload, and the post-event reconciler runs as for any event (ADR 0002). Prompts are declarative dialog specs (ADR 0020); extensions draw no UI of their own.
- **Long work is a job.** `jobs.TypeTask` carries a title and `Run(ctx, emit)`. It has no pre-scan, conflicts or undo, reuses the existing progress fields and events, and takes a lease named by the task (one `fileindex` task at a time) instead of the transfer lease. `Sources` holds the paths it works on, so path marks and volume contention apply. Task jobs cannot be paused; cancel cancels `ctx`. The jobs view, failure state and quit confirmation work unchanged.
- **Filesystem backend.** An extension may register one `pathloc` scheme. fileindex registers `fileindex://` with absolute host paths, so `Base`/`Parent` reuse the file-scheme rules. `List`/`Stat` read the index. `OpenRead` opens the real file when it exists, so copying out goes through the normal backend path in `ops`. `Rename` maps to an index-only move (`MoveInstance`/`MoveDir`), and `Remove` drops the entry from the index without touching disk; their dialogs say so. Mkdir, symlink and writes return a read-only error. `IsRemote()` is true for the scheme, so git, folder colours, disk usage and the embedded terminal stay off.
- **Columns.** A column provider is a Go function called once per directory listing, `func(ctx, dir, entries) map[path]string`, run by the meta-column runner beside shell commands. Its results go into the same cache, so sort, filter, carousel and layout behave as for meta columns. Providers run on local panels too, so fileindex tags show next to real files.
- **fileindex as a dependency.** fileindex exposes its index, scan and config packages outside `internal/` under the module path `github.com/paranoidi/fileindex`. Scan runs as a library call `scan.Run(ctx, db, roots, opts, reporter)`, where `reporter` is an interface; the CLI's terminal progress is one implementation and the pc task job is another. Every scan phase, prune included, stops when `ctx` is cancelled. pc opens the database with `RequireSchema` and refuses a version it does not know; only the fileindex CLI migrates. Scan include/exclude rules come from fileindex's own `config.ini`, so the CLI and pc index the same files. pc's own `[fileindex]` config section holds the database path and worker count, with defaults in `internal/config/builtin.go` (ADR 0007).
- **Relation to ADR 0022.** Lua addons, if built, register into this same registry with the same action, column and job contracts. Their asynchronous loading conflicts with registration before the keymap is built; ADR 0022 must resolve that before it is implemented.

## Consequences

- Extensions inherit the single static binary. Adding or upgrading one means rebuilding pc, and fileindex adds `modernc.org/sqlite` (pure Go) to the binary size.
- pc's minimum Go version follows the highest extension module's `go` line.
- fileindex's exported packages become an API that pc depends on; a schema change in fileindex needs a matching pc release.
- An extension runs with the app's full privileges and can crash the process; there is no sandbox. That is acceptable only because extensions are code reviewed into this repository.
- A user's binding wins over an extension default, and so does any built-in action, so an extension's advertised key may be missing; the dropped-key message is the only signal.
- SQLite in WAL mode serves reads while a scan writes; a tag write waits behind a scan's batch commit (busy timeout), which is fine off the UI thread and would not be on it.
- Index-only rename and delete in a `fileindex://` panel reuse file-operation dialogs whose wording assumes disk changes; those dialogs need a per-backend note.
- The index does not follow renames, moves and deletes that pc makes on disk. Subscribing an extension to `fschange` reports (ADR 0023) would fix that, but `fschange` supports one sink per context today, so it needs fan-out first.
