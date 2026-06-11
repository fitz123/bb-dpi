package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"bb-dpi/client/bb-vpn/pkg/state"
)

func statusCmd(args []string) int {
	var jsonOut bool
	fs := flag.NewFlagSet("bb-vpn status", flag.ContinueOnError)
	fs.BoolVar(&jsonOut, "json", false, "emit status.json verbatim instead of human format")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	s, err := state.ReadStatus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn status: %v\n", err)
		return exitSoftware
	}
	if jsonOut {
		data, _ := json.MarshalIndent(s, "", "  ")
		fmt.Println(string(data))
		return 0
	}
	// Human-readable.
	fmt.Printf("last_sync:            %s\n", or(s.LastSync, "never"))
	fmt.Printf("last_error:           %s\n", or(s.LastError, "(none)"))
	// LastFetchError surfaces a degraded fetch path (cached-bundle
	// fallback) independently of LastError. Without this line a
	// degraded tick where the control plane is unreachable would only
	// be visible via --json, which most operators won't think to try.
	fmt.Printf("last_fetch_error:     %s\n", or(s.LastFetchError, "(none)"))
	// Target falls back to the selector file when status.json predates
	// the field (or hasn't been stamped by a tick yet) so the operator
	// always sees the channel the NEXT tick will fetch.
	fmt.Printf("target:               %s\n", or(s.Target, string(state.ActiveTarget())))
	fmt.Printf("current_issued_at:    %s\n", or(s.CurrentIssuedAt, "(none)"))
	fmt.Printf("current_server_count: %d\n", s.CurrentServerCount)
	fmt.Printf("last_identity_change: %s\n", or(s.LastIdentityChange, "(none)"))
	return 0
}

func or(a, b string) string {
	if a == "" {
		return b
	}
	return a
}
