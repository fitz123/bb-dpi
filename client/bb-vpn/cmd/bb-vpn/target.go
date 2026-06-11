package main

import (
	"fmt"
	"os"

	"bb-dpi/client/bb-vpn/pkg/state"
)

// geteuid is a seam for tests: the root-required write branch must be
// exercisable both as root and non-root regardless of who runs
// `go test`.
var geteuid = os.Geteuid

// targetCmd is `bb-vpn target [test|prod]`. With no argument it prints
// the active publish target (read-only, no root — mirrors `bb-vpn
// status`). With an argument it persists the selector via
// state.SetTarget, which requires root because the target file lives
// in the root-owned state tree (mirrors `sudo bb-vpn start/stop`).
//
// Setting the target deliberately does NOT trigger a sync — the
// subcommand stays single-purpose; the printed hint points the
// operator at `sudo bb-vpn sync` for an immediate flip, otherwise the
// next 15-min tick picks it up.
func targetCmd(args []string) int {
	t, err := parseTargetArg(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn target: %v\n", err)
		fmt.Fprintln(os.Stderr, "usage: bb-vpn target [test|prod]")
		return exitUsage
	}
	if t == "" {
		fmt.Println(state.ActiveTarget())
		return 0
	}
	if geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "bb-vpn target: setting the target requires root (run via sudo)")
		return exitUsage
	}
	if err := state.SetTarget(t); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn target: %v\n", err)
		return exitSoftware
	}
	fmt.Printf("bb-vpn target: now %s — run `sudo bb-vpn sync` to fetch it immediately\n", t)
	return 0
}

// parseTargetArg validates the `bb-vpn target` argv. Returns "" for
// the no-arg read mode, the validated target for a write, or an error
// for anything else (unknown value, extra args). Validation here maps
// a bad value to the usage exit; state.SetTarget re-validates as the
// last line of defense.
func parseTargetArg(args []string) (state.Target, error) {
	switch len(args) {
	case 0:
		return "", nil
	case 1:
		t := state.Target(args[0])
		if t != state.TargetProd && t != state.TargetTest {
			return "", fmt.Errorf("invalid target %q: must be %q or %q", args[0], state.TargetTest, state.TargetProd)
		}
		return t, nil
	default:
		return "", fmt.Errorf("unexpected extra args: %v", args[1:])
	}
}
