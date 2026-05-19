package main

import (
	"fmt"
	"net/url"
	"os"

	"bb-dpi/client/bb-vpn/pkg/state"
)

// enrollCmd handles `bb-vpn enroll <bb-vpn://enroll?uuid=...>`.
// Parses the magic-link URI, extracts the UUID, validates format,
// drops an enroll-*.json into inbox/. Runs as the logged-in user;
// the root sync daemon picks up the request on its next tick and
// writes the canonical identity.json.
func enrollCmd(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "bb-vpn enroll: usage: bb-vpn enroll bb-vpn://enroll?uuid=<uuid>")
		return exitUsage
	}
	raw := args[0]
	u, err := url.Parse(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn enroll: parse URI: %v\n", err)
		return exitUsage
	}
	if u.Scheme != "bb-vpn" || u.Host != "enroll" {
		fmt.Fprintf(os.Stderr, "bb-vpn enroll: URI must match bb-vpn://enroll?uuid=...\n")
		return exitUsage
	}
	uuid := u.Query().Get("uuid")
	if err := state.ValidateUUID(uuid); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn enroll: %v\n", err)
		return exitUsage
	}
	if err := state.WriteEnroll(state.EnrollRequest{UUID: uuid, Source: "cli"}); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn enroll: write inbox: %v\n", err)
		return exitSoftware
	}
	fmt.Fprintf(os.Stderr, "bb-vpn enroll: queued — root daemon will pick up on next sync tick\n")
	return 0
}
