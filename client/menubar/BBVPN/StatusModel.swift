// StatusModel — polls /Library/Application Support/bb-dpi/status.json
// every 5 seconds (see pollInterval below) and exposes the menu-bar UI
// surface (color/header/timestamps). Read-only — never writes to /Library/.
//
// Status JSON shape (matches pkg/state/status.go on the daemon side).
// We only decode the four fields the menu bar actually consumes:
//   {
//     "last_sync":            "2026-05-20T16:44:01Z",
//     "last_error":           "(none) or key",
//     "last_fetch_error":     "(none) or key",
//     "current_issued_at":    "2026-05-17T22:25:04Z or empty",
//     ...                    (other fields ignored)
//   }
//
// We don't track identity.json directly — the daemon's status fields
// already reflect identity presence via the empty/non-empty
// current_issued_at + last_error="no_identity" pattern.

import Darwin
import Foundation
import SwiftUI

@MainActor
final class StatusModel: ObservableObject {
    @Published private(set) var snapshot: Snapshot = .grey
    @Published private(set) var exitCountry: String? = nil

    private var timer: Timer?
    // 5s polling so the icon reflects `sudo bb-vpn start/stop/sync`
    // and daemon-side state changes within a few seconds of click.
    // Cheap — status.json is ~200 bytes and the read is local.
    private let pollInterval: TimeInterval = 5
    private let staleAfter: TimeInterval = 30 * 60

    init() {
        // Don't preload exit country here — the first load() below
        // will trigger refreshExitCountry() via the daemonStateChanged
        // check (snapshot starts as .grey with nil daemon states;
        // real values are always != nil, so the first read fires it).
        load()
        // Register the timer on RunLoop.main in `.common` modes so it
        // keeps firing while the user has the menulet open or an NSAlert
        // is up (both push the run loop into `.eventTracking` /
        // `.modalPanel`, which suspends `.default`-only timers).
        let t = Timer(timeInterval: pollInterval, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.load() }
        }
        RunLoop.main.add(t, forMode: .common)
        timer = t
        // No periodic exit-country timer — we re-fetch only when the
        // daemon state changes (start/stop or a fresh sync), driven
        // from load() below. ifconfig.co rate-limits aggressive polling.
    }

    deinit {
        timer?.invalidate()
    }

    var color: Color {
        switch snapshot.state {
        case .green:  return .green
        case .yellow: return .yellow
        case .grey:   return .secondary
        }
    }

    var headerText: String {
        switch snapshot.state {
        case .green:  return "bb-vpn: connected"
        case .yellow: return "bb-vpn: degraded"
        case .grey:
            // Two distinct grey sub-states share the same color but
            // need different headers: an intentional `sudo bb-vpn stop`
            // (isStopped=true) vs. "no identity / fresh install"
            // (isStopped=false). Without this branch, the operator
            // can't tell from the menu whether they need to enroll or
            // just hit `sudo bb-vpn start`.
            return snapshot.isStopped ? "bb-vpn: stopped" : "bb-vpn: not enrolled"
        }
    }

    var accessibilityLabel: String { headerText }

    var lastSyncDisplay: String? {
        guard let ts = snapshot.lastSync else { return nil }
        return ts.formatted(date: .omitted, time: .standard)
    }

    // Centralized suppression of error lines when state == .grey.
    // The "not enrolled" header already conveys the situation, and a
    // grey snapshot can carry a sentinel `last_error = "no_identity"`
    // (or a stale `last_fetch_error` from the prior identity) that
    // would otherwise render a redundant "error: no_identity" / "fetch
    // error: …" line under "bb-vpn: not enrolled". Suppress at this
    // accessor so both menuContent error rows go away in lockstep.
    var lastErrorDisplay: String? {
        guard snapshot.state != .grey,
              let e = snapshot.lastError, !e.isEmpty else { return nil }
        return e
    }

    var lastFetchErrorDisplay: String? {
        guard snapshot.state != .grey,
              let e = snapshot.lastFetchError, !e.isEmpty else { return nil }
        return e
    }

    // "running" / "stopped" / "—" for the menu rows. nil = daemon
    // hasn't written the field yet (old status.json or pre-Phase-5);
    // collapse to "—" so the row still renders consistently.
    var singBoxRunningDisplay: String {
        switch snapshot.singBoxRunning {
        case .some(true):  return "running"
        case .some(false): return "stopped"
        case .none:        return "—"
        }
    }

    var xrayRunningDisplay: String {
        switch snapshot.xrayRunning {
        case .some(true):  return "running"
        case .some(false): return "stopped"
        case .none:        return "—"
        }
    }

    var exitCountryDisplay: String {
        exitCountry ?? "—"
    }

    // MARK: - polling

    // bootTime returns the current kernel boot time via
    // `sysctl kern.boottime`. Used by load() to reject any last_sync
    // timestamp from before the current boot — the only correct way to
    // tell "this status.json was written by THIS boot's daemon" apart
    // from "this is a leftover from before reboot". A purely time-based
    // window (e.g. "sync within the last 5 minutes") leaks the
    // operator's direct public IP through ifconfig.co on fast reboots:
    // if the Mac comes up <5min after the prior healthy tick,
    // status.json still shows state=.green + a fresh-looking last_sync,
    // but sing-box hasn't been kickstarted yet (RunAtLoad=false), so
    // the menubar's exit-country probe goes through the direct
    // connection instead of the tunnel. Anchoring against the kernel's
    // boottime makes the gate robust regardless of how recent the
    // pre-reboot sync was.
    //
    // Returns nil on sysctl failure — caller treats that as "can't
    // prove sync is post-boot" and suppresses the country probe.
    private static func bootTime() -> Date? {
        var tv = timeval()
        var size = MemoryLayout<timeval>.size
        var mib: [Int32] = [CTL_KERN, KERN_BOOTTIME]
        let rc = mib.withUnsafeMutableBufferPointer { mibPtr -> Int32 in
            sysctl(mibPtr.baseAddress, 2, &tv, &size, nil, 0)
        }
        if rc != 0 { return nil }
        return Date(timeIntervalSince1970: TimeInterval(tv.tv_sec))
    }

    private func load() {
        let url = EnrollHandler.statusFileURL
        guard let data = try? Data(contentsOf: url),
              let raw = try? JSONDecoder().decode(StatusJSON.self, from: data) else {
            snapshot = .grey
            // Reset exit country alongside the snapshot: when status.json
            // is unreadable or corrupt the tunnel state is unknown, and
            // displaying a stale country (from a prior healthy snapshot)
            // would lie about the current exit. Menubar renders "—".
            exitCountry = nil
            return
        }
        let next = Snapshot(from: raw, staleAfter: staleAfter)
        // Trigger an exit-country re-fetch when daemon liveness OR the
        // top-level state flips. Daemon liveness alone isn't enough:
        // state-only transitions (green → yellow from fetch error,
        // green → grey from no_identity, etc.) can happen without a
        // daemon process flipping, and would leave stale exitCountry
        // text in the menulet. last_sync changes every 15min on a
        // no-op tick but doesn't move the state, so we skip pure
        // last_sync churn to avoid hammering ifconfig.co.
        let daemonStateChanged = next.singBoxRunning != snapshot.singBoxRunning
            || next.xrayRunning != snapshot.xrayRunning
        let stateChanged = next.state != snapshot.state
        snapshot = next
        if daemonStateChanged || stateChanged {
            // Only probe ifconfig.co when the VPN is presumed up (green).
            // Yellow/grey states mean the tunnel may be down (manually stopped,
            // not enrolled, daemons crashed) — fetching the public IP through
            // a third-party service in those states would leak the operator's
            // direct (un-tunneled) public IP. Clear the country so the menubar
            // renders "exit country: —" instead of stale data.
            //
            // Post-reboot staleness gate: after reboot, BBVPN.app's
            // LaunchAgent fires at login and reads the pre-reboot
            // status.json which may still show state=.green from the
            // last live tick. sing-box's plist has RunAtLoad=false, so
            // the daemon ISN'T actually up yet — firing
            // refreshExitCountry here would send the request over the
            // direct connection, leaking the operator's real public IP
            // to ifconfig.co. bb-vpn-sync's RunAtLoad fires immediately
            // at login and writes a fresh status.json within ~30s.
            //
            // Gate on kernel boot time, not a fixed staleness window:
            // a purely time-based gate (e.g. "sync within 5 minutes")
            // still leaks on fast reboots (Mac comes back up <5min
            // after the prior tick → pre-reboot last_sync looks fresh
            // but sing-box hasn't started yet). Anchoring against
            // `kern.boottime` rejects any sync timestamp from before
            // the current boot regardless of how recent it is, which
            // is the actual invariant we need.
            //
            // If bootTime() returns nil (sysctl failure — shouldn't
            // happen on macOS but be safe), treat the sync as not
            // post-boot and suppress the probe rather than risk a leak.
            let sysBoot = StatusModel.bootTime()
            let postBoot = next.lastSync.map { sync in
                sysBoot.map { sync > $0 } ?? false
            } ?? false
            if next.state == .green && postBoot {
                refreshExitCountry()
            } else {
                exitCountry = nil
            }
        }
    }

    // refreshExitCountry fires a 3s-timeout request to ifconfig.co/json
    // and publishes the parsed `country` field. We show country instead
    // of IP so the menubar isn't a PII screen — the operator sees
    // "country: Finland" (VPN up) or "country: Russia" (VPN down) at
    // a glance. Errors set exitCountry to nil → menubar shows "—".
    private func refreshExitCountry() {
        guard let url = URL(string: "https://ifconfig.co/json") else { return }
        var req = URLRequest(url: url)
        req.timeoutInterval = 3
        URLSession.shared.dataTask(with: req) { [weak self] data, _, _ in
            let country: String? = {
                guard let data,
                      let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                      let s = obj["country"] as? String,
                      !s.isEmpty, s.count <= 100 else { return nil }
                return s
            }()
            Task { @MainActor in
                guard let self else { return }
                // The dataTask callback can land seconds after a state
                // transition flipped the snapshot away from .green
                // (e.g. tunnel dropped while the request was in
                // flight). Publishing the result regardless would
                // overwrite exitCountry with a value resolved through
                // the now-down VPN — at worst stamping the operator's
                // direct public-IP country into a UI row labelled
                // "exit country" while .yellow/.grey is rendered.
                // Drop late callbacks when we're no longer green.
                guard self.snapshot.state == .green else { return }
                self.exitCountry = country
            }
        }.resume()
    }

    // MARK: - inner types

    enum State { case green, yellow, grey }

    struct Snapshot {
        let state: State
        let lastSync: Date?
        let lastError: String?
        let lastFetchError: String?
        let singBoxRunning: Bool?
        let xrayRunning: Bool?
        // True when status.json's last_error == "manually_stopped" —
        // i.e. the operator ran `sudo bb-vpn stop` and the daemons are
        // intentionally down. Render as grey ("stopped"), distinct from
        // grey ("not enrolled") via headerText. Default false so the
        // synthesized memberwise init + `.grey` static still compile
        // without callers needing to pass it explicitly.
        let isStopped: Bool

        // The struct intentionally has no explicit init — Swift
        // synthesizes the memberwise init, which `.grey` uses directly.
        // The `from:staleAfter:` convenience initializer lives in an
        // extension below so it doesn't suppress that synthesis.
        static let grey = Snapshot(
            state: .grey,
            lastSync: nil,
            lastError: nil,
            lastFetchError: nil,
            singBoxRunning: nil,
            xrayRunning: nil,
            isStopped: false
        )
    }

    struct StatusJSON: Decodable {
        let last_sync: String
        let last_error: String
        // `omitempty` on the daemon side (pkg/state/status.go) — the
        // key vanishes from status.json when LastFetchError is empty.
        // Treat absent as empty string.
        let last_fetch_error: String?
        let current_issued_at: String
        // Optional in case the daemon hasn't been bumped yet — old
        // status.json files don't have these.
        let sing_box_running: Bool?
        let xray_running: Bool?
        // xray_needed: true iff the most recent render produced an
        // xray config. tcp-vision-only fleets legitimately have
        // xray_running=false + xray_needed=false → green, not yellow.
        // Optional for forward-compat with old daemons that don't
        // emit the field (treat absent as "unknown" → fall back to
        // the legacy rule that any xray_running=false implies yellow).
        let xray_needed: Bool?

        func parsedLastSync() -> Date? {
            guard !last_sync.isEmpty else { return nil }
            let f = ISO8601DateFormatter()
            return f.date(from: last_sync)
        }
    }
}

// `from:staleAfter:` lives in an extension so the main struct keeps its
// synthesized memberwise init (used by `.grey`).
extension StatusModel.Snapshot {
    init(from raw: StatusModel.StatusJSON, staleAfter: TimeInterval) {
        let last = raw.parsedLastSync()
        self.lastSync = last
        let fetchErr = raw.last_fetch_error ?? ""
        self.lastFetchError = fetchErr.isEmpty ? nil : fetchErr
        self.singBoxRunning = raw.sing_box_running
        self.xrayRunning = raw.xray_running

        // "manually_stopped" is the sentinel the daemon writes after
        // `sudo bb-vpn stop`. It's not a real error — the operator
        // deliberately took the daemons down. Render as a third grey
        // sub-state ("stopped") so it's visually distinct from a
        // crashed daemon (yellow "degraded"). Suppress the sentinel
        // from lastError so the redundant "error: manually_stopped"
        // row in the menu doesn't appear (also guarded centrally by
        // lastErrorDisplay's state == .grey check, but belt-and-braces
        // in case future code paths read .lastError directly).
        if raw.last_error == "manually_stopped" {
            self.lastError = nil
            self.isStopped = true
            self.state = .grey
            return
        }

        self.lastError = raw.last_error.isEmpty ? nil : raw.last_error
        self.isStopped = false

        // grey = "not enrolled / pre-first-sync". Two distinct triggers:
        //
        // 1. last_error == "no_identity" — daemon explicitly reports
        //    identity.json is missing. This trumps any stale
        //    current_issued_at (post-uninstall + re-enroll window where
        //    issued_at lingers from the prior identity): if the daemon
        //    says "no identity", the UI must say "not enrolled".
        //
        // 2. current_issued_at is empty AND last_error is empty — fresh
        //    install pre-first-sync, no signal of any kind. Show grey
        //    ("not enrolled") rather than yellow (which implies a real
        //    failure to investigate).
        //
        // A fresh install that enrolled successfully but failed the
        // first bundle fetch (last_error="fetch_failed",
        // "control_plane_unreadable", etc.) has current_issued_at empty
        // AND a non-empty, non-"no_identity" last_error — those are real
        // errors the operator must see, so they fall through to yellow.
        if raw.last_error == "no_identity" {
            self.state = .grey
            return
        }
        if raw.current_issued_at.isEmpty && raw.last_error.isEmpty {
            self.state = .grey
            return
        }

        // yellow if any error, or last_sync stale, or fetch error, or
        // either daemon reported not running by the last tick (the
        // operator probably ran `sudo bb-vpn stop` or a daemon crashed).
        //
        // xray_running=false only counts as "down" when the most recent
        // render actually produced an xray config (xray_needed=true).
        // tcp-vision-only fleets render no xray config and sync.Tick
        // bootouts the daemon — that's a healthy steady state, not
        // degraded. When xray_needed is unknown (old daemon pre-Phase-5),
        // fall back to the legacy rule of treating any xray_running=false
        // as "down" so we don't silently downgrade real failures.
        let stale = last.map { Date().timeIntervalSince($0) > staleAfter } ?? true
        let hasErr = !raw.last_error.isEmpty
        let hasFetchErr = !fetchErr.isEmpty
        let sbDown = raw.sing_box_running == false
        let xrDown: Bool = {
            guard raw.xray_running == false else { return false }
            // xray reported not-running. Whether that's degraded depends
            // on whether this fleet renders xray at all.
            switch raw.xray_needed {
            case .some(false): return false // expected on tcp-vision-only
            case .some(true), .none: return true
            }
        }()
        self.state = (stale || hasErr || hasFetchErr || sbDown || xrDown) ? .yellow : .green
    }
}
