#!/bin/bash
# build.sh — compile BBVPN.app from Swift sources.
#
# Output: <project>/build/menubar/BBVPN.app
#
# Builds with `xcrun -sdk macosx swiftc` so we don't need a .xcodeproj.
# The resulting .app is unsigned (ad-hoc codesign happens in Phase 6
# alongside the .pkg signing pass).
#
# Universal binary: arm64 + x86_64 via `-target` flags + lipo, so the
# same .app runs on both Apple Silicon and Intel Macs.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && cd .. && pwd)"

SRC_DIR="$SCRIPT_DIR/BBVPN"
INFO_PLIST="$SCRIPT_DIR/Info.plist"
OUT_ROOT="$PROJECT_DIR/build/menubar"
APP_DIR="$OUT_ROOT/BBVPN.app"
CONTENTS="$APP_DIR/Contents"
MACOS_DIR="$CONTENTS/MacOS"

die() { echo "Error: $*" >&2; exit 1; }

command -v xcrun >/dev/null 2>&1 || die "xcrun is required (Xcode CLT)"
[[ -d "$SRC_DIR"     ]] || die "missing $SRC_DIR"
[[ -f "$INFO_PLIST"  ]] || die "missing $INFO_PLIST"

SWIFT_SOURCES=("$SRC_DIR"/*.swift)
[[ "${#SWIFT_SOURCES[@]}" -ge 1 ]] || die "no .swift files in $SRC_DIR"

rm -rf "$APP_DIR"
mkdir -p "$MACOS_DIR"

echo "Compiling Swift sources for arm64..."
xcrun -sdk macosx swiftc \
    -target arm64-apple-macos13.0 \
    -O \
    -framework AppKit \
    -framework SwiftUI \
    -o "$MACOS_DIR/BBVPN.arm64" \
    "${SWIFT_SOURCES[@]}"

echo "Compiling Swift sources for x86_64..."
xcrun -sdk macosx swiftc \
    -target x86_64-apple-macos13.0 \
    -O \
    -framework AppKit \
    -framework SwiftUI \
    -o "$MACOS_DIR/BBVPN.x86_64" \
    "${SWIFT_SOURCES[@]}"

echo "Linking universal binary..."
lipo -create -output "$MACOS_DIR/BBVPN" \
    "$MACOS_DIR/BBVPN.arm64" \
    "$MACOS_DIR/BBVPN.x86_64"
rm "$MACOS_DIR/BBVPN.arm64" "$MACOS_DIR/BBVPN.x86_64"
chmod 0755 "$MACOS_DIR/BBVPN"

# Copy Info.plist (validate first — Xcode silently accepts malformed
# XML but Launch Services later refuses to register the URL scheme).
plutil -lint "$INFO_PLIST" >/dev/null
cp "$INFO_PLIST" "$CONTENTS/Info.plist"

# PkgInfo helps older Launch Services + makes the .app introspectable
# via `mdls -name kMDItemKind`.
printf 'APPL????' > "$CONTENTS/PkgInfo"

# Strip xattrs (com.apple.provenance on copied files) so the resulting
# .app doesn't carry AppleDouble sidecars when bundled into a .pkg.
xattr -cr "$APP_DIR"

echo "Built $APP_DIR"
file "$MACOS_DIR/BBVPN"
