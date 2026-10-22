#!/usr/bin/env bash
# Recreate the demo HOME used by the docs tapes: fixture.sh <dir>
# Keep <dir> outside the repo so git ignore marks don't show in the panels.
# Theme: a personal media collection (trips, parties, board-game nights,
# aerial drone videos) plus a small photo-tools script repo.
set -euo pipefail
dir=${1:?usage: fixture.sh <dir>}
mkdir -p "$dir/.config/pc"
m="$dir/media"

# blob <bytes> <seed>: deterministic content, differs per seed
blob() { yes "$2" | head -c "$1" || true; }  # yes dies of SIGPIPE
f() { mkdir -p "$(dirname "$1")"; blob "$2" "$3" > "$1"; }

t="$m/trips/2025-07-lapland-hike"
f "$t/reindeer-crossing.jpg" 91000 reindeer
f "$t/campfire-at-dusk.jpg" 73000 campfire
f "$t/fell-summit-panorama.jpg" 128000 summit
f "$t/midnight-sun-lake.jpg" 86000 sun
f "$t/trail-map.pdf" 48000 trailmap
f "$m/trips/2025-12-lisbon-weekend/tram-28-uphill.jpg" 82000 tram
f "$m/trips/2025-12-lisbon-weekend/pasteis-tasting.jpg" 64000 pasteis
f "$m/trips/2025-12-lisbon-weekend/itinerary.md" 1800 itinerary
f "$m/parties/2025-06-midsummer/bonfire-wide.jpg" 97000 bonfire
f "$m/parties/2025-06-midsummer/sauna-jump.mp4" 1500000 sauna
f "$m/parties/2025-10-costume-party/best-costume-vote.jpg" 70000 vote
f "$m/parties/2025-10-costume-party/playlist.txt" 2300 playlist
f "$m/game-nights/2025-11-14/final-scoreboard.jpg" 52000 score
f "$m/game-nights/2025-11-14/table-setup.jpg" 66000 table
f "$m/game-nights/house-rules.md" 2600 rules
f "$m/aerial/coastline-flyover.mp4" 2400000 coast
f "$m/aerial/forest-lake-orbit.mp4" 1900000 orbit
f "$m/aerial/harbor-reveal-4k.mp4" 2100000 harbor
f "$m/aerial/raw/flight-log.csv" 5200 flightlog
truncate -s 8M "$m/aerial/raw/CLIP_0042.mp4"

# duplicate groups: the same file exported or backed up to several places
blob 120000 dupA | tee "$m/trips/2025-07-lapland-hike/group-photo-at-cabin.jpg" "$m/parties/2025-06-midsummer/group-photo-at-cabin.jpg" "$m/aerial/raw/group-photo-at-cabin.jpg" >/dev/null
blob 74000 dupB | tee "$m/game-nights/2025-11-14/winner-podium.jpg" "$m/parties/2025-10-costume-party/winner-podium.jpg" >/dev/null
blob 4200 dupC | tee "$m/trips/2025-07-lapland-hike/packing-list.txt" "$m/trips/2025-12-lisbon-weekend/packing-list.txt" "$m/parties/2025-06-midsummer/packing-list.txt" "$m/game-nights/packing-list.txt" >/dev/null
blob 950000 dupD | tee "$m/aerial/sunset-over-fjord.mp4" "$m/aerial/raw/sunset-over-fjord.mp4" >/dev/null

# disk usage: a screenful of drone flights with gradually shrinking sizes
# (sparse files; disk usage counts apparent size)
size=$((48 * 1024 * 1024 * 1024))
for name in arctic-coast-timelapse glacier-tongue-flyover volcano-crater-orbit \
    island-hopping-day2 fjord-ferry-chase lighthouse-storm-reveal \
    wedding-venue-sweep forest-canopy-dawn canyon-river-descent \
    city-rooftops-night ski-slope-follow surf-break-topdown \
    vineyard-rows-autumn harbor-regatta-start waterfall-pullback \
    dune-ridge-sunset cliff-diving-spot lake-ice-cracks \
    castle-ruins-orbit marathon-bridge-pass festival-crowd-overhead \
    sailboat-tracking beach-bonfire-night farm-fields-patchwork \
    bridge-construction-pan reindeer-herd-overhead mountain-hut-approach \
    kayak-trip-followcam summer-cottage-dock garden-party-overview; do
  mkdir -p "$dir/drone-archive/$name"
  truncate -s $((size * 3 / 5)) "$dir/drone-archive/$name/master-4k.mp4"
  truncate -s $((size * 2 / 5)) "$dir/drone-archive/$name/raw-footage.mp4"
  size=$((size * 85 / 100))
done

# compare: a camera-card import against its edited album, every result class
a="$dir/albums/lisbon-card-import" b="$dir/albums/lisbon-album"
for p in day1/tram-28-uphill.jpg day1/miradouro-view.jpg day2/belem-tower.jpg \
    day2/pasteis-tasting.jpg day3/alfama-alley.jpg day3/fado-bar.jpg; do
  f "$a/$p" 2600 "same-$p"; f "$b/$p" 2600 "same-$p"               # identical
done
f "$a/day1/sunset-river.jpg" 84000 river; f "$b/favorites/sunset-river.jpg" 84000 river  # relocated
f "$a/captions.txt" 3000 cap1;  f "$b/captions.txt" 3400 cap2          # differs
f "$a/day2/street-band.jpg" 900 band1; f "$b/day2/street-band.jpg" 950 band2
f "$a/day3/blurry-selfie.jpg" 2100 blurry                              # only left
f "$b/day3/panorama-stitched.jpg" 2400 pano; f "$b/slideshow.mp4" 1700 slides  # only right

# git repo for the git status column: committed, modified, ignored and new files
g="$m/photo-tools"
mkdir -p "$g/scripts"
cat > "$g/scripts/sort_by_date.sh" <<'EOF'
#!/usr/bin/env bash
# Move camera files into YYYY-MM-DD folders by EXIF date.
set -euo pipefail
src=${1:-.}
for img in "$src"/*.jpg; do
  day=$(exiftool -d '%Y-%m-%d' -DateTimeOriginal -s3 "$img")
  mkdir -p "$src/$day"
  mv -n "$img" "$src/$day/"
done
EOF
cat > "$g/scripts/make_thumbnails.sh" <<'EOF'
#!/usr/bin/env bash
# Write 400px thumbnails next to each album.
set -euo pipefail
for img in "$1"/*.jpg; do
  convert "$img" -resize 400x400 "$1/thumbs/$(basename "$img")"
done
EOF
cat > "$g/scripts/gallery.toml" <<'EOF'
title = "Family trips"
thumbnail_size = 400
sort = "date"
EOF
echo "thumbs-cache/" > "$g/.gitignore"
mkdir -p "$g/thumbs-cache"; truncate -s 300K "$g/thumbs-cache/index.db"
git -C "$g" init -q
git -C "$g" -c user.name=demo -c user.email=demo@example.invalid add .gitignore scripts
git -C "$g" -c user.name=demo -c user.email=demo@example.invalid commit -qm init
sed -i 's/mv -n "\$img"/mv -n -v "$img"/; s/^set -euo pipefail$/set -euo pipefail\nshopt -s nullglob/' "$g/scripts/sort_by_date.sh"
cat > "$g/scripts/drone_rename.sh" <<'EOF'
#!/usr/bin/env bash
# Rename CLIP_*.mp4 files to flight date and location.
set -euo pipefail
EOF

find "$dir" -exec touch -h -d '2026-01-01 12:00' {} +
