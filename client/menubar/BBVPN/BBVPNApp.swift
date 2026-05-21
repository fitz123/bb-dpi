// BBVPNApp — bb-dpi menu-bar app entry point.
//
// Renders a MenuBarExtra (top-right menulet) with three states:
//   green   — enrolled, last sync ≤ 30 min ago, no errors
//   yellow  — enrolled but stale (>30 min) or last_error non-empty
//   grey    — not enrolled (status.json absent or missing identity)
//
// Menu items (see `menuContent` below):
//   "Show logs…"   — opens /Library/Logs/bb-dpi/ in Finder so the user
//                    can pick sing-box.log / xray.log / bb-vpn-sync.log
//                    (etc.) directly without `sudo cat` from a terminal.
//   "Quit"
//
// Daemon lifecycle (start / stop / sync) deliberately lives in the
// `bb-vpn` CLI (`sudo bb-vpn start|stop|sync`), not the menubar — the
// menubar runs non-elevated and is read-only against `/Library/`. The
// only write path it has is via `bb-vpn enroll` (see EnrollHandler),
// which the URL handler shells out to on `bb-vpn://enroll?uuid=…`
// receipt and which drops one `inbox/enroll-*.json` for the root
// daemon to ingest (drop-box is mode 1733 by the .pkg postinstall).

import AppKit
import SwiftUI

@main
struct BBVPNApp: App {
    // Launch Services delivers `bb-vpn://enroll?…` via NSApplication's
    // `application(_:open:)` delegate hook — the canonical AppKit URL
    // entry point. We bind it via NSApplicationDelegateAdaptor because
    // SwiftUI's `.onOpenURL` on `MenuBarExtra` does not reliably fire
    // for LSUIElement (background) apps on macOS 13/14: end-to-end
    // testing on macold showed Launch Services brought BBVPN.app to
    // the foreground (visible in syslog as SetFrontProcess) but the
    // SwiftUI scene-modifier never received the URL. The previous
    // NSAppleEventManager-in-init approach had the same problem.
    // application(_:open:) works because AppKit dispatches it
    // directly off the run loop regardless of scene/window state.
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var status = StatusModel()

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

        Button("Show logs…") {
            // Opens /Library/Logs/bb-dpi/ in Finder. Console.app
            // can read these too, but Finder lets the user pick a
            // file by name (sing-box.log, xray.log, bb-vpn-sync.log,
            // bb-vpn-menubar.log) and open it with their preferred
            // viewer.
            NSWorkspace.shared.open(EnrollHandler.logDirURL)
        }
        // Daemon lifecycle (start / stop / sync) lives in the
        // `bb-vpn` CLI — run from a terminal with sudo. Menubar is
        // status + URI enroll only.

        Divider()

        Button("Quit") { NSApp.terminate(nil) }
            .keyboardShortcut("q")
    }
}

// AppDelegate exists only to provide application(_:open:) — the
// AppKit hook Launch Services uses for `bb-vpn://` clicks. URLs
// arrive here as an array because macOS can deliver multiple at
// once (e.g., the user clicks several enroll links in quick
// succession before the app is fully booted); each is dispatched
// to EnrollHandler independently.
//
// `application(_:open:)` is also AppKit's catch-all for file:// URLs
// from `open -a BBVPN <files>` and Finder drag-and-drop — scopes the
// previous NSAppleEventManager + kInternetEventClass handler did
// NOT receive. Filter to bb-vpn:// only so a stray file drop doesn't
// fire N "Not a bb-vpn enroll link" modals serially on the main
// thread.
final class AppDelegate: NSObject, NSApplicationDelegate {
    func application(_ application: NSApplication, open urls: [URL]) {
        for url in urls where url.scheme?.lowercased() == "bb-vpn" {
            EnrollHandler.handleEnrollURL(url)
        }
    }
}
