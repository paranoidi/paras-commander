# fileindex is a built-in feature linked as a Go module; no extension registry yet

Status: accepted.

pc integrates fileindex (a SQLite content-hash index of Files, Instances, Dirs and Tags) as an ordinary built-in feature: `github.com/paranoidi/fileindex` is a Go module dependency, `internal/fileindex` holds the backend, column provider and job runner, and `internal/apphandler/fileindex` owns its UI behaviour behind a `Host` (ADR 0001). Its actions are built-in action specs like any other. Three pieces are built general because they are useful on their own: task jobs, Go column providers and a third path scheme. An extension registry is deferred until a second extension or Lua addons (ADR 0022) exist.

## Context

- Users want to browse the fileindex index as a filesystem, read and write its data (scan, prune, tags, index-only moves), see tags as panel columns, and watch long operations such as scan in the jobs view.
- fileindex is a separate Go project with its own CLI. It exposes `db`, `scan` and `config` as public packages, scan as a library call with a progress interface and cancellation, and index-only operations as functions.
- The release binary is `CGO_ENABLED=0`. That rules out Go's `plugin` package (cgo, identical toolchain and dependency versions, Linux/macOS only).
- Alternatives considered:
  - **An extension registry** (`internal/extension`: actions registered at startup, default keys dropped when taken, a generic Host): keeps fileindex behind one interface and prepares for Lua, but with a single extension it is a registry with one entry, plus a dynamic keymap layer that built-in actions do not need.
  - **Lua addon** (ADR 0022): Lua is not in use, and a sandboxed addon cannot open SQLite.
  - **`hashicorp/go-plugin` or a long-lived `fileindex serve` subprocess**: version-decoupled and crash-isolated, but a protocol, process lifecycle and an RPC per listing before the first feature works.
  - **CLI invocation per call** (meta-column and user-menu commands): too slow for listing; no machine-readable output.
- The job queue knows only file transfers (copy, move, flatten, delete, extract, rename, undo); the meta-column runner runs only shell commands; `pathloc` knows only `file` and `sftp`.

## Decision

- **Dependency.** pc requires `github.com/paranoidi/fileindex` and imports its `db`, `scan` and `config` packages. It opens the database lazily, off the UI thread, and checks `RequireSchema`; a missing database makes pc ask whether to create an empty one (fileindex's `Init` on a new file, only after the user agrees), and a mismatched one disables fileindex actions with one Messages error. pc never migrates the database. Scan include/exclude rules come from fileindex's own `config.ini` (read without writing), so the CLI and pc index the same files. pc's `[fileindex]` config section holds the database path and worker count, with defaults in `internal/config/builtin.go` (ADR 0007).
- **Actions.** `fileindex.*` actions are entries in `keymap.DefaultActionSpecs()` with section "fileindex", a leader-menu group, and default keys chosen like any built-in. They are `[main]` actions handled by `internal/apphandler/fileindex`; all database work runs on goroutines and returns through wake payloads (ADRs 0002, 0008). Tag prompts are declarative dialog specs (ADR 0020).
- **Task jobs.** `jobs.TypeTask` carries a title and `Run(ctx, emit)`. It has no pre-scan, conflicts, undo or retry, reuses the existing progress fields and events, and takes a lease named by the task (one `fileindex` task at a time) instead of the transfer lease. `Sources` holds the paths it works on, so path marks and volume contention apply. Task jobs cannot be paused; cancel cancels `ctx`. fileindex scan and prune run as task jobs, with fileindex's scan reporter mapped onto job events.
- **Column providers.** The meta-column runner accepts a Go provider, called once per directory listing on a worker goroutine, beside shell commands. Results use the meta-column cache, so sort, filter, carousel and layout are shared. The fileindex tags column is a provider and works on local panels as well as index views.
- **`fileindex://` scheme.** `pathloc` gets a third scheme: `fileindex://` plus an absolute host path, with file-scheme `Base`/`Parent` rules. `IsRemote()` is true for it, so git, folder colours, disk usage and the embedded terminal stay off. The backend lists and stats from the index; `OpenRead` opens the real file when it exists, so copying out goes through `ops`; `Rename` is an index-only move and `Remove` an index-only remove tree, and their dialogs say so. Mkdir, symlink and writes return a read-only error.
- **Deferred registry.** When a second integration or Lua addons arrive, an extension registry is extracted from what fileindex then uses: action specs registered before the keymap is built, default keys dropped when taken, and the task-job, column-provider and scheme contracts above.

## Consequences

- fileindex ships in every pc build and adds `modernc.org/sqlite` (pure Go) to the binary size. pc's minimum Go version follows fileindex's `go` line.
- fileindex's public packages are an API pc depends on; a schema change in fileindex needs a matching pc release.
- Task jobs, Go column providers and the extra scheme are available to any built-in feature, not only fileindex.
- Index-only rename and delete reuse file-operation dialogs whose wording assumes disk changes; those dialogs carry a per-scheme note.
- SQLite in WAL mode serves reads while a scan writes; a tag write waits behind a scan's batch commit (busy timeout), which is acceptable off the UI thread only.
- The index does not follow renames, moves and deletes that pc makes on disk. Feeding `fschange` reports (ADR 0023) to fileindex would, but `fschange` supports one sink per context today, so it needs fan-out first.
- Extracting the registry later means moving fileindex's action specs out of the static list; that cost is accepted in exchange for not building a registry with one user.
