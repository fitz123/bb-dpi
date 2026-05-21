package main

import (
	"fmt"
	"io"
	"os"
	"strings"
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
// Log lines: every physical line written to stderr begins with a UTC
// RFC3339 timestamp (millisecond precision) so
// /Library/Logs/bb-dpi/bb-vpn-sync.log answers "when did this tick
// happen" directly. The OK line also carries duration_ms (so a slow
// control-plane fetch / kickstart is visible at a glance) and
// surfaces inbox_drain_err when that non-fatal failure occurs —
// previously inbox-drain errors only appeared on status.json.
func syncCmd(args []string) int {
	_ = args // sync takes no flags today
	// time.Now() preserves Go's monotonic clock reading; .UTC()
	// would strip it and make time.Since() wall-clock-based, which
	// could produce wrong (or negative) duration_ms if NTP stepped
	// the clock during the tick. logSync stamps its own UTC emit
	// time, so we don't need a UTC `start` here anyway.
	start := time.Now()

	if os.Geteuid() != 0 {
		logSync("requires root (run via launchd or sudo)")
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
		logSync(fmt.Sprintf("error duration_ms=%d: %v", durMS, res.Err))
		if res.BlackholeEntered {
			logSync("entered runtime_blackhole — run `sudo bb-vpn recover` to recover")
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
	logSync(msg)
	return 0
}

// logSyncOut is the destination logSync writes to. Defaults to
// os.Stderr (which the launchd plist redirects to
// /Library/Logs/bb-dpi/bb-vpn-sync.log). Tests rewire it to a buffer.
var logSyncOut io.Writer = os.Stderr

// logSync writes one or more physical lines to logSyncOut, each
// prefixed with the current UTC timestamp (millisecond precision) and
// the "bb-vpn sync:" component tag.
//
// The timestamp is stamped at emit time (not call time captured by the
// caller) so it can't lag behind: a slow tick that captured start at
// T0 and only reaches logSync at T0+10s will stamp T0+10s on its OK
// line, matching when the line actually hit the log.
//
// msg may contain embedded newlines (e.g., sing-box check / xray -test
// dumps multi-line stderr into res.Err). Splitting+prefixing per
// physical line keeps the log grep-friendly: every line that lands in
// bb-vpn-sync.log is timestamped, not just the first.
func logSync(msg string) {
	t := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	// Drop the trailing newline so we don't emit a spurious empty
	// prefixed line at the end; the per-line loop adds its own \n.
	msg = strings.TrimRight(msg, "\n")
	for _, line := range strings.Split(msg, "\n") {
		fmt.Fprintf(logSyncOut, "%s bb-vpn sync: %s\n", t, line)
	}
}
