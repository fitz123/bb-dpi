// bb-vpn is the bb-dpi client-side control-plane daemon and CLI.
//
// Subcommands:
//
//	sync     — fetch+render+kickstart loop (root LaunchDaemon)
//	enroll   — handle bb-vpn://enroll?uuid=... URI (user-space)
//	status   — print state to stdout (user-space, read-only)
//	recover  — clear runtime_blackhole circuit breaker (sudo)
//	render   — render configs from a bundle for goldens/debug (no root)
//
// Phase 2 PR A ships scaffolding only — every subcommand returns
// "not yet implemented" until the relevant pkg/* lands in PR C-E.
package main

import (
	"fmt"
	"os"
)

const (
	exitUsage       = 64 // EX_USAGE — bad cmdline (unknown / missing subcommand)
	exitUnavailable = 69 // EX_UNAVAILABLE — service unavailable (subcommand stubbed out)
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
	switch os.Args[1] {
	case "sync":
		stub("sync")
	case "enroll":
		stub("enroll")
	case "status":
		stub("status")
	case "recover":
		stub("recover")
	case "render":
		stub("render")
	case "-h", "--help", "help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "bb-vpn: unknown subcommand %q\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
}

func stub(name string) {
	fmt.Fprintf(os.Stderr, "bb-vpn %s: not yet implemented (Phase 2 PR A scaffold)\n", name)
	os.Exit(exitUnavailable)
}

func usage(w *os.File) {
	fmt.Fprintln(w, `usage: bb-vpn <subcommand> [args]

subcommands:
  sync     fetch bundle, render configs, kickstart services (root)
  enroll   handle bb-vpn://enroll?uuid=... (user)
  status   print current state (user)
  recover  clear runtime_blackhole circuit breaker (sudo)
  render   render configs from a bundle (no root; for debug/goldens)`)
}
