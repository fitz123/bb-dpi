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
//
// Exit-server display: on each green tick, the model polls sing-box's
// clash-api (http://127.0.0.1:9090/proxies/auto) for the urltest's
// currently-selected outbound tag, and resolves the tag to a friendly
// (server name, host) via a tiny lazy cache of bundles/current.json.
// This is local-only — no third-party probes — and refreshes within
// ~5s of urltest swapping. Preserve-last-known on transient errors
// (connection refused mid-restart, parse failure, timeout) so the
// menubar doesn't flicker between a value and "—" every poll.

import Darwin
import Foundation
import SwiftUI

@MainActor
final class StatusModel: ObservableObject {
    @Published private(set) var snapshot: Snapshot = .grey
    @Published private(set) var currentOutbound: CurrentOutbound? = nil

    private var timer: Timer?
    // 5s polling so the icon reflects `sudo bb-vpn start/stop/sync`
    // and daemon-side state changes within a few seconds of click.
    // Cheap — status.json is ~200 bytes and the read is local.
    private let pollInterval: TimeInterval = 5
    private let staleAfter: TimeInterval = 30 * 60

    // Lazy serverName → host cache, sourced from
    // /Library/Application Support/bb-dpi/bundles/current.json. The
    // file is now 0o644 (Task 6) so the menubar can read it as the
    // console user. mtime gate avoids re-parsing on every tick.
    private var bundleMap: [String: String] = [:]
    private var bundleMapMTime: Date? = nil

    // URLSession dedicated to the clash-api probe with aggressive
    // localhost-grade timeouts. Healthy response on 127.0.0.1:9090 is
    // <50ms; a refused connection returns immediately; the timeout
    // only fires if sing-box is hung. We don't want the menubar's
    // run-loop sitting on the default 60s timeout when the daemon is
    // wedged — keep it snappy.
    private let clashSession: URLSession = {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.timeoutIntervalForRequest = 1.5
        cfg.timeoutIntervalForResource = 2.0
        // Belt-and-braces — never proxy a 127.0.0.1 call through the
        // system proxy (the tunnel we're trying to inspect is itself a
        // proxy). `connectionProxyDictionary = [:]` disables proxies
        // for this session.
        cfg.connectionProxyDictionary = [:]
        return URLSession(configuration: cfg)
    }()

    init() {
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

    // "name (host)" or "—" when no current pick is known (cold start,
    // urltest still converging, daemons down, parse error, …).
    var currentOutboundDisplay: String {
        guard let o = currentOutbound else { return "—" }
        return "\(o.name) (\(o.host))"
    }

    // MARK: - polling

    private func load() {
        let url = EnrollHandler.statusFileURL
        guard let data = try? Data(contentsOf: url),
              let raw = try? JSONDecoder().decode(StatusJSON.self, from: data) else {
            snapshot = .grey
            // status.json is unreadable → the tunnel state is unknown;
            // clear currentOutbound so the menubar shows "—" rather
            // than a stale pick from a prior healthy snapshot.
            currentOutbound = nil
            return
        }
        let next = Snapshot(from: raw, staleAfter: staleAfter)
        // Daemon liveness flips are the authoritative signal that the
        // tunnel actually moved (start, stop, crash). On a pure
        // state-only transition (sync error → yellow, etc.) keep the
        // last-known currentOutbound so the menubar doesn't blank out
        // a value that's still correct.
        let daemonStateChanged = next.singBoxRunning != snapshot.singBoxRunning
            || next.xrayRunning != snapshot.xrayRunning
        snapshot = next
        if daemonStateChanged {
            currentOutbound = nil
        }
        // Only probe clash-api when the VPN is presumed up. Yellow/grey
        // states mean the tunnel may be down, sing-box may not be
        // listening on :9090, and any answer we got would either be
        // stale or come from a daemon mid-restart.
        if next.state == .green {
            refreshBundleMapIfChanged()
            Task { @MainActor [weak self] in
                guard let self else { return }
                let result = await self.fetchClashCurrentOutbound()
                // The async hop can land seconds after a state
                // transition flipped the snapshot away from .green
                // (tunnel dropped while the request was in flight).
                // Drop late results when we're no longer green to
                // avoid stamping a stale pick over a now-down tunnel.
                guard self.snapshot.state == .green else { return }
                switch result {
                case .success(.some(let tag)):
                    // Tags from pkg/render/singbox.go are `xhttp-<name>`
                    // or `tcp-<name>` (both prefixed). Strip the prefix
                    // to recover the server name, then look up the host
                    // in the bundle map. If the tag doesn't match any
                    // known prefix or the name isn't in the map, keep
                    // the last value — the bundle is likely drifting
                    // from the running sing-box config and the next
                    // mtime refresh will reconcile.
                    let name = Self.stripOutboundPrefix(tag)
                    if let host = self.bundleMap[name] {
                        self.currentOutbound = CurrentOutbound(name: name, host: host)
                    }
                case .success(.none):
                    // urltest hasn't converged yet (typical in the first
                    // ~30s after sing-box start). Keep the last value.
                    break
                case .failure:
                    // Connection refused, timeout, parse failure, bad
                    // status — keep the last value. The next tick will
                    // retry; preserving prevents flicker.
                    break
                }
            }
        }
    }

    // Strip `xhttp-` or `tcp-` from a clash-api outbound tag. Returns
    // the original tag if no known prefix matches (callers treat that
    // as a lookup miss, not a crash).
    private static func stripOutboundPrefix(_ tag: String) -> String {
        if tag.hasPrefix("xhttp-") { return String(tag.dropFirst("xhttp-".count)) }
        if tag.hasPrefix("tcp-")   { return String(tag.dropFirst("tcp-".count)) }
        return tag
    }

    // refreshBundleMapIfChanged stats bundles/current.json and re-parses
    // only when mtime changes (cold start or after a sync tick that
    // rotated the bundle). Decode failures collapse to an empty map —
    // the menubar shows "—" until a healthy bundle lands, never crash.
    private func refreshBundleMapIfChanged() {
        let path = "/Library/Application Support/bb-dpi/bundles/current.json"
        let url = URL(fileURLWithPath: path)
        let mtime: Date? = (try? FileManager.default.attributesOfItem(atPath: path))?[.modificationDate] as? Date
        guard let mtime else {
            // File missing or unreadable — leave whatever cache we
            // have. Status will fall through to "—" if the cache is
            // empty.
            return
        }
        if let prev = bundleMapMTime, prev == mtime { return }
        bundleMapMTime = mtime

        guard let data = try? Data(contentsOf: url) else { return }
        guard let parsed = try? JSONDecoder().decode(BundleMinimal.self, from: data) else {
            bundleMap = [:]
            return
        }
        var next: [String: String] = [:]
        for s in parsed.servers {
            next[s.name] = s.host
        }
        bundleMap = next
    }

    // fetchClashCurrentOutbound queries sing-box's clash-api for the
    // urltest's currently-selected outbound tag. Returns:
    //   - .success(.some(tag)) for non-empty `now`
    //   - .success(.none) for empty `now` (urltest not converged)
    //   - .failure for network/parse error / non-2xx
    private func fetchClashCurrentOutbound() async -> Result<String?, Error> {
        guard let url = URL(string: "http://127.0.0.1:9090/proxies/auto") else {
            return .failure(NSError(domain: "BBVPN.clash", code: -1,
                                    userInfo: [NSLocalizedDescriptionKey: "bad URL"]))
        }
        do {
            let (data, response) = try await clashSession.data(from: url)
            if let http = response as? HTTPURLResponse, !(200...299).contains(http.statusCode) {
                return .failure(NSError(domain: "BBVPN.clash", code: http.statusCode,
                                        userInfo: [NSLocalizedDescriptionKey: "http \(http.statusCode)"]))
            }
            let decoded = try JSONDecoder().decode(ClashProxy.self, from: data)
            let now = decoded.now ?? ""
            return .success(now.isEmpty ? nil : now)
        } catch {
            return .failure(error)
        }
    }

    // MARK: - inner types

    enum State { case green, yellow, grey }

    struct CurrentOutbound: Equatable {
        let name: String
        let host: String
    }

    // BundleMinimal is the minimal projection of bundles/current.json
    // we need to map outbound tags → server hosts. Anything outside
    // `servers[].name/host` is ignored. We do NOT use
    // DisallowUnknownFields here — bundle schema changes shouldn't
    // crash the menubar; only the daemon-side parser is strict.
    private struct BundleMinimal: Decodable {
        struct Server: Decodable {
            let name: String
            let host: String
        }
        let servers: [Server]
    }

    // ClashProxy is the minimal projection of /proxies/auto. The full
    // response carries `all`, `history`, `type`, etc.; we only need
    // `now`.
    private struct ClashProxy: Decodable {
        let now: String?
    }

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
