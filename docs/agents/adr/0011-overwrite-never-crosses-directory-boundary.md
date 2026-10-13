# Overwrite never crosses a directory boundary

Overwrite only ever replaces a non-directory with a non-directory. When exactly one side of a conflict is a directory (a real directory, or a symlink to one), or the sides are a directory and a symlink to a directory, the collision is a **mismatch**: it can only be skipped, renamed (keep both) or canceled. Local overwrite is a plain `rename(2)`, which atomically replaces the target; only SFTP renames the existing destination aside to `.paras-move-stash-N` first, because SFTP v3 rename fails when the target exists.

## Context

- Overwriting must never destroy a tree the user did not ask to lose: a file landing on a directory used to delete the whole directory.
- Directory onto directory merges at every depth unless the user skips or cancels, so the merge path never reaches the overwrite question.
- A symlink is followed only to classify it; nothing is merged through one.
- The rule has to hold for copy, move and flatten, so it lives in the shared conflict path rather than in each operation.

## Decision

- `ops.FileConflictFacts` carries the kind of both sides (`SourceTarget`, `DestTarget`); `OverwriteUnsafe()` is the single predicate. `StatFileConflictFacts` (local) and `statConflictFacts` (backend) fill it.
- `resolveConflict` (`internal/ops/conflict_prep.go`) is the choke point: an `ActionOverwrite` on a mismatch is converted to skip, whatever the resolver returned.
- The conflict dialog and jobs-view panel name the destination kind (`Destination is a directory` / `Destination is a file`, no item counts) and offer Rename, Rename All, Skip, Skip All, Cancel. Remembered Skip all and Rename all apply silently; remembered Overwrite all and every advanced rule set are ignored for a mismatch and the user is asked.
- A directory item meeting a non-directory in the copy plan is resolved the same way (`copyDirItem`); skipping drops its whole subtree, renaming re-roots it under the sibling name.
- Local move overwrite renames the source onto the destination. SFTP move overwrite parks the original, renames in, and drops the original, restoring it on failure. A crash can leave a stash name behind on SFTP.
- Completed earlier renames are never rolled back (see ADR 0019).
