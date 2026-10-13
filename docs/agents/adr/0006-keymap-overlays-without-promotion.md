# Shortcut tables are independent overlays with no cross-map promotion

Each view/dialog has its own `keybindings.toml` table that resolves before `[main]` while focused; `buildBundle` never copies bindings between tables, and each table validates its allowed action ids. A shared action bound in two dialogs is configured twice. This keeps every chord's scope explicit and user-overridable at the cost of some repetition in config.
