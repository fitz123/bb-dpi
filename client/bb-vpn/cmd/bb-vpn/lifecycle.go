package main

import (
	"fmt"
	"os"

	"bb-dpi/client/bb-vpn/pkg/launchctl"
	"bb-dpi/client/bb-vpn/pkg/state"
)

// startCmd is `sudo bb-vpn start`: clears the manually_stopped flag
// and kickstarts sing-box + xray from the live configs. Returns 0
// after both kickstarts complete.
//
// `launchctl kickstart -k` blocks ~5–10s while sing-box sets up the
// TUN device and runs initial urltest probes — print progress so the
// operator doesn't think the command hung.
//
// xray failure is fatal IFF configs/xray.json exists (i.e., the fleet
// needs xray). If xray.json is absent (tcp-vision-only render), xray
// kickstart is skipped entirely — EnsureRunning() against a missing
// config would either fail or crash-loop the LaunchDaemon (KeepAlive=
// true). sing-box failure is always fatal (no VPN without it).
//
// startCmd does NOT `launchctl enable` either daemon. The sync.Tick
// runtime_blackhole circuit breaker (pkg/launchctl/sync.go) calls
// `launchctl disable` on both daemons when a smoke test fails on a
// broken bundle; that disable persists across reboot by design. The
// dedicated recovery path is `sudo bb-vpn recover` — running `bb-vpn
// start` (or postinstall auto-calling it) must NOT resurrect a known-
// bad bundle by re-enabling the daemons behind the circuit breaker.
//
// Note (deferred): startCmd does NOT re-run `sing-box check` / `xray
// -test` before kickstart. The pre-restart validation lives in
// sync.Tick step 4, so the live configs/ files have already been
// validated by the most recent successful tick. The two paths that
// reach this code are:
//   - postinstall (guarded by `[[ -f identity.json ]]` in postinstall
//     — implies enrollment happened and at least one sync.Tick already
//     promoted + validated configs/)
//   - terminal `sudo bb-vpn start` (operator-initiated; if they hand-
//     edited configs/ behind sync.Tick's back, that's their call)
//
// In both cases configs/ is sync.Tick-managed, so re-validating here
// would be redundant cost on the hot start path.
func startCmd(args []string) int {
	_ = args
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "bb-vpn start: requires root (run via sudo)")
		return exitUsage
	}
	if err := state.ClearManuallyStopped(); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn start: clear stopped flag: %v\n", err)
		return exitSoftware
	}
	// NB: deliberately no `launchctl enable` here. stopCmd uses Bootout
	// (not Disable), so the daemons stay enabled across stop/start. The
	// only path that disables them is sync.Tick's runtime_blackhole
	// circuit breaker — and that requires `sudo bb-vpn recover` to clear,
	// not `bb-vpn start`. Re-enabling here would silently resurrect a
	// known-bad bundle behind the circuit breaker.
	// Order matters: xray FIRST, then sing-box. xray serves the SOCKS
	// proxies sing-box's urltest probes on start; bringing sing-box up
	// first makes it probe a not-yet-running xray and mark those outbounds
	// dead (see pkg/launchctl/sync.go Step 7 for the full rationale).
	//
	// Only kickstart xray when there's a rendered xray config — file
	// existence is the simplest proxy for "this fleet needs xray". A
	// tcp-vision-only render leaves no configs/xray.json, and sync.Tick
	// already booted out the daemon when XrayNeeded went false.
	//
	// When xray.json exists, xray IS needed: an XHTTP fleet without
	// xray running has its SOCKS outbound dead, so sing-box's urltest
	// falls back to tcp-vision only (or fully degraded). Treating that
	// as success would lie to the operator — return exitSoftware so
	// `bb-vpn start` accurately reflects a partial start.
	if _, err := os.Stat(state.Path(state.XrayConfig)); err == nil {
		fmt.Println("bb-vpn start: kickstarting xray...")
		if err := launchctl.EnsureRunning(launchctl.Xray); err != nil {
			fmt.Fprintf(os.Stderr, "bb-vpn start: kickstart xray: %v\n", err)
			updateRunningStatus(false)
			return exitSoftware
		}
	} else if os.IsNotExist(err) {
		fmt.Println("bb-vpn start: no xray.json (tcp-vision-only fleet), skipping xray")
	} else {
		fmt.Fprintf(os.Stderr, "bb-vpn start: stat xray.json: %v\n", err)
		updateRunningStatus(false)
		return exitSoftware
	}
	fmt.Println("bb-vpn start: kickstarting sing-box (~5s for TUN + probes)...")
	if err := launchctl.EnsureRunning(launchctl.SingBox); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn start: kickstart sing-box: %v\n", err)
		// Surface real daemon state to the menubar — without this,
		// status.json keeps the stale LastError="manually_stopped"
		// from before the start attempt and the menubar shows grey
		// "stopped" instead of yellow "degraded" for the failed start.
		// ClearManuallyStopped() ran above so the sentinel is gone;
		// re-Print()ing the daemons via updateRunningStatus reflects
		// the actual (failed) liveness.
		updateRunningStatus(false)
		return exitSoftware
	}
	// Refresh status.json so the menubar shows the new running state
	// immediately instead of waiting for the next sync tick (~15 min).
	updateRunningStatus(false)
	fmt.Println("bb-vpn start: done")
	return 0
}

// updateRunningStatus re-Print()s both daemons and updates the
// sing_box_running / xray_running fields in status.json. Called after
// start/stop so the menubar dot reflects reality within the next
// 5-second poll instead of the next 15-min sync tick.
//
// When stopped=true, also stamps LastError="manually_stopped" so the
// menubar can immediately render the grey "stopped" sub-state instead
// of inferring it only from daemon liveness (which the menubar can't
// distinguish from a crash). When stopped=false, only the
// "manually_stopped" sentinel is cleared from LastError — any other
// error state (validate_failed, kickstart_*_failed, …) is preserved
// so a subsequent `bb-vpn start` doesn't silently swallow real errors.
func updateRunningStatus(stopped bool) {
	s, err := state.ReadStatus()
	if err != nil {
		return // best-effort
	}
	s.SingBoxRunning, _ = launchctl.Print(launchctl.SingBox)
	s.XrayRunning, _ = launchctl.Print(launchctl.Xray)
	// Re-derive XrayNeeded from configs/xray.json existence — the same
	// proxy startCmd uses to decide whether to kickstart xray. Without
	// this, an upgrade path where status.json predates the XrayNeeded
	// field (or was last written by an older daemon) would round-trip
	// the Go zero value (false) and clobber the menubar's xray_needed
	// signal — causing the menubar to suppress the yellow "degraded"
	// badge on an XHTTP fleet where xray IS needed but isn't running.
	if _, err := os.Stat(state.Path(state.XrayConfig)); err == nil {
		s.XrayNeeded = true
	} else {
		s.XrayNeeded = false
	}
	if stopped {
		s.LastError = "manually_stopped"
	} else if s.LastError == "manually_stopped" {
		s.LastError = ""
	}
	_ = state.WriteStatus(s)
}

// stopCmd is `sudo bb-vpn stop`: bootouts sing-box + xray and persists
// the manually_stopped flag so subsequent `bb-vpn sync` ticks skip
// kickstart (i.e., the VPN stays down across reboots until `start`).
// Instant.
//
// Order matters: SetManuallyStopped() runs FIRST so a racing sync.Tick
// (fires every 15min plus on plist reload) sees the flag and respects
// it. The previous order (bootout → set-flag) had a small window where
// a tick could observe `!ManuallyStopped()` between the two Bootout
// calls and kickstart the daemons back up.
func stopCmd(args []string) int {
	_ = args
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "bb-vpn stop: requires root (run via sudo)")
		return exitUsage
	}
	if err := state.SetManuallyStopped(); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn stop: set stopped flag: %v\n", err)
		return exitSoftware
	}
	// sing-box bootout failure is fatal: the VPN may still be running,
	// so leaving manually_stopped=true would have status.json lie about
	// the real daemon state. Revert the flag and resync status to truth
	// before bailing.
	if err := launchctl.Bootout(launchctl.SingBox); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn stop: bootout sing-box: %v\n", err)
		_ = state.ClearManuallyStopped()
		updateRunningStatus(false)
		return exitSoftware
	}
	if err := launchctl.Bootout(launchctl.Xray); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn stop: bootout xray (non-fatal): %v\n", err)
	}
	updateRunningStatus(true)
	fmt.Println("bb-vpn stop: daemons stopped (use `sudo bb-vpn start` to resume)")
	return 0
}
