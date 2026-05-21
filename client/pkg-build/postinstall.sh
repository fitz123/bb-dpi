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
LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Versions/A/Frameworks/LaunchServices.framework/Versions/A/Support/lsregister"

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

# Phase 5 menu-bar LaunchAgent. Same ownership/perms as the daemons —
# system-domain agents live in /Library/LaunchAgents and must be
# root:wheel 0644 so launchd accepts them.
chown root:wheel "/Library/LaunchAgents/com.bb-dpi.bb-vpn-menubar.plist"
chmod 0644       "/Library/LaunchAgents/com.bb-dpi.bb-vpn-menubar.plist"

# Prepare the log directory BEFORE bootstrapping bb-vpn-menubar below —
# the agent runs in the console user's GUI domain and its plist points
# stdout/stderr at /Library/Logs/bb-dpi/bb-vpn-menubar.log. If we leave
# the dir at the prior 0755 root:wheel default until the runtime-dirs
# block at the bottom of the script, launchd's very first start of the
# agent races on a non-user-writable log path and may silently fail
# (the bootstrap `|| true` would hide it). Group `admin` covers the
# console user; 0775 lets both root (sync daemon) and user (menu-bar
# agent) open files here.
mkdir -p "$LOG_DIR"
chown root:admin "$LOG_DIR"
chmod 0775 "$LOG_DIR"

# Phase 5 menu-bar app. Apps in /Applications conventionally are
# root:admin (system-installed) so any admin user can update/replace
# them; the executable inside Contents/MacOS/ stays executable.
if [[ -d "/Applications/BBVPN.app" ]]; then
    log "applying perms on /Applications/BBVPN.app"
    chown -R root:admin "/Applications/BBVPN.app"
    # Register the bundle with Launch Services so bb-vpn:// clicks route
    # to it on first click, not after the user double-launches the .app.
    # Root's lsregister database is separate from the console user's; if
    # we only register as root, the user's LS DB doesn't learn about the
    # URL scheme until next login or a background re-scan. Register in
    # the console user's GUI session via `launchctl asuser` so bb-vpn://
    # routes on first click. Keep the root-side call as a fallback for
    # the (rare) no-console-user case.
    REG_USER=$(stat -f %Su /dev/console)
    REG_UID=""
    if [[ -n "$REG_USER" && "$REG_USER" != "root" ]]; then
        REG_UID=$(id -u "$REG_USER" 2>/dev/null || echo "")
        if [[ -n "$REG_UID" ]]; then
            launchctl asuser "$REG_UID" sudo -u "$REG_USER" \
                "$LSREGISTER" -f "/Applications/BBVPN.app" >/dev/null 2>&1 || true
        fi
    fi
    "$LSREGISTER" -f "/Applications/BBVPN.app" >/dev/null 2>&1 || true

    # Bootstrap the menu-bar LaunchAgent into the console user's GUI
    # domain so BBVPN.app starts immediately (post-install) and at every
    # subsequent login. Without this the .pkg lays down the agent plist
    # but launchd never picks it up until the user manually logs out/in.
    # bootout-then-bootstrap is idempotent — handles reinstall-over-existing.
    if [[ -n "$REG_USER" && "$REG_USER" != "root" && -n "$REG_UID" ]]; then
        log "bootstrapping bb-vpn-menubar LaunchAgent for $REG_USER"
        launchctl bootout "gui/$REG_UID/com.bb-dpi.bb-vpn-menubar" >/dev/null 2>&1 || true
        launchctl bootstrap "gui/$REG_UID" \
            "/Library/LaunchAgents/com.bb-dpi.bb-vpn-menubar.plist" >/dev/null 2>&1 || true
    fi
fi

log "ensuring runtime directories"
mkdir -p "$APP_SUPPORT/bundles" "$APP_SUPPORT/configs" "$APP_SUPPORT/staging" "$APP_SUPPORT/inbox"
chown root:wheel "$APP_SUPPORT/bundles" "$APP_SUPPORT/configs" "$APP_SUPPORT/staging" "$APP_SUPPORT/inbox"
chmod 0755 "$APP_SUPPORT/bundles" "$APP_SUPPORT/configs" "$APP_SUPPORT/staging"
# inbox is the only drop-box a non-root user can write to (enrollment
# requests, sync-now markers). Sticky + world-writable: anyone can
# create a file, only root can delete arbitrary entries.
chmod 1733 "$APP_SUPPORT/inbox"

# Log dir was already prepared above (before the LaunchAgent bootstrap)
# so the agent's first start doesn't race a non-user-writable path.

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

# Idempotent bootout-then-bootstrap of bb-vpn-sync. The bootout is
# required so a plain bootstrap doesn't error on an already-loaded
# service; without it, the OLD bb-vpn binary keeps running with the
# OLD sync logic (launchd holds the prior inode open until reboot),
# defeating the binary-upgrade path.
log "(re)bootstrapping bb-vpn-sync"
launchctl bootout system/com.bb-dpi.bb-vpn-sync >/dev/null 2>&1 || true
launchctl bootstrap system "$LAUNCH_DAEMONS/com.bb-dpi.bb-vpn-sync.plist"

# Kickstart sing-box + xray. bb-vpn-sync's RunAtLoad tick can't do
# this on its own: the bundle hasn't changed across install, so
# sync.Tick short-circuits at step 5 (configs match) without
# kickstarting. Result: a reinstall where the postinstall just
# bootouted both daemons leaves them down. Calling `bb-vpn start`
# directly from postinstall fixes that.
#
# Two guards before invoking it:
#
#  1. manually_stopped.flag — the operator previously ran
#     `sudo bb-vpn stop` (captive portal, debugging, …) and expects
#     the daemons to stay down across reboots. `bb-vpn start` would
#     unconditionally clear that flag, so reinstalling the .pkg
#     would silently re-enable the VPN without operator consent.
#     Skip the start; the flag persists across reinstall (only the
#     bb-vpn-uninstall script wipes $APP_SUPPORT entirely).
#
#  2. identity.json absent — fresh install before any `bb-vpn enroll`.
#     There are no rendered configs in configs/ yet, so sing-box's
#     KeepAlive=true plist would crash-loop on a missing config file.
#     Let the first enroll flow trigger sync.Tick instead.
if [[ -f "$APP_SUPPORT/manually_stopped.flag" ]]; then
    log "manually_stopped flag present — skipping bb-vpn start (operator ran 'sudo bb-vpn stop'; reinstall preserves that)"
elif [[ ! -f "$APP_SUPPORT/identity.json" ]]; then
    log "no identity.json (fresh install, not yet enrolled) — skipping bb-vpn start; first 'bb-vpn enroll' will sync"
else
    log "starting sing-box + xray"
    "$APP_SUPPORT/bin/bb-vpn" start || log "WARN: bb-vpn start exited non-zero (non-fatal — next sync will retry)"
fi

log "postinstall complete"
exit 0
