package main

import (
	"fmt"
	"os"

	"bb-dpi/client/bb-vpn/pkg/launchctl"
	"bb-dpi/client/bb-vpn/pkg/state"
)

// recoverCmd clears the runtime_blackhole circuit breaker state:
//
//  1. launchctl enable system/com.sing-box-vpn (and xray)
//  2. archive bundles/current.json → bundles/blackhole-<ts>.json
//  3. clear status.json.last_error
//  4. force an immediate sync via syncCmd
//
// Must run as root (Disable/Enable + write to root-owned state files
// require it). Without explicit recovery the Mac stays dead-VPN
// forever — launchctl disable persists across reboot.
func recoverCmd(args []string) int {
	_ = args
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "bb-vpn recover: requires root (run via sudo)")
		return exitUsage
	}
	if err := launchctl.Enable(launchctl.SingBox); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn recover: enable sing-box: %v\n", err)
		return exitSoftware
	}
	if err := launchctl.Enable(launchctl.Xray); err != nil {
		// Xray enable failure is non-fatal — proto=tcp-vision flows
		// don't need xray running.
		fmt.Fprintf(os.Stderr, "bb-vpn recover: enable xray (non-fatal): %v\n", err)
	}
	if err := state.ArchiveBundleBlackhole(); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn recover: archive bundle: %v\n", err)
		return exitSoftware
	}
	s, _ := state.ReadStatus()
	s.LastError = ""
	_ = state.WriteStatus(s)

	fmt.Fprintln(os.Stderr, "bb-vpn recover: cleared circuit breaker; running immediate sync")
	return syncCmd(nil)
}
