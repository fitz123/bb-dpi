#!/bin/bash
# postinstall — runs as root immediately after the .pkg payload lands.
#
# Idempotent: tolerates reinstall-over-existing. Order matters; see
# Phase 4 §postinstall.sh of the plan for why sing-box/xray must be
# bootouted BEFORE bb-vpn-sync is bootstrapped.

set -euo pipefail

APP_SUPPORT="/Library/Application Support/bb-dpi"
LAUNCH_DAEMONS="/Library/LaunchDaemons"
LOG_DIR="/Library/Logs/bb-dpi"

log() { echo "[bb-dpi postinstall] $*"; }

log "applying ownership + perms on payload"
chown -R root:wheel "$APP_SUPPORT"
chmod 0755 "$APP_SUPPORT"
chmod 0755 "$APP_SUPPORT/bin"
chmod 0755 "$APP_SUPPORT/bin/bb-vpn"
chmod 0755 "$APP_SUPPORT/bin/sing-box"
chmod 0755 "$APP_SUPPORT/bin/xray"
chmod 0755 "$APP_SUPPORT/bin/bb-vpn-uninstall"
# control-plane.json holds the bearer token. Root-only readable —
# the only consumer is the bb-vpn root LaunchDaemon. 0644 would
# leak the bearer to every non-admin user on a multi-user Mac.
chmod 0600 "$APP_SUPPORT/control-plane.json"

chown root:wheel "$LAUNCH_DAEMONS/com.bb-dpi.bb-vpn-sync.plist"
chown root:wheel "$LAUNCH_DAEMONS/com.sing-box-vpn.plist"
chown root:wheel "$LAUNCH_DAEMONS/com.xray-xhttp.plist"
chmod 0644 "$LAUNCH_DAEMONS/com.bb-dpi.bb-vpn-sync.plist"
chmod 0644 "$LAUNCH_DAEMONS/com.sing-box-vpn.plist"
chmod 0644 "$LAUNCH_DAEMONS/com.xray-xhttp.plist"

log "ensuring runtime directories"
mkdir -p "$APP_SUPPORT/bundles" "$APP_SUPPORT/configs" "$APP_SUPPORT/staging" "$APP_SUPPORT/inbox"
chown root:wheel "$APP_SUPPORT/bundles" "$APP_SUPPORT/configs" "$APP_SUPPORT/staging" "$APP_SUPPORT/inbox"
chmod 0755 "$APP_SUPPORT/bundles" "$APP_SUPPORT/configs" "$APP_SUPPORT/staging"
# inbox is the only drop-box a non-root user can write to (enrollment
# requests, sync-now markers). Sticky + world-writable: anyone can
# create a file, only root can delete arbitrary entries.
chmod 1733 "$APP_SUPPORT/inbox"

mkdir -p "$LOG_DIR"
chown root:wheel "$LOG_DIR"
chmod 0755 "$LOG_DIR"

# Substitute BB_VPN_HOME in the sync plist with the console user's
# real home. Required: bb-vpn sync's buildSyncEnv hard-rejects
# HOME=/var/root (the launchd-sanitized default).
CONSOLE_USER=$(stat -f %Su /dev/console)
if [[ "$CONSOLE_USER" == "root" || -z "$CONSOLE_USER" ]]; then
    log "WARN: could not detect a logged-in console user (got '$CONSOLE_USER'); leaving plist BB_VPN_HOME placeholder in place. bb-vpn sync will fail until you edit $LAUNCH_DAEMONS/com.bb-dpi.bb-vpn-sync.plist manually."
else
    HOME_PATH="/Users/$CONSOLE_USER"
    if [[ ! -d "$HOME_PATH" ]]; then
        log "WARN: $HOME_PATH does not exist; leaving placeholder. Edit the plist manually."
    else
        log "substituting BB_VPN_HOME=$HOME_PATH in bb-vpn-sync plist"
        # In-place edit using a literal string (no regex metas), via a
        # safe round-trip through plutil → sed → plutil.
        /usr/bin/sed -i '' \
            "s|/Users/REPLACE_BEFORE_DEPLOY|$HOME_PATH|" \
            "$LAUNCH_DAEMONS/com.bb-dpi.bb-vpn-sync.plist"
        if ! plutil -lint "$LAUNCH_DAEMONS/com.bb-dpi.bb-vpn-sync.plist" >/dev/null; then
            log "ERROR: sync plist failed plutil lint after BB_VPN_HOME substitution; aborting"
            exit 1
        fi
    fi
fi

# Console-user terminal-convenience symlink. Best-effort: missing
# ~/.local/bin gets created with that user's ownership; failures
# (sandboxed home, FileVault edge case) are logged but non-fatal.
if [[ "$CONSOLE_USER" != "root" && -n "$CONSOLE_USER" ]]; then
    HOME_PATH="/Users/$CONSOLE_USER"
    LOCAL_BIN="$HOME_PATH/.local/bin"
    if [[ -d "$HOME_PATH" ]]; then
        if [[ ! -d "$LOCAL_BIN" ]]; then
            install -d -m 0755 -o "$CONSOLE_USER" -g staff "$LOCAL_BIN" \
                || log "WARN: could not create $LOCAL_BIN"
        fi
        ln -sfn "$APP_SUPPORT/bin/bb-vpn" "$LOCAL_BIN/bb-vpn" \
            && chown -h "$CONSOLE_USER:staff" "$LOCAL_BIN/bb-vpn" \
            || log "WARN: could not symlink $LOCAL_BIN/bb-vpn"
    fi
fi

# Idempotent bootout of sing-box and xray FIRST. The plists on disk
# just got replaced, but launchd still holds the OLD semantics — boot
# them out so bb-vpn's first sync can re-bootstrap from the fresh
# plists. Order matters: tearing these down must happen BEFORE
# bb-vpn-sync is bootstrapped, otherwise the new sync (RunAtLoad=true)
# fires immediately, re-bootstraps these, and step 4 then tears them
# back down again.
log "booting out sing-box and xray (idempotent)"
launchctl bootout system/com.sing-box-vpn >/dev/null 2>&1 || true
launchctl bootout system/com.xray-xhttp   >/dev/null 2>&1 || true

# Idempotent bootout-then-bootstrap of bb-vpn-sync LAST. The bootout
# is required so a plain bootstrap doesn't error on an already-loaded
# service; without it, the OLD bb-vpn binary keeps running with the
# OLD sync logic (launchd holds the prior inode open until reboot),
# defeating the binary-upgrade path.
log "(re)bootstrapping bb-vpn-sync"
launchctl bootout system/com.bb-dpi.bb-vpn-sync >/dev/null 2>&1 || true
launchctl bootstrap system "$LAUNCH_DAEMONS/com.bb-dpi.bb-vpn-sync.plist"

log "postinstall complete"
exit 0
