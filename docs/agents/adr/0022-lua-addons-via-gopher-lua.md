# Scriptable addons are sandboxed Lua run by gopher-lua, one state per addon goroutine

Status: proposed (not implemented). Extensions compiled into pc (ADR 0024) are the first extension mechanism; Lua addons would register into the same registry.

Addons are Lua 5.1 scripts run in-process by `github.com/yuin/gopher-lua`. Each addon owns one `*lua.LState` on its own goroutine, sees only a `pc` API module (no `os`, `io` or native loading), and reaches the app only through a `Host` interface in `internal/addon`. Addon-defined actions join the keymap as ordinary action ids, so they bind in `keybindings.toml` and show in help and menus like built-ins.

## Context

- Users want behaviour the user menu and `cmdmacro` shell commands cannot express: logic over the selection, custom previewers and columns, reactions to navigation.
- The release binary is `CGO_ENABLED=0`. That rules out LuaJIT, CPython, V8 and Go's `plugin` package (which also needs a binary built with identical toolchain and dependency versions).
- UI-thread IO is forbidden (repo performance rule), and derived UI state is settled only in the post-event reconciler (ADR 0002). An addon must not touch `Model` from another goroutine or block the event loop.
- File access must work on every `fsbackend` (local and SFTP), not just the local disk.
- Alternatives considered:
  - **Starlark**: hermetic and deterministic, but built for configuration: frozen globals, no exceptions, a poor fit for stateful event hooks.
  - **goja (JavaScript)**: heavier runtime and startup cost; async idioms that users expect (promises) map badly onto a single-threaded embedded VM.
  - **WASM via wazero**: any source language and strong isolation, but needs a custom ABI and per-language guest tooling before the first addon exists.
  - **Yaegi (Go interpreter)**: language-compatibility gaps and no practical sandbox.
  - **External processes over JSON-RPC**: language-agnostic and crash-isolated, but slow round-trips and process lifecycle management. Kept open as a later second tier for heavyweight addons, not the first.
- Lua is the established extension language for comparable tools (Yazi, Neovim, mpv, WezTerm), and gopher-lua is pure Go.

## Decision

- **Discovery.** Each addon is a directory `$XDG_CONFIG_HOME/pc/addons/<name>/` with an `init.lua` entry point. Addons load asynchronously after startup; a failing addon is reported in the Messages view and skipped, never aborting startup. Enablement and per-addon settings live in the main config (`[addons]`), with defaults in `internal/config/builtin.go` (ADR 0007).
- **Sandbox.** Only the `base` (minus `dofile`, `loadfile`), `table`, `string` and `math` libraries are opened. `os`, `io`, `debug` and `package.loadlib` are absent; `require` resolves only inside the addon's own directory. Every capability (filesystem, running commands, clipboard) comes from the `pc` module and goes through the same code paths as built-in features.
- **Threading.** One `LState` per addon, owned by one goroutine that drains a request queue (hook calls, action invocations). Calls into an addon carry an immutable snapshot of the relevant state (active panel path, cursor entry, selection), never live `Model` pointers. Every call runs under a context with a deadline (`LState.SetContext`); an addon that exceeds it is cancelled and its call reported as failed.
- **Effects back into the app.** An addon never mutates UI state directly. `pc.*` calls that change state (navigate, select, show message, invoke an action) are queued as commands and delivered to the event loop as one coalesced interrupt payload (`host.WakeCoalescer`); the event loop applies them and the reconciler runs as for any other event.
- **Boundary.** `internal/addon` owns the runtime and the `pc` module, and depends on a narrow `Host` interface implemented by `internal/app` with forwarders, exactly like feature handlers (ADR 0001). `internal/addon` never imports `internal/app` or `internal/ui`.
- **Actions and keys.** An addon registers actions with ids `addon.<name>.<action>` plus title, section and optional default keys. Registrations append `keymap.ActionSpec` entries before the keymap bundle is built, so `keybindings.toml` binds them, validation knows them and F1 help lists them. Addon actions are allowed only in `[main]` and the user menu; dialog and view overlay tables keep their fixed allowed-id lists (ADR 0006).
- **IO and long work.** `pc.fs.*` goes through `fsbackend`, so addons work on SFTP panels. Operations that copy, move or delete are submitted as jobs (`pc.jobs.submit`) and get progress, conflict handling, rate limits and quit safety from the job queue; external programs run as commands in the Commands view.
- **Initial hook surface.** Custom actions, user-menu entries, previewers by file type, extra panel columns, and notifications for directory change and selection change. Hooks that run per frame or per entry (columns) are fed from cached results computed on the addon goroutine, never called synchronously from rendering.

## Consequences

- Addons inherit the single-binary, static build; no runtime to install.
- Lua 5.1 semantics only (no integers, no `goto`, no 5.3+ library changes). gopher-lua is slower than PUC Lua, which is acceptable for hook-sized work but not for per-file processing of huge trees; that work should be a job or a command.
- Snapshot-in, command-out means addons always act on slightly stale state; a command whose target changed (e.g. the panel navigated away) is validated when applied and dropped with a message.
- The `pc` module is a public API: renaming or removing its functions breaks user addons. Within the prototyping phase it changes freely like everything else; once stabilised it needs versioning.
- Panel columns and previewers from addons are asynchronous by construction, so they render a placeholder until the addon answers.
- A misbehaving addon can still spin its own goroutine or exhaust memory until its deadline; gopher-lua offers no memory cap, so the sandbox protects the filesystem and UI thread, not resources.
