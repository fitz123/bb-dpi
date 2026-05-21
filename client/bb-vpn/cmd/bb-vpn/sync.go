package main

import (
	"fmt"
	"os"
	"time"

	"bb-dpi/client/bb-vpn/pkg/launchctl"
	"bb-dpi/client/bb-vpn/pkg/state"
)

// syncCmd is `sudo bb-vpn sync` — invoked as root by the
// com.bb-dpi.bb-vpn-sync LaunchDaemon every 15 minutes, or directly
// by the operator from a terminal for an immediate tick.
//
// Honors BB_VPN_BIN_DIR (override the binary lookup dir; production
// is state.Path("bin")) and BB_VPN_DEV (skip kickstart calls for
// dev-mode use on macold or similar).
//
// Log lines: every line written to stderr begins with a UTC RFC3339
// timestamp so /Library/Logs/bb-dpi/bb-vpn-sync.log answers "when did
// this tick happen" directly. The OK line also carries duration_ms
// (so a slow control-plane fetch / kickstart is visible at a glance)
// and surfaces inbox_drain_err when that non-fatal failure occurs —
// previously inbox-drain errors only appeared on status.json.
func syncCmd(args []string) int {
	_ = args // sync takes no flags today
	start := time.Now().UTC()

	if os.Geteuid() != 0 {
		logSync(start, "requires root (run via launchd or sudo)")
		return exitUsage
	}

	binDir := os.Getenv("BB_VPN_BIN_DIR")
	if binDir == "" {
		binDir = state.Path(state.BinDir)
	}
	opts := launchctl.SyncOptions{
		BinDir:  binDir,
		DevMode: os.Getenv("BB_VPN_DEV") == "1",
	}
	res := launchctl.Tick(opts)
	durMS := time.Since(start).Milliseconds()
	if res.Err != nil {
		logSync(start, fmt.Sprintf("error duration_ms=%d: %v", durMS, res.Err))
		if res.BlackholeEntered {
			logSync(start, "entered runtime_blackhole — run `sudo bb-vpn recover` to recover")
			return exitSoftware
		}
		return exitSoftware
	}
	// Build the OK line. inbox_drain_err is non-fatal but worth
	// surfacing alongside the OK so a perms accident on inbox/ is
	// visible without reading status.json separately.
	msg := fmt.Sprintf("ok (duration_ms=%d issued_at=%s servers=%d xray=%v rendered=%v promoted=%v kickstarted=%v",
		durMS, res.BundleIssuedAt, res.ServerCount, res.XrayNeeded, res.Rendered, res.Promoted, res.Kickstarted)
	if res.InboxDrainErr != nil {
		msg += fmt.Sprintf(" inbox_drain_err=%q", res.InboxDrainErr.Error())
	}
	msg += ")"
	logSync(start, msg)
	return 0
}

// logSync writes "<RFC3339Nano UTC> bb-vpn sync: <msg>\n" to stderr.
// The launchd plist redirects stderr to /Library/Logs/bb-dpi/bb-vpn-sync.log,
// so this is the canonical log entrypoint for the sync daemon. The
// timestamp uses RFC3339 with millisecond precision — enough to
// distinguish back-to-back ticks (e.g., an enroll-triggered WatchPaths
// sync immediately followed by a forced sync) without the noise of
// nanoseconds.
func logSync(t time.Time, msg string) {
	fmt.Fprintf(os.Stderr, "%s bb-vpn sync: %s\n", t.Format("2006-01-02T15:04:05.000Z"), msg)
}
