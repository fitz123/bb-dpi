package main

import (
	"fmt"
	"os"

	"bb-dpi/client/bb-vpn/pkg/launchctl"
	"bb-dpi/client/bb-vpn/pkg/state"
)

// syncCmd is `bb-vpn sync` — invoked as root by the
// com.bb-dpi.bb-vpn-sync LaunchDaemon every 15 minutes.
//
// Honors BB_VPN_BIN_DIR (override the binary lookup dir; production
// is state.Path("bin")) and BB_VPN_DEV (skip kickstart calls for
// dev-mode use on macold or similar).
func syncCmd(args []string) int {
	_ = args // sync takes no flags today

	// A user accidentally running `bb-vpn sync` from a terminal would
	// partially succeed (DrainInbox) then fail at WriteIdentity /
	// PromoteBundle with EACCES, persisting bogus error keys to
	// status.json. Reject early.
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "bb-vpn sync: requires root (run via launchd or sudo)")
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
	if res.Err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn sync: %v\n", res.Err)
		if res.BlackholeEntered {
			fmt.Fprintln(os.Stderr, "bb-vpn sync: entered runtime_blackhole — run `sudo bb-vpn recover` to recover")
			return exitSoftware
		}
		return exitSoftware
	}
	fmt.Fprintf(os.Stderr, "bb-vpn sync: ok (issued_at=%s servers=%d xray=%v rendered=%v promoted=%v kickstarted=%v)\n",
		res.BundleIssuedAt, res.ServerCount, res.XrayNeeded, res.Rendered, res.Promoted, res.Kickstarted)
	return 0
}
