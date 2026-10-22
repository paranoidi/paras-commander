#!/usr/bin/env bash
# Recreate the demo HOME used by the docs tapes: fixture.sh <dir>
# Keep <dir> outside the repo so git ignore marks don't show in the panels.
set -euo pipefail
dir=${1:?usage: fixture.sh <dir>}
mkdir -p "$dir/.config/pc"
w="$dir/work"
mkdir -p "$w"/{harbor-lantern/{charts,logs},quiet_meadow/{notes,photos},velvet-orbit/{clips,backup},copper-garden/src}

# blob <bytes> <seed>: deterministic content, differs per seed
blob() { yes "$2" | head -c "$1" || true; }  # yes dies of SIGPIPE
f() { blob "$2" "$3" > "$1"; }

f "$w/harbor-lantern/readme.md" 1800 readme
f "$w/harbor-lantern/charts/tide-table.pdf" 48000 tide
f "$w/harbor-lantern/logs/amber-river.txt" 5200 amber
f "$w/quiet_meadow/notes/quiet_meadow_notes.md" 2400 meadow
f "$w/quiet_meadow/photos/silver-fox.jpg" 91000 fox
f "$w/quiet_meadow/photos/maple-dawn.jpg" 73000 maple
f "$w/velvet-orbit/clips/velvet-orbit.mkv" 2400000 orbit
f "$w/velvet-orbit/clips/frozen-canyon.mkv" 1500000 canyon
f "$w/velvet-orbit/backup/pine-archive.tar.gz" 640000 pine
f "$w/copper-garden/src/main.go" 3100 main
f "$w/copper-garden/src/gentle_wind.go" 2700 wind
truncate -s 8M "$w/velvet-orbit/sparse-image.bin"
truncate -s 300K "$w/copper-garden/hollow-cache.dat"

# duplicate groups: identical bytes in different directories
blob 120000 dupA | tee "$w/harbor-lantern/charts/coastal-map.pdf" "$w/quiet_meadow/notes/coastal-map-copy.pdf" "$w/velvet-orbit/backup/coastal-map.pdf" >/dev/null
blob 74000 dupB | tee "$w/quiet_meadow/photos/lake-sunrise.jpg" "$w/velvet-orbit/backup/lake-sunrise.jpg" >/dev/null
blob 4200 dupC | tee "$w/harbor-lantern/logs/cedar-trail.txt" "$w/copper-garden/src/cedar-trail.txt" "$w/quiet_meadow/notes/cedar-trail-old.txt" "$w/velvet-orbit/clips/cedar-trail.txt" >/dev/null
blob 950000 dupD | tee "$w/velvet-orbit/clips/misty-valley.mkv" "$w/velvet-orbit/backup/misty-valley.mkv" >/dev/null

# git repo for the git status column: one committed, one modified, one ignored, one new
g="$w/copper-garden"
echo "hollow-cache.dat" > "$g/.gitignore"
git -C "$g" init -q
git -C "$g" -c user.name=demo -c user.email=demo@example.invalid add .gitignore src
git -C "$g" -c user.name=demo -c user.email=demo@example.invalid commit -qm init
echo "// tweak" >> "$g/src/main.go"
f "$g/src/bright_lake.go" 1900 lake

find "$dir" -exec touch -h -d '2026-01-01 12:00' {} +
