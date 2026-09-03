#!/usr/bin/env bash
# Generates a synthetic directory corpus under ./slowfs/data and mounts it through slowfs
# (github.com's FUSE latency simulator at /home/paranoidi/projects/slowfs) at ./slowfs/mount,
# to reproduce and measure the select-all UI-freeze report locally on a repeatable slow
# "network mount"-like filesystem instead of a real Samba share.
#
# Usage: ./slowfs/setup.sh [metadata-op-time]
#   metadata-op-time: per-metadata-syscall latency slowfs injects (stat, readdir, ...).
#                      Defaults to 15ms, roughly a slow SMB round-trip.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="$SCRIPT_DIR/data"
MOUNT_DIR="$SCRIPT_DIR/mount"
CONFIG_FILE="$SCRIPT_DIR/config.json"
SLOWFS_BIN="/home/paranoidi/projects/slowfs/slowfs-bin"
METADATA_OP_TIME="${1:-15ms}"
DIR_COUNT=3000

WORDS=(alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike november
       oscar papa quebec romeo sierra tango uniform victor whiskey xray yankee zulu)

if [ ! -x "$SLOWFS_BIN" ]; then
    echo "slowfs binary not found at $SLOWFS_BIN (build it: cd /home/paranoidi/projects/slowfs && go build -o slowfs-bin .)" >&2
    exit 1
fi

mkdir -p "$MOUNT_DIR"

if mountpoint -q "$MOUNT_DIR"; then
    echo "$MOUNT_DIR is already mounted; run ./slowfs/teardown.sh first" >&2
    exit 1
fi

if [ ! -d "$DATA_DIR" ] || [ -z "$(ls -A "$DATA_DIR" 2>/dev/null)" ]; then
    echo "generating corpus: $DIR_COUNT directories under $DATA_DIR ..."
    mkdir -p "$DATA_DIR"
    for i in $(seq 1 "$DIR_COUNT"); do
        w1=${WORDS[RANDOM % ${#WORDS[@]}]}
        w2=${WORDS[RANDOM % ${#WORDS[@]}]}
        d="$DATA_DIR/$(printf '%04d' "$i")_${w1}_${w2}"
        mkdir -p "$d/sub"
        truncate -s 10K "$d/file1.dat" "$d/file2.dat" "$d/sub/file1.dat" "$d/sub/file2.dat"
    done
    echo "corpus generated: $DIR_COUNT top-level directories, each with 2 files + 1 subdirectory (2 files)."
else
    echo "corpus already exists at $DATA_DIR, reusing (delete it to regenerate)."
fi

cat > "$CONFIG_FILE" <<EOF
[
  {
    "Name": "slow-network",
    "SeekWindow": "64KiB",
    "SeekTime": "10ms",
    "ReadBytesPerSecond": "10MiB",
    "WriteBytesPerSecond": "5MiB",
    "AllocateBytesPerSecond": "1GiB",
    "RequestReorderMaxDelay": "500us",
    "FsyncStrategy": "dumb",
    "WriteStrategy": "simulate",
    "MetadataOpTime": "$METADATA_OP_TIME"
  }
]
EOF

echo "mounting $DATA_DIR at $MOUNT_DIR (metadata-op-time=$METADATA_OP_TIME) ..."
nohup "$SLOWFS_BIN" \
    --backing-dir="$DATA_DIR" \
    --mount-dir="$MOUNT_DIR" \
    --config-file="$CONFIG_FILE" \
    --config-name="slow-network" \
    > "$SCRIPT_DIR/slowfs.log" 2>&1 &
echo $! > "$SCRIPT_DIR/.slowfs.pid"

for _ in $(seq 1 50); do
    mountpoint -q "$MOUNT_DIR" && break
    sleep 0.1
done

if ! mountpoint -q "$MOUNT_DIR"; then
    echo "mount did not come up; see $SCRIPT_DIR/slowfs.log" >&2
    exit 1
fi

echo "mounted at $MOUNT_DIR (pid $(cat "$SCRIPT_DIR/.slowfs.pid")). Run ./slowfs/teardown.sh to unmount."
