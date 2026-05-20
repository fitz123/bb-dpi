package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"bb-dpi/client/bb-vpn/pkg/state"
)

// enrollCmd handles `bb-vpn enroll <bb-vpn://enroll?uuid=...|<uuid>>`.
// Parses the magic-link URI (or accepts a bare UUID for terminal
// ergonomics — `?` in the URI is a shell metacharacter that wants
// quoting), extracts the UUID, validates format, drops an
// enroll-*.json into inbox/. Runs as the logged-in user; the root
// sync daemon picks up the request on its next tick and writes the
// canonical identity.json.
func enrollCmd(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "bb-vpn enroll: usage: bb-vpn enroll <bb-vpn://enroll?uuid=UUID | UUID>")
		return exitUsage
	}
	uuid, err := parseEnrollArg(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn enroll: %v\n", err)
		return exitUsage
	}
	if err := state.WriteEnroll(state.EnrollRequest{UUID: uuid, Source: "cli"}); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn enroll: write inbox: %v\n", err)
		return exitSoftware
	}
	fmt.Println("bb-vpn enroll: queued — root daemon will pick up on next sync tick")
	return 0
}

// parseEnrollArg accepts either a full `bb-vpn://enroll?uuid=...` URI
// or a bare UUID literal and returns the validated UUID. Detection is
// by `bb-vpn://` prefix — anything else is treated as bare-UUID and
// run through state.ValidateUUID for shape-checking.
func parseEnrollArg(arg string) (string, error) {
	if strings.HasPrefix(arg, "bb-vpn://") {
		u, err := url.Parse(arg)
		if err != nil {
			return "", fmt.Errorf("parse URI: %w", err)
		}
		if u.Host != "enroll" {
			return "", fmt.Errorf("URI must match bb-vpn://enroll?uuid=...")
		}
		uuid := u.Query().Get("uuid")
		if err := state.ValidateUUID(uuid); err != nil {
			return "", err
		}
		return uuid, nil
	}
	if err := state.ValidateUUID(arg); err != nil {
		return "", err
	}
	return arg, nil
}
