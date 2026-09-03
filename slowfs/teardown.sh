#!/usr/bin/env bash
# Unmounts ./slowfs/mount. Pass --clean to also delete the generated corpus (./slowfs/data).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MOUNT_DIR="$SCRIPT_DIR/mount"

if mountpoint -q "$MOUNT_DIR"; then
    fusermount3 -u "$MOUNT_DIR" 2>/dev/null || fusermount -u "$MOUNT_DIR" 2>/dev/null || umount "$MOUNT_DIR"
    echo "unmounted $MOUNT_DIR"
else
    echo "$MOUNT_DIR is not mounted"
fi

rm -f "$SCRIPT_DIR/.slowfs.pid"

if [ "${1:-}" = "--clean" ]; then
    rm -rf "$SCRIPT_DIR/data"
    echo "removed $SCRIPT_DIR/data"
fi
