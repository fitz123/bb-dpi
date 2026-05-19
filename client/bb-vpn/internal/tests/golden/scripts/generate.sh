#!/bin/bash
# Generate (or regenerate) the parity-harness golden corpus by running
# the current bash `scripts/render-config` over the full input matrix.
#
# Matrix (36 cells):
#   proto:       all | tcp-vision | xhttp        (3)
#   tailscale:   on | off                        (2)
#   corp-dns:    on | off                        (2)
#   server count: 1 | 2 | 3                      (3)
#
# Each cell produces a sing-box output. For proto in {all, xhttp} we
# also produce an xray output. Cells where the bash script writes
# nothing for xray (proto=tcp-vision) intentionally leave the xray
# golden absent — pkg/render must match that exact "no xray output"
# behavior.
#
# Usage:
#   bash internal/tests/golden/scripts/generate.sh
#   (run from client/bb-vpn or the project root — both work)
#
# Output: internal/tests/golden/expected/<cell>/{sing-box,xray}.json
#
# The goldens encode the bash script's current behavior INCLUDING any
# latent bugs. Divergence from these goldens is a PARITY BREAK, not a
# bug fix. Fix bugs in a follow-up commit that explicitly regenerates
# the goldens.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT_ROOT="$(cd "$HERE/../../../../.." && pwd)"
RENDER_CONFIG="$PROJECT_ROOT/scripts/render-config"
EXPECTED_DIR="$HERE/expected"
INPUTS_DIR="$HERE/inputs"
SKELETONS_DIR="$HERE/skeletons"

if [[ ! -x "$RENDER_CONFIG" ]]; then
    echo "Error: $RENDER_CONFIG not found / not executable" >&2
    exit 1
fi

# Wipe + recreate expected/ so removed cells don't linger as zombies.
rm -rf "$EXPECTED_DIR"
mkdir -p "$EXPECTED_DIR"

# Deterministic env for every cell.
# shellcheck disable=SC1091
source "$INPUTS_DIR/env.sh"

# render-config reads $PROJECT_DIR/servers.json relative to its own
# location AND $PROJECT_DIR/.env. We need to point it at our synthetic
# servers per-cell. Strategy: build a throwaway tree under /tmp that
# contains:
#   scripts/render-config  (copy of real script)
#   config/client/{sing-box-skeleton,xray-xhttp-skeleton}.json (copies)
#   servers.json           (the per-cell synthetic file)
#   .env                   (empty — env vars come from env.sh export)
#
# This keeps the operator's real ~/bb-dpi servers.json untouched.

THROWAWAY="$(mktemp -d /tmp/bb-vpn-goldens.XXXXXX)"
trap 'rm -rf "$THROWAWAY"' EXIT

mkdir -p "$THROWAWAY/scripts" "$THROWAWAY/config/client"
cp "$RENDER_CONFIG" "$THROWAWAY/scripts/render-config"
chmod +x "$THROWAWAY/scripts/render-config"
# Skeletons under internal/tests/golden/skeletons/ are a deliberate
# snapshot of config/client/*.json at the time the goldens were
# generated. PR C's byte-equal test detects any divergence — if those
# tests fail after a config/client/*.json edit, re-copy the skeletons
# and regenerate the corpus in the same commit.
cp "$SKELETONS_DIR/sing-box-skeleton.json"  "$THROWAWAY/config/client/"
cp "$SKELETONS_DIR/xray-xhttp-skeleton.json" "$THROWAWAY/config/client/"
: > "$THROWAWAY/.env" # empty; env vars come from sourcing env.sh above

cell_count=0
for proto in all tcp-vision xhttp; do
    for ts in off on; do
        for corp in off on; do
            for n in 1 2 3; do
                cell="proto-${proto}_ts-${ts}_corp-${corp}_n${n}"
                outdir="$EXPECTED_DIR/$cell"
                mkdir -p "$outdir"

                cp "$INPUTS_DIR/servers-${n}.json" "$THROWAWAY/servers.json"

                flags=( --proto "$proto" )
                [[ "$ts"   == on ]] && flags+=( --with-tailscale )
                [[ "$corp" == on ]] && flags+=( --with-corp-dns )

                # SINGBOX_OUTPUT / XRAY_OUTPUT override paths.
                # Stderr streams through so failures are diagnosable on
                # the first run. tcp-vision cells naturally leave xray
                # absent — bash render-config doesn't write it, and
                # outdir was freshly created above. That absence IS the
                # contract.
                if ! SINGBOX_OUTPUT="$outdir/sing-box.json" \
                     XRAY_OUTPUT="$outdir/xray.json" \
                     "$THROWAWAY/scripts/render-config" "${flags[@]}" >/dev/null; then
                    echo "FAIL: $cell" >&2
                    exit 1
                fi

                # Canonicalize each rendered file via `jq -S --indent 2`
                # so the parity test in pkg/render can byte-compare against
                # Go's stdlib `encoding/json` MarshalIndent output (which
                # also sorts keys alphabetically). Without canonicalization
                # the goldens preserve render-config's jq-insertion order,
                # which Go's stdlib can't reproduce without rolling a
                # full ordered-JSON marshaller.
                for f in "$outdir/sing-box.json" "$outdir/xray.json"; do
                    [[ -f "$f" ]] || continue
                    jq -S --indent 2 . "$f" > "$f.canonical"
                    mv "$f.canonical" "$f"
                done

                cell_count=$((cell_count + 1))
            done
        done
    done
done

echo "Generated $cell_count cells under $EXPECTED_DIR"
