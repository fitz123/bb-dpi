#!/bin/bash
# bb-vpn-uninstall — full reverse of the .pkg postinstall.
#
# Installed to /Library/Application Support/bb-dpi/bin/bb-vpn-uninstall.
# Run as:
#   sudo "/Library/Application Support/bb-dpi/bin/bb-vpn-uninstall"
#
# Tears down all three LaunchDaemons, removes the plists from
# /Library/LaunchDaemons, deletes the whole /Library/Application Support/bb-dpi
# tree (binaries, configs, bundles, identity, status, control-plane.json,
# logs are under /Library/Logs/bb-dpi and also swept), removes the
# console-user terminal symlink. Brew-installed sing-box/xray binaries
# (if any) live elsewhere and are NOT touched.

set -eu

APP_SUPPORT="/Library/Application Support/bb-dpi"
LAUNCH_DAEMONS="/Library/LaunchDaemons"
LOG_DIR="/Library/Logs/bb-dpi"

log() { echo "[bb-dpi uninstall] $*"; }

if [[ "$(id -u)" -ne 0 ]]; then
    echo "must run as root: sudo \"$APP_SUPPORT/bin/bb-vpn-uninstall\"" >&2
    exit 1
fi

log "booting out daemons"
launchctl bootout system/com.bb-dpi.bb-vpn-sync >/dev/null 2>&1 || true
launchctl bootout system/com.sing-box-vpn       >/dev/null 2>&1 || true
launchctl bootout system/com.xray-xhttp         >/dev/null 2>&1 || true

# Drop the runtime_blackhole disables in case the circuit breaker
# was tripped — otherwise a future re-install would still find them
# stuck in `disabled` state.
launchctl enable system/com.sing-box-vpn >/dev/null 2>&1 || true
launchctl enable system/com.xray-xhttp   >/dev/null 2>&1 || true

log "removing LaunchDaemon plists"
rm -f "$LAUNCH_DAEMONS/com.bb-dpi.bb-vpn-sync.plist"
rm -f "$LAUNCH_DAEMONS/com.sing-box-vpn.plist"
rm -f "$LAUNCH_DAEMONS/com.xray-xhttp.plist"

# Console-user terminal-convenience symlink. The postinstall wrote
# this under the logged-in user's ~/.local/bin; clean it up too. If
# the symlink points elsewhere (e.g., the user moved bb-vpn manually),
# leave it alone.
CONSOLE_USER=$(stat -f %Su /dev/console)
if [[ "$CONSOLE_USER" != "root" && -n "$CONSOLE_USER" ]]; then
    LOCAL_BIN="/Users/$CONSOLE_USER/.local/bin/bb-vpn"
    if [[ -L "$LOCAL_BIN" ]]; then
        target=$(readlink "$LOCAL_BIN")
        if [[ "$target" == "$APP_SUPPORT/bin/bb-vpn" ]]; then
            log "removing $LOCAL_BIN"
            rm -f "$LOCAL_BIN"
        fi
    fi
fi

log "removing $APP_SUPPORT"
rm -rf "$APP_SUPPORT"
log "removing $LOG_DIR"
rm -rf "$LOG_DIR"

log "uninstall complete"
exit 0
