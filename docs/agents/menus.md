# Leader menu strip

The bottom-docked strip behind the Esc menu, F2 user menu, `"` copy-path menu, `:` preview menu and Alt-G git filter menu, plus the per-view `:` menus. (Pulldown menus and the F9 bar are in `architecture.md`.)

## Shared strip

`internal/ui/leader_menu_render.go` (`DrawLeaderMenu`), state in `leader_menu_state.go` (`LeaderMenuState`). Callers:

| Menu | Action / key | Code |
|------|--------------|------|
| Built-in (grouped, multi-column, direct-key hints) | `app.leader-menu` (Esc) | `internal/app/leader_menu.go` |
| F2 user menu (`menu.toml`) | `app.user-menu` | `internal/app/user_menu.go` |
| Copy-path | `"` | |
| Fullscreen preview | `:` (`preview.menu`) | `internal/app/preview_menu.go`, `preview.md` |
| Git filter | Alt-G (`panel.git-filter-menu`) | `internal/app/git_filter_menu.go` |

- `LeaderMenuState.UserMenu` / `CopyMenu` / `PreviewMenu` flag the active caller (all false for the git filter menu). `LeaderMenuItem.GroupTitle` marks a group header; only the built-in menu and per-view menus set it, the F2, preview and git filter menus are flat. The Esc menu's Tools group lists **User menu** (`X`), which closes it and opens F2.
- **Activation**: a plain letter runs the entry. `leaderMenuKeys` resolves pinned `key` or an auto mnemonic, reserving every pinned key first so an auto letter never collides regardless of order; pinned keys match case-sensitively, auto letters case-insensitively. `handleLeaderMenuKey` dispatches through `App.leaderMenuOnActivate`, closing the strip first (`activateLeaderMenu`). Esc closes the strip (in an F2 submenu it steps back one level). F9 in the F2 menu edits `menu.toml` and reloads (`editUserMenuConfigFromDialog` / `reloadLeaderMenu`, `internal/app/dialog_config_edit.go`).
- **Direct-key hints** (`[ui].leader_menu_show_direct_keys`, F3 toggles, `toggleLeaderMenuDirectKeys`) apply to the Esc menu (looked up in `Global`) and the preview menu (`[preview]` overlay via `refreshLeaderMenuDirectKeys`); not to the F2 or copy-path menus.
- Panel chrome stays active/inactive underneath (`Model.PanelsChromeBlocked` excludes `LeaderMenu.Open`) but the cursor-row highlight dims (`CursorRowActive = FileListFocus && !LeaderMenu.Open`).
- **Git filter menu** (`openGitFilterMenu`): five flat rows (No filtering / Staged / Unstaged / Tracked / Untracked) via `openLeaderMenuStrip`; activation calls `panel.State.SetEntryFilter` on the active panel. Filter framework: `panels.md`.

## F2 user menu and submenus

- `openUserMenu` resolves the `menu.toml` path and calls `loadUserMenuVisible` (also used by F9 `reloadLeaderMenu`): `usermenu.LoadFile`, `MenuFile.ValidatePoolRefs`, `usermenu.FilterVisible`, emitting critical/warn messages itself. `openUserMenuLevel` is the single place any level is shown (top, submenu, back, reload) and therefore the single place `App.userMenuWarnings` (`MenuFile.Warnings`) is toasted at `MessageUrgencyWarn`, so warnings reappear each time the menu is displayed.
- A table with nested child tables instead of `command` is a submenu (`usermenu.MenuEntry.IsSubmenu`), suffixed with `Theme.IconTreeExpand`. Picking it pushes the current list onto `App.userMenuStack` (depth unbounded, file-header order); picking a leaf clears the stack and runs `runUserMenuEntry`.
- `usermenu.back` (`[dialog.user_menu]`, `keys.UserMenu`; Left/Backspace) pops one level (no-op at top); Esc closes everything (`closeUserMenu`).
- `key=` and `default = true` uniqueness are per level. A duplicate `key=` within a level is not fatal: the first claimant keeps it, later ones get auto letters, and `MenuFile.Warnings` records each duplicate. Entry fields (`command`, `when`, `run_for_each`, `pool`, `interactive`, `detach`, `background`, `toast`): `commands.md`.

## Per-view leader menus (Compare / Dedup / Jobs / Commands / Messages)

- `:` in these views opens a menu scoped to the view's own actions on the same plumbing (`LeaderMenuState`, `openLeaderMenuStrip`, `DrawLeaderMenu`). Each view's key handler (`handleCompareViewKey`, `handleDedupViewKey`, `handleMessagesViewKey`; `HandleJobsViewKey` in `apphandler/jobs`; `HandleViewKey` in `apphandler/commands`, both via `Host.ToggleLeaderMenu`) checks `keymap.ActionAppLeaderMenu` right after its F9 check and calls `toggleBuiltinLeaderMenu`, which branches on `Model.ViewMode`: browser -> `openBuiltinLeaderMenu`, otherwise `openViewLeaderMenu`.
- `openViewLeaderMenu` maps `ViewMode` to a `keymap.HelpViews` bit (`helpkeys.ViewMask`) and builds entries via `Bundle.LeaderMenuEntriesForView`. Membership is `leaderMenuViewSpecs` in `internal/keymap/leader_menu.go` (parallel to `leaderMenuGroupActions` for the browser menu). Every per-view menu has a shared App group (Quit) and Display group (links to other aux views) plus its own groups (Compare's File, Selection, Compare and Add mark; Jobs' Queue; Commands; Messages; Dedup's Tree, Selection (range, invert, unselect all) and Duplicates). `ActionAppUserMenu` is omitted from per-view App groups since F2 only opens from the browser. Each spec's `column` map puts the view's own groups first and Display above App, rightmost, mirroring the Esc menu. An action that also appears in the browser leader menu sits under the same group title there (Compare's Delete under File with the Selection group in the leftmost column, the Compare group to their right, Pin under Add mark above Display/App; Help `?` in App).
- Close is always `x` (`jobs.close`/`messages.close`/`commands.close`); Clear is always `c` (`jobs.clear-finished`/`messages.clear`).
- Activation reuses `activateHelpAction` (`internal/app/help.go`), the same per-view dispatch as F1 help (`activateCompareHelpAction`, `activateDedupHelpAction`, `activateJobsHelpAction`, `activateCommandsHelpAction`, `activateMessagesHelpAction`), so the action runs through that view's own controller. `openLeaderMenuDispatch` takes the activation callback so the browser/copy menus (using `dispatchActionLikeKeyboardShortcut`) and per-view menus share opening code. The strip has no title row; the empty/hidden-entries warnings use `helpContext.menuTitle` (e.g. `"Compare — Actions"`, `helpContextFor`).
- **Same action, same key**: a selection action uses one F9 menu letter in every menu (range `k`, select group `g`, unselect group `n`, unselect all `u`, invert `I`), matching its leader letter and chord; `TestMenuShortcutsConsistent` enforces it.
- **Letter scope**: a keypress resolves only against the open menu's items, so one letter may mean different actions in different views (Compare `c` = Close, Dedup `c` = Collapse). `ActionSpec.LeaderKey` is a single global field per action, so `validateLeaderKeysPerScope` enforces uniqueness separately for the browser menu and each per-view menu, and `Bundle.ActionForLeaderKey` restricts its reverse lookup to the browser menu's action set (an unscoped lookup would be ambiguous).
- Every `dedup.*`/`compare.*`/`jobs.*` action keeps its own overlay chord (`DefaultJobsOverlayKeys`, `DefaultCompareOverlayKeys`, `DefaultDedupOverlayKeys`; Dedup list in `panels.md`) in addition to the `:` menu, F9 menu and (in vi-motion) the leader letter. Letters and menus are convenience paths layered on the chord. Global `[main]` defaults and shared panel actions have no per-view letter, and bare letters are unsafe on panels because of type-to-filter.

## Vi-motion in the auxiliary views

`ui.Model.ViMotionMode` (`internal/ui/render.go`) is one global bool, toggled only from the browser (`panel.vi-motion-toggle`, `internal/app/input.go`). It stays on when opening the five views, whose key handlers read it and apply:

- **hjkl**: `keymap.RemapViMotionKey` (`internal/keymap/chord.go`) turns plain h/j/k/l into arrow events before the rest of the handler (after sub-dialog early-returns; for Jobs, only while the blocker/conflict pane lacks focus). Each view's existing arrow handling then applies unchanged (including Dedup's overlay-shadowed Left/Right collapse/expand).
- **Direct-fire letters**: `Bundle.ActionForLeaderKeyInView(r, vm)` (`internal/keymap/bundle.go`) is the per-view reverse lookup over `leaderMenuViewSpecs[vm]`. `App.dispatchLeaderLetterDirectFire` (`internal/app/leader_menu.go`) checks `ViMotionMode` plus a plain rune and runs the hit through `activateHelpAction` instead of opening the menu. Jobs and Commands reach it through `Host.DispatchLeaderLetter` (`ShellHost`, `appShellHost` in `internal/app/shell_host.go`). hjkl is consumed first and always wins a mnemonic clash: in Commands bare `k` moves the selection, so Kill is reached via `:`, F9 or `S-F8`.
- Esc is unaffected: each view's own hardcoded `tcell.KeyEsc` case (or Compare/Dedup overlay Esc-close) closes it; the toggle is never intercepted there.
- **Footer**: `viMotionFooterKeys` (`internal/app/input.go`, generic over `menu.FunctionKey.ActionID`) is applied to all five footers via `App.viMotionAuxiliaryFooterKeys`, wrapping `auxiliaryViewFooterKeys` as `activeFooterKeys` wraps the browser's. Every entry in `menu.FunctionKeysJobs`/`FunctionKeysCommands`/`FunctionKeysMessages` (`internal/ui/menu/fkeys.go`) and `compareViewFooterKeys`/`dedupViewFooterKeys` must carry an `ActionID`; entries with no `LeaderKey` (F9 Menu) are left untouched.
