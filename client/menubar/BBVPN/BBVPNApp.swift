// BBVPNApp — bb-dpi menu-bar app entry point.
//
// Renders a MenuBarExtra (top-right menulet) with three states:
//   green   — enrolled, last sync ≤ 30 min ago, no errors
//   yellow  — enrolled but stale (>30 min) or last_error non-empty
//   grey    — not enrolled (status.json absent or missing identity)
//
// Menu items (see `menuContent` below):
//   "Show log…"    — opens status.json in the user's default JSON viewer.
//   "Quit"
//
// Daemon lifecycle (start / stop / sync) deliberately lives in the
// `bb-vpn` CLI (`sudo bb-vpn start|stop|sync`), not the menubar — the
// menubar runs non-elevated and is read-only against `/Library/`. The
// only write path it has is via `bb-vpn enroll` (see EnrollHandler),
// which the URL handler shells out to on `bb-vpn://enroll?uuid=…`
// receipt and which drops one `inbox/enroll-*.json` for the root
// daemon to ingest (drop-box is mode 1733 by the .pkg postinstall).

import SwiftUI

@main
struct BBVPNApp: App {
    @StateObject private var status = StatusModel()

    init() {
        // Wire Launch Services → EnrollHandler. Done once at process
        // start so the very first bb-vpn:// click after install lands
        // even though the user hasn't opened the app yet (MenuBarExtra
        // apps launch implicitly on first URL receipt).
        URLEventHandler.shared.register()
    }

    var body: some Scene {
        MenuBarExtra {
            menuContent
        } label: {
            // MenuBarExtra converts a single SF Symbol label to a template
            // image by default (NSImage.isTemplate=true), which forces
            // monochrome rendering. `.symbolRenderingMode(.palette)` tells
            // SF Symbols to honor the explicit foreground color instead.
            // Without `.palette`, the icon shows as a plain white/black
            // dot regardless of status. A plain Circle() shape collapses
            // to zero size in the MenuBarExtra label context (no
            // intrinsic content size) and renders nothing, so we stay
            // with the SF Symbol.
            Image(systemName: "circle.fill")
                .symbolRenderingMode(.palette)
                .foregroundStyle(status.color)
                .accessibilityLabel(status.accessibilityLabel)
        }
        .menuBarExtraStyle(.menu)
    }

    @ViewBuilder
    private var menuContent: some View {
        Text(status.headerText)
            .font(.system(size: 11, weight: .semibold))

        if let ts = status.lastSyncDisplay {
            Text("last sync: \(ts)")
                .font(.system(size: 11))
        }
        if let err = status.lastErrorDisplay {
            Text("error: \(err)")
                .font(.system(size: 11))
                .foregroundStyle(.secondary)
        }
        if let fetchErr = status.lastFetchErrorDisplay {
            Text("fetch error: \(fetchErr)")
                .font(.system(size: 11))
                .foregroundStyle(.secondary)
        }
        Text("sing-box:    \(status.singBoxRunningDisplay)")
            .font(.system(size: 11))
        Text("xray:        \(status.xrayRunningDisplay)")
            .font(.system(size: 11))
        Text("exit country: \(status.exitCountryDisplay)")
            .font(.system(size: 11))

        Divider()

        Button("Show log…") {
            NSWorkspace.shared.open(EnrollHandler.statusFileURL)
        }
        // Daemon lifecycle (start / stop / sync) lives in the
        // `bb-vpn` CLI — run from a terminal with sudo. Menubar is
        // status + URI enroll only.

        Divider()

        Button("Quit") { NSApp.terminate(nil) }
            .keyboardShortcut("q")
    }
}
