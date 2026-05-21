# client/menubar — BBVPN.app

Tiny SwiftUI menu-bar app. Two responsibilities:

1. **Show status** — polls `/Library/Application Support/bb-dpi/status.json`
   every 5s. Menulet icon turns green/yellow/grey to reflect connected /
   degraded / not-enrolled. The 5s cadence is so the icon reflects
   `sudo bb-vpn start/stop/sync` (terminal-issued) within seconds.
2. **Handle `bb-vpn://enroll?uuid=…`** — Launch Services routes the URI
   to the app; the app shells out to `bb-vpn enroll <uri>` which
   validates and drops an enrollment file into
   `/Library/Application Support/bb-dpi/inbox/` for the root daemon
   to ingest.

Menu items: just **Show log…** + **Quit**. Daemon lifecycle (`start`,
`stop`, `sync`) lives in the `bb-vpn` CLI and requires `sudo`. This
avoids the macOS privilege-escalation gymnastics that an in-menubar
Start/Stop would need (osascript admin prompts on every click, or an
SMJobBless helper). Operators run lifecycle commands from a terminal.

The app **never invokes daemons or launchctl directly** — it only reads
status.json and shells out to the `bb-vpn` CLI (user-context for
`enroll`). The system-domain bb-vpn LaunchDaemon owns everything in
`/Library/Application Support/bb-dpi/` except `inbox/` (mode 1733
sticky+world-writable drop-box, set by the .pkg postinstall).

## Files

| File | Role |
|---|---|
| `BBVPN/BBVPNApp.swift` | `@main` entry point, `MenuBarExtra` scene, menu items |
| `BBVPN/StatusModel.swift` | `@MainActor` 5s-poll of `status.json` |
| `BBVPN/EnrollHandler.swift` | `bb-vpn://` URI handler — shells out to `bb-vpn enroll` |
| `Info.plist` | `CFBundleURLTypes` (registers `bb-vpn://`), `LSUIElement=true` |
| `build.sh` | universal binary (arm64+x86_64) → `BBVPN.app` bundle |

## Building

```
./client/menubar/build.sh
# → build/menubar/BBVPN.app  (unsigned, universal)
```

Invoked automatically by `make build-pkg` so the .pkg always ships a
fresh `BBVPN.app` aligned with its `bb-vpn` version.

## Auto-start

The .pkg installs `com.bb-dpi.bb-vpn-menubar.plist` to
`/Library/LaunchAgents/` and bootstraps it into the console user's GUI
domain on install. launchd auto-starts `BBVPN.app` at every login
(and restarts it if quit via **Quit** — `KeepAlive=true`). The
uninstall script boots the agent out and removes the plist.

## Out of scope

- **Code signing** — Phase 6 adds ad-hoc `codesign -s -`. Until then,
  first-launch needs the right-click → Open dance.
- **Localization** — single-locale (`en`).
