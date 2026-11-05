#!/usr/bin/env bash
# Builds a tree and an isolated fileindex database for manually checking the directory state
# icon, fileindex.report-differences, and operations that keep the disk and the index in step.
#
# Usage: test-cases/gen-fileindex-drift.sh [dir]     (default: fi-testdata/ in the worktree root)
#
# The tree is scanned with the fileindex version pc pins in go.mod, then mutated. Nothing touches
# your real index: the database and fileindex's config.ini live in the output directory, and the
# launch command printed at the end selects them. Keep the output untracked but not gitignored
# (pc's gitignore filter would hide it) and do not commit it.
#
# Expected state of each directory under tree/, browsing with fileindex:
#
#   clean                    indexed
#   new-file                 differs, cause visible below it
#   changed-size             differs, cause visible below it
#   changed-mtime            differs, cause visible below it
#   missing-file             differs, cause visible below it (ghost row)
#   missing-subdir           differs, cause visible below it (ghost row)
#   hidden-new               differs, cause shown only by the report or with hidden files on
#   symlink-new              differs, cause visible on the symlink row
#   symlink-dangling         differs, cause visible on the symlink row
#   symlink-target-changed   differs, cause visible on the symlink row
#   excluded-recorded        indexed (records under node_modules, which the walk skips)
#   unreadable               indexed (records under a directory the walk cannot read)
#   deep                     unchecked; the report still finds the difference
#   ops-delete, ops-rename, ops-move, ops-flatten
#                            indexed; each holds non-indexed files (excluded names) to try
#                            operations from a Browse-with and a fileindex:// panel
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIR="${1:-${REPO}/fi-testdata}"

if [[ -e "${DIR}" ]]; then
	echo "error: ${DIR} already exists; remove it first:" >&2
	echo "  chmod -R u+rwx '${DIR}' && rm -rf '${DIR}'" >&2
	exit 1
fi
mkdir -p "${DIR}"
DIR="$(cd "${DIR}" && pwd -P)"
TREE="${DIR}/tree"
OUTSIDE="${DIR}/outside" # symlink targets, not scanned
mkdir -p "${TREE}" "${OUTSIDE}"
# Go skips a directory holding a go.mod in ./...: the unreadable fixture would fail go test and lint.
printf 'module fitestdata\n' >"${DIR}/go.mod"

# The same settings reach the scans below and pc: a config.ini of yours cannot change the rules.
EXCLUDE_DIRS='.git,.cache,node_modules,__pycache__'
EXCLUDE_FILES='.DS_Store,Thumbs.db'
export FILEINDEX_DATABASE_PATH="${DIR}/index.sqlite"
export FILEINDEX_SCANNING_FILES='*'
export FILEINDEX_SCANNING_EXCLUDE_DIRS="${EXCLUDE_DIRS}"
export FILEINDEX_SCANNING_EXCLUDE_FILES="${EXCLUDE_FILES}"

# go.mod pins the fileindex library; its CLI's own dependencies are not in pc's go.sum, so the
# command runs as package@version.
VERSION="$(cd "${REPO}" && go list -m -f '{{.Version}}' github.com/paranoidi/fileindex)"
fileindex() {
	# --config: without it the CLI writes ~/.config/fileindex/config.ini.
	(cd "${DIR}" && go run "github.com/paranoidi/fileindex/cmd/fileindex@${VERSION}" --config "${DIR}/config.ini" "$@")
}

# write PATH TEXT creates a file, and its directories, under tree/.
write() {
	mkdir -p "$(dirname "${TREE}/$1")"
	printf '%s\n' "$2" >"${TREE}/$1"
}

# --- state before the scan ---------------------------------------------------------------------
write clean/lantern.md 'amber lantern'
write clean/orchard/meadow.md 'quiet meadow'

write new-file/harbor.md 'grey harbor'

write changed-size/thimble.md 'small thimble'
write changed-mtime/compass.md 'brass compass'

write missing-file/pebble.md 'round pebble'
write missing-file/walnut.md 'hollow walnut'
write missing-subdir/willow.md 'bent willow'
write missing-subdir/canyon/ember.md 'last ember'

write hidden-new/sparrow.md 'brown sparrow'

write symlink-new/kettle.md 'copper kettle'

write symlink-dangling/ribbon.md 'silk ribbon'
printf 'far anchor\n' >"${OUTSIDE}/anchor.md"
ln -s "${OUTSIDE}/anchor.md" "${TREE}/symlink-dangling/anchor-link.md"

write symlink-target-changed/saddle.md 'worn saddle'
printf 'old beacon\n' >"${OUTSIDE}/beacon.md"
ln -s "${OUTSIDE}/beacon.md" "${TREE}/symlink-target-changed/beacon-link.md"

write excluded-recorded/marble.md 'white marble'
write excluded-recorded/node_modules/velvet/index.js 'module.exports = 1'

write unreadable/feather.md 'blue feather'
write unreadable/vault/tunnel.md 'dark tunnel'

write deep/gravel.md 'loose gravel'
write deep/one/two/three/four/five/cobalt.md 'cobalt shard'

# ops-*: an indexed file or two, plus files the index never lists (excluded names), which must
# travel with their directory.
write ops-delete/victim/candle.md 'tall candle'
write ops-delete/victim/node_modules/pepper/index.js 'module.exports = 2'
write ops-delete/victim/.cache/mirror.bin 'mirror'
write ops-delete/single.md 'lone file'

write ops-rename/folder/basket.md 'woven basket'
write ops-rename/folder/node_modules/thistle/index.js 'module.exports = 3'
write ops-rename/whistle.md 'tin whistle'
write ops-rename/Thumbs.db 'not indexed: renaming onto it must be refused from fileindex://'

write ops-move/source/bucket.md 'iron bucket'
write ops-move/source/node_modules/clover/index.js 'module.exports = 4'
write ops-move/single.md 'lone file'
write ops-move/target/keep.md 'stays here'

write ops-flatten/nest/alpha/acorn.md 'green acorn'
write ops-flatten/nest/alpha/beta/hazel.md 'ripe hazel'
write ops-flatten/nest/gamma/juniper.md 'dry juniper'
write ops-flatten/nest/gamma/node_modules/fern/index.js 'module.exports = 5'
write ops-flatten/nest/gamma/Thumbs.db 'not indexed'

# --- scan --------------------------------------------------------------------------------------
fileindex init --non-interactive >/dev/null
fileindex scan -q "${TREE}"
# Records the walk never reaches: as if scanned before node_modules became excluded.
fileindex scan -q "${TREE}/excluded-recorded/node_modules"

# --- mutate ------------------------------------------------------------------------------------
write new-file/lagoon.md 'still lagoon'

printf 'and a bit more\n' >>"${TREE}/changed-size/thimble.md"
touch -d '2001-02-03 04:05:06' "${TREE}/changed-mtime/compass.md"

rm "${TREE}/missing-file/walnut.md"
rm -r "${TREE}/missing-subdir/canyon"

write hidden-new/.starling 'hidden bird'

printf 'link target\n' >"${OUTSIDE}/glacier.md"
ln -s "${OUTSIDE}/glacier.md" "${TREE}/symlink-new/glacier-link.md"

rm "${OUTSIDE}/anchor.md"
printf 'new, longer beacon\n' >"${OUTSIDE}/beacon.md"

chmod 000 "${TREE}/unreadable/vault"

write deep/one/two/three/four/five/quartz.md 'clear quartz, below the checked depth'

# --- done --------------------------------------------------------------------------------------
cat <<DONE
Test data in ${DIR}

Launch (add ./pc or ~/go/bin/pc as you like):

  env FILEINDEX_DATABASE_PATH='${FILEINDEX_DATABASE_PATH}' \\
      FILEINDEX_SCANNING_FILES='*' \\
      FILEINDEX_SCANNING_EXCLUDE_DIRS='${EXCLUDE_DIRS}' \\
      FILEINDEX_SCANNING_EXCLUDE_FILES='${EXCLUDE_FILES}' \\
      ${REPO}/pc '${TREE}'

Then run fileindex.browse-with (or fileindex.browse) in that panel. A [fileindex].database in
pc's config.toml overrides FILEINDEX_DATABASE_PATH: remove it for this run.

Remove the data with:

  chmod -R u+rwx '${DIR}' && rm -rf '${DIR}'
DONE
