# Move renames each source first and merges directories child by child

Move handles each top-level source on its own, in the style of mc: try the rename, merge a directory into an existing directory child by child, and fall back to copy and delete for a source only when a rename cannot be used (cross-device or cross-host). Earlier renames are never rolled back when a later source fails or needs the fallback.

## Context

- A same-device move must be O(1) however large the tree is, so there is no pre-scan and no upfront totals; progress counts top-level sources.
- A directory meeting a directory of the same name is merged completely unless the user skips or cancels; only colliding items are listed and asked about.
- Rolling back renames would need a journal and could itself fail; the user can see exactly what moved.
- Overwrite rules for the colliding items are in ADR 0011.

## Decision

- `ExecuteMove` resolves each source's destination, then `moveOne`: a directory onto a real directory merges via `mergeMoveDir` (recursing through `moveOne`, removing the source directory once it is empty); anything else goes through the conflict resolver and `RenameFastPathCtx`.
- A source whose rename is not possible is planned, disk-checked, copied and removed on its own (`moveCopyFallback`, `finishMoveCopyPhase`); the same-host SFTP rename is used when both sides share a host.
- Skipped items keep their source and parent directories in place without error.
