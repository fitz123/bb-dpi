// EnrollHandler — receives the `bb-vpn://enroll?uuid=…` URI from
// Launch Services and shells out to `bb-vpn enroll` (user-context,
// no sudo). Daemon lifecycle (start/stop/sync) is intentionally NOT
// in the menubar — those need root and live in the `bb-vpn` CLI for
// the operator to run with sudo.
//
// File paths exposed as static URLs so StatusModel can share them
// without re-deriving paths.

import AppKit
import Foundation

enum EnrollHandler {
    static let appSupportURL = URL(fileURLWithPath: "/Library/Application Support/bb-dpi")
    static let statusFileURL = appSupportURL.appendingPathComponent("status.json")
    // Absolute path to the bb-vpn binary the .pkg installs. Using the
    // private-path binary directly (not ~/.local/bin/bb-vpn) so the
    // menubar works even if the user's PATH or symlink is broken.
    static let bbvpnBin = "/Library/Application Support/bb-dpi/bin/bb-vpn"

    /// Called by Launch Services for `bb-vpn://enroll?uuid=…`. The CLI
    /// does URI parsing + UUID validation + inbox write.
    static func handleEnrollURL(_ url: URL) {
        guard url.scheme?.lowercased() == "bb-vpn",
              url.host?.lowercased() == "enroll" else {
            present("Not a bb-vpn enroll link: \(url.absoluteString)", title: "bb-vpn")
            return
        }
        let result = runBBVPN(["enroll", url.absoluteString])
        if result.status == 0 {
            present("Enrollment queued. The VPN will activate within a few seconds.",
                    title: "bb-vpn")
        } else {
            present(result.stderr.isEmpty ? "bb-vpn enroll failed (exit \(result.status))" : result.stderr,
                    title: "bb-vpn enroll error")
        }
    }

    // MARK: - internals

    private struct CLIResult {
        let status: Int32
        let stderr: String
    }

    private static func runBBVPN(_ args: [String]) -> CLIResult {
        let proc = Process()
        proc.executableURL = URL(fileURLWithPath: bbvpnBin)
        proc.arguments = args
        let errPipe = Pipe()
        proc.standardError = errPipe
        proc.standardOutput = Pipe()  // discard stdout
        // Watchdog: handleEnrollURL runs on the main thread (Launch Services
        // delivers the bb-vpn:// click via AppDelegate.application(_:open:)
        // in BBVPNApp.swift → here). A hung `bb-vpn enroll` (slow disk,
        // future enroll flow with a network call, etc.) would freeze the
        // menubar UI indefinitely. Cap at 10s — enroll is local-only
        // (validate UUID, write inbox/<uuid>.json), so 10s is well over
        // the realistic worst case and short enough to keep the UI
        // responsive.
        let exited = DispatchSemaphore(value: 0)
        proc.terminationHandler = { _ in exited.signal() }
        do {
            try proc.run()
        } catch {
            return CLIResult(status: -1, stderr: "spawn \(bbvpnBin): \(error.localizedDescription)")
        }
        if exited.wait(timeout: .now() + 10) == .timedOut {
            proc.terminate()
            return CLIResult(status: -1, stderr: "bb-vpn enroll timed out after 10s")
        }
        let data = (try? errPipe.fileHandleForReading.readToEnd()) ?? Data()
        let stderrText = String(data: data, encoding: .utf8) ?? ""
        return CLIResult(status: proc.terminationStatus, stderr: stderrText)
    }

    private static func present(_ message: String, title: String) {
        let alert = NSAlert()
        alert.messageText = title
        alert.informativeText = message
        alert.addButton(withTitle: "OK")
        // LSUIElement=true apps don't get key focus automatically;
        // without explicit activation, runModal() can render the sheet
        // behind whichever app delivered the bb-vpn:// URL.
        NSApp.activate(ignoringOtherApps: true)
        alert.runModal()
    }
}

// URL dispatch (Launch Services → bb-vpn://) is wired via
// NSApplicationDelegateAdaptor + AppDelegate.application(_:open:)
// in BBVPNApp.swift. The previous NSAppleEventManager registration
// in BBVPNApp.init() was removed — it failed to fire for LSUIElement
// (background) menubar apps on macOS 13/14 even though Launch
// Services successfully foregrounded BBVPN.app.
