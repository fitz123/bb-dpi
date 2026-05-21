#!/bin/bash
# preinstall — runs as root BEFORE the .pkg payload is extracted.
#
# Wipes any pre-existing ui/ dir (e.g., a Yacd-meta cache from a
# previous install, or stale metacubexd assets from an earlier
# bundled-UI release) so it doesn't shadow the bundled metacubexd UI
# from this .pkg's payload. macOS .pkg payload extraction merges into
# existing target directories: files in the payload overwrite same-
# path files on disk, but files that exist on disk and are NOT in the
# new payload survive. Doing this in preinstall (not postinstall)
# ensures we don't wipe the freshly-extracted UI ourselves.

set -euo pipefail

LEGACY_UI="/Library/Application Support/bb-dpi/ui"
if [[ -d "$LEGACY_UI" ]]; then
    rm -rf "$LEGACY_UI"
fi

exit 0
