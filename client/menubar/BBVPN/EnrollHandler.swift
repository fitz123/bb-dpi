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
        // Discard stdout to /dev/null. An unread Pipe buffers the
        // child's writes and blocks on write() once the kernel buffer
        // fills (~16-64KB on macOS), which would hang the child while
        // the parent blocks on `exited.wait()` and force the watchdog
        // to terminate with a misleading "timeout" error.
        proc.standardOutput = FileHandle.nullDevice

        // Stderr feeds the user-visible NSAlert on failure, so we
        // can't discard it — but we also can't leave the Pipe unread
        // (same deadlock as stdout) or read synchronously *after*
        // `exited.wait()` returns (readToEnd would block forever if
        // a future grandchild inherits the writer fd, escaping the
        // watchdog). Drain concurrently via readabilityHandler:
        // bytes accumulate into stderrData while the child runs, and
        // we snapshot it after the child exits. The handler also
        // self-clears on EOF (empty chunk = child closed its writer),
        // and we defensively clear it again after the wait in case
        // the handler missed the final delivery due to scheduling.
        let errPipe = Pipe()
        proc.standardError = errPipe
        var stderrData = Data()
        let stderrLock = NSLock()
        // EOF is signaled by the readabilityHandler on its empty-chunk
        // delivery (writer-side close = child exit). NOTE_EXIT (which
        // signals `exited` below) and EVFILT_READ (which fires the
        // readabilityHandler) are independent kevents on separate GCD
        // queues — the termination semaphore CAN fire before the
        // handler has drained the bytes still buffered in the pipe.
        // Without this extra synchronization, runBBVPN can return an
        // empty stderr and the user sees "bb-vpn enroll failed (exit
        // N)" instead of the actual error string.
        let stderrEOF = DispatchSemaphore(value: 0)
        errPipe.fileHandleForReading.readabilityHandler = { handle in
            let chunk = handle.availableData
            if chunk.isEmpty {
                handle.readabilityHandler = nil
                stderrEOF.signal()
                return
            }
            stderrLock.lock()
            stderrData.append(chunk)
            stderrLock.unlock()
        }

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
            errPipe.fileHandleForReading.readabilityHandler = nil
            return CLIResult(status: -1, stderr: "spawn \(bbvpnBin): \(error.localizedDescription)")
        }
        if exited.wait(timeout: .now() + 10) == .timedOut {
            proc.terminate()
            // Still wait briefly for EOF — proc.terminate() closes
            // the child's stderr fd, which should unblock the
            // readabilityHandler with a final empty chunk. Bounded so
            // a leaked-fd grandchild can't pin us.
            _ = stderrEOF.wait(timeout: .now() + 0.1)
            errPipe.fileHandleForReading.readabilityHandler = nil
            stderrLock.lock()
            let captured = String(data: stderrData, encoding: .utf8) ?? ""
            stderrLock.unlock()
            // Preserve whatever stderr the child managed to emit
            // before hanging — that's the diagnostic the watchdog
            // exists to make visible.
            let msg = captured.isEmpty
                ? "bb-vpn enroll timed out after 10s"
                : "bb-vpn enroll timed out after 10s. Last stderr: \(captured)"
            return CLIResult(status: -1, stderr: msg)
        }
        // Normal exit: wait up to 100ms for the readabilityHandler to
        // drain final bytes. Bounded short — if EOF doesn't arrive in
        // this window, return what's been captured (a future grandchild
        // inheriting the fd would otherwise pin us indefinitely).
        _ = stderrEOF.wait(timeout: .now() + 0.1)
        errPipe.fileHandleForReading.readabilityHandler = nil

        stderrLock.lock()
        let stderrText = String(data: stderrData, encoding: .utf8) ?? ""
        stderrLock.unlock()
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
