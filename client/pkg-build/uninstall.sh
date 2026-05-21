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
# console-user terminal symlink, and removes /Applications/BBVPN.app.
# Brew-installed sing-box/xray binaries (if any) live elsewhere and
# are NOT touched.

set -eu

APP_SUPPORT="/Library/Application Support/bb-dpi"
LAUNCH_DAEMONS="/Library/LaunchDaemons"
LOG_DIR="/Library/Logs/bb-dpi"
LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Versions/A/Frameworks/LaunchServices.framework/Versions/A/Support/lsregister"

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

# Phase 5 menu-bar app. Detect the console user once — reused for both
# the LaunchAgent bootout (so launchd doesn't immediately respawn BBVPN
# after killall) and the per-user Launch Services unregister below.
UNREG_USER=$(stat -f %Su /dev/console)
UNREG_UID=""
if [[ -n "$UNREG_USER" && "$UNREG_USER" != "root" ]]; then
    UNREG_UID=$(id -u "$UNREG_USER" 2>/dev/null || echo "")
fi

# Boot out the menu-bar LaunchAgent FIRST. Without this, launchd's
# KeepAlive=true on the agent restarts BBVPN immediately after killall,
# leaving a zombie process holding the binary inode under the deleted
# bundle path.
if [[ -n "$UNREG_UID" ]]; then
    log "booting out bb-vpn-menubar LaunchAgent for $UNREG_USER"
    launchctl bootout "gui/$UNREG_UID/com.bb-dpi.bb-vpn-menubar" >/dev/null 2>&1 || true
fi

# Quit any running instance so deleting the bundle doesn't leave a
# zombie process holding the binary inode.
if pgrep -x BBVPN >/dev/null 2>&1; then
    log "quitting running BBVPN.app"
    /usr/bin/killall BBVPN >/dev/null 2>&1 || true
    sleep 1
fi
if [[ -d "/Applications/BBVPN.app" ]]; then
    # Unregister the bundle BEFORE removing it from disk: lsregister -u
    # needs the bundle present so it can read Info.plist's
    # CFBundleIdentifier + CFBundleURLTypes to identify which Launch
    # Services entries to clear. If we rm -rf first, lsregister -u
    # fails with exit 1 ("failed to scan ..., error -10814") and the
    # user's LS DB keeps a stale bb-vpn:// → BBVPN.app mapping until
    # next login or a background re-scan.
    #
    # Mirror postinstall: root's LS database is separate from the
    # console user's. Run the unregister in the console user's GUI
    # session first via `launchctl asuser`, then also in the root
    # session as a fallback for the no-console-user case. Both calls
    # are guarded with `|| true` — best-effort cleanup.
    log "unregistering /Applications/BBVPN.app from Launch Services"
    if [[ -n "$UNREG_USER" && "$UNREG_USER" != "root" && -n "$UNREG_UID" ]]; then
        launchctl asuser "$UNREG_UID" sudo -u "$UNREG_USER" \
            "$LSREGISTER" -u "/Applications/BBVPN.app" >/dev/null 2>&1 || true
    fi
    "$LSREGISTER" -u "/Applications/BBVPN.app" >/dev/null 2>&1 || true
    log "removing /Applications/BBVPN.app"
    rm -rf "/Applications/BBVPN.app"
fi

# Remove the LaunchAgent plist from /Library/LaunchAgents. Bootout
# above already detached it from launchd's running state.
rm -f "/Library/LaunchAgents/com.bb-dpi.bb-vpn-menubar.plist"

log "uninstall complete"
exit 0
