#!/bin/bash
# build.sh — assemble the BB-VPN macOS installer .pkg.
#
# Coupling contract (Phase 4 of pkg-and-pull-control-plane plan):
#   sing-box, xray, and bb-vpn versions baked into the .pkg payload MUST
#   match config/control-plane/package-manifest.json bb_vpn/sing_box/xray
#   keys. Mismatch aborts the build before pkgbuild runs.
#
# Inputs expected by this script:
#   client/pkg-build/payload-binaries/sing-box   (executable, darwin universal)
#   client/pkg-build/payload-binaries/xray       (executable, darwin universal)
#   build/pkg/bb-vpn                              (built by `make build-bb-vpn-pkg`)
#
# Output: client/pkg-build/dist/BB-VPN-<bb_vpn>.pkg

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && cd .. && pwd)"

MANIFEST="$PROJECT_DIR/config/control-plane/package-manifest.json"
ENDPOINTS_FILE="$PROJECT_DIR/config/control-plane/endpoints.json"
TOKEN_FILE="$PROJECT_DIR/config/control-plane/token"
PAYLOAD_BINS="$SCRIPT_DIR/payload-binaries"
BB_VPN_BIN="$PROJECT_DIR/build/pkg/bb-vpn"
PLISTS_DIR="$PROJECT_DIR/client/plists"
DIST_DIR="$SCRIPT_DIR/dist"
STAGING_DIR="$PROJECT_DIR/build/pkg-staging"
SCRIPTS_DIR="$PROJECT_DIR/build/pkg-scripts"

red()   { printf "\033[0;31m%s\033[0m\n" "$*" >&2; }
green() { printf "\033[0;32m%s\033[0m\n" "$*"; }
blue()  { printf "\033[0;34m%s\033[0m\n" "$*"; }

die() { red "Error: $*"; exit 1; }

# Strictly extract the first `\d+\.\d+\.\d+` triple from a version
# string. Matches both sing-box ("sing-box version 1.13.11") and xray
# ("Xray 26.3.27 ...") layouts.
extract_version() {
    local s="$1"
    [[ "$s" =~ ([0-9]+\.[0-9]+\.[0-9]+) ]] || return 1
    printf '%s' "${BASH_REMATCH[1]}"
}

[[ -f "$MANIFEST"      ]] || die "missing $MANIFEST"
[[ -f "$ENDPOINTS_FILE" ]] || die "missing $ENDPOINTS_FILE — see docs/control-plane-bootstrap.md"
[[ -f "$TOKEN_FILE"     ]] || die "missing $TOKEN_FILE — see docs/control-plane-bootstrap.md"
command -v jq        >/dev/null 2>&1 || die "jq is required"
command -v pkgbuild  >/dev/null 2>&1 || die "pkgbuild is required (Xcode CLT)"
command -v productbuild >/dev/null 2>&1 || die "productbuild is required (Xcode CLT)"

EXPECT_BB=$(jq -r '.bb_vpn'   "$MANIFEST")
EXPECT_SB=$(jq -r '.sing_box' "$MANIFEST")
EXPECT_XR=$(jq -r '.xray'     "$MANIFEST")
[[ -n "$EXPECT_BB" && "$EXPECT_BB" != "null" ]] || die "manifest.bb_vpn is empty"
[[ -n "$EXPECT_SB" && "$EXPECT_SB" != "null" ]] || die "manifest.sing_box is empty"
[[ -n "$EXPECT_XR" && "$EXPECT_XR" != "null" ]] || die "manifest.xray is empty"

blue "manifest pins: bb-vpn=$EXPECT_BB  sing-box=$EXPECT_SB  xray=$EXPECT_XR"

[[ -x "$BB_VPN_BIN" ]] || die "missing $BB_VPN_BIN — run 'make build-bb-vpn-pkg BB_VPN_VERSION=$EXPECT_BB' first"
[[ -x "$PAYLOAD_BINS/sing-box" ]] || die "missing $PAYLOAD_BINS/sing-box (drop the bundled binary in place; see client/pkg-build/README.md)"
[[ -x "$PAYLOAD_BINS/xray"     ]] || die "missing $PAYLOAD_BINS/xray (drop the bundled binary in place; see client/pkg-build/README.md)"

# Version-coupling check: invoke each binary, parse, compare.
BB_OUT=$("$BB_VPN_BIN" --version)
SB_OUT=$("$PAYLOAD_BINS/sing-box" version 2>&1 | head -1)
# `xray version` writes a multi-line banner to stdout; take the line
# containing "Xray".
XR_OUT=$("$PAYLOAD_BINS/xray" version 2>&1 | grep -m1 '^Xray ' || true)
[[ -n "$XR_OUT" ]] || die "xray version did not produce a recognizable banner"

ACTUAL_BB=$(extract_version "$BB_OUT") || die "could not parse bb-vpn version from: $BB_OUT"
ACTUAL_SB=$(extract_version "$SB_OUT") || die "could not parse sing-box version from: $SB_OUT"
ACTUAL_XR=$(extract_version "$XR_OUT") || die "could not parse xray version from: $XR_OUT"

mismatch=0
[[ "$ACTUAL_BB" == "$EXPECT_BB" ]] || { red "bb-vpn:   manifest=$EXPECT_BB actual=$ACTUAL_BB"; mismatch=1; }
[[ "$ACTUAL_SB" == "$EXPECT_SB" ]] || { red "sing-box: manifest=$EXPECT_SB actual=$ACTUAL_SB"; mismatch=1; }
[[ "$ACTUAL_XR" == "$EXPECT_XR" ]] || { red "xray:     manifest=$EXPECT_XR actual=$ACTUAL_XR"; mismatch=1; }
if [[ "$mismatch" -ne 0 ]]; then
    red ""
    red "Version mismatch between manifest and shipped binaries."
    red "Fix one of:"
    red "  (a) update config/control-plane/package-manifest.json to match the binaries you intend to ship"
    red "  (b) replace the binaries under $PAYLOAD_BINS/ (and rebuild bb-vpn) to match the manifest"
    red "Manifest + .pkg + future bundle.min_versions must be edited together."
    exit 1
fi

green "version-coupling check passed."

# Compose staging tree mirroring the on-disk layout the postinstall
# script promotes from /tmp into /Library and /usr/local.
rm -rf "$STAGING_DIR" "$SCRIPTS_DIR"
mkdir -p "$STAGING_DIR/Library/Application Support/bb-dpi/bin"
mkdir -p "$STAGING_DIR/Library/LaunchDaemons"
mkdir -p "$STAGING_DIR/Library/Logs/bb-dpi"
mkdir -p "$SCRIPTS_DIR"

install -m 0755 "$BB_VPN_BIN"            "$STAGING_DIR/Library/Application Support/bb-dpi/bin/bb-vpn"
install -m 0755 "$PAYLOAD_BINS/sing-box" "$STAGING_DIR/Library/Application Support/bb-dpi/bin/sing-box"
install -m 0755 "$PAYLOAD_BINS/xray"     "$STAGING_DIR/Library/Application Support/bb-dpi/bin/xray"
install -m 0755 "$SCRIPT_DIR/uninstall.sh" "$STAGING_DIR/Library/Application Support/bb-dpi/bin/bb-vpn-uninstall"

install -m 0644 "$PLISTS_DIR/com.bb-dpi.bb-vpn-sync.plist" "$STAGING_DIR/Library/LaunchDaemons/"
install -m 0644 "$PLISTS_DIR/com.sing-box-vpn.plist"        "$STAGING_DIR/Library/LaunchDaemons/"
install -m 0644 "$PLISTS_DIR/com.xray-xhttp.plist"          "$STAGING_DIR/Library/LaunchDaemons/"

# Assemble control-plane.json from the operator-secret
# endpoints.json + token files. cphttp.LoadConfig expects the
# {endpoints:[...], token:"..."} shape; ssh/remote_bundle_path are
# publish-bundle-only fields and are stripped here. postinstall
# chmods this to 0600 root:wheel.
TOKEN_TRIMMED=$(tr -d '\r\n' < "$TOKEN_FILE")
jq --arg tok "$TOKEN_TRIMMED" \
    '{endpoints: [.[] | {label, url, host_ip, sni, placeholder}], token: $tok}' \
    "$ENDPOINTS_FILE" \
    > "$STAGING_DIR/Library/Application Support/bb-dpi/control-plane.json"
chmod 0600 "$STAGING_DIR/Library/Application Support/bb-dpi/control-plane.json"

# Strip xattrs so pkgbuild doesn't emit AppleDouble ._* sidecars for
# every payload file (com.apple.provenance and friends on brew-installed
# binaries). Keeps the .pkg smaller and the payload listing clean.
xattr -cr "$STAGING_DIR"

# postinstall lives in pkgbuild's --scripts dir, NOT the payload tree.
install -m 0755 "$SCRIPT_DIR/postinstall.sh" "$SCRIPTS_DIR/postinstall"

mkdir -p "$DIST_DIR"
# productbuild looks up the component by the unversioned filename
# referenced in distribution.xml — match that exactly here, then rm
# the intermediate after the flat distribution-style .pkg is sealed.
COMPONENT_PKG="$DIST_DIR/BB-VPN-component.pkg"
FINAL_PKG="$DIST_DIR/BB-VPN-$EXPECT_BB.pkg"

blue "running pkgbuild..."
pkgbuild \
    --root "$STAGING_DIR" \
    --identifier "com.bb-dpi.bb-vpn" \
    --version "$EXPECT_BB" \
    --scripts "$SCRIPTS_DIR" \
    --install-location "/" \
    "$COMPONENT_PKG"

blue "running productbuild..."
productbuild \
    --distribution "$SCRIPT_DIR/distribution.xml" \
    --package-path "$DIST_DIR" \
    "$FINAL_PKG"

rm -f "$COMPONENT_PKG"

green "built $FINAL_PKG"
