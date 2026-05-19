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
// PR C: render is implemented. sync/enroll/status/recover remain stubs
// (land in PR D with pkg/state + pkg/cphttp + pkg/launchctl).
package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	exitUsage       = 64 // EX_USAGE — bad cmdline (unknown / missing subcommand)
	exitUnavailable = 69 // EX_UNAVAILABLE — service unavailable (subcommand stubbed out)
	exitSoftware    = 70 // EX_SOFTWARE — internal error (file IO, parse fail, etc.)
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
		os.Exit(renderCmd(os.Args[2:]))
	case "-h", "--help", "help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "bb-vpn: unknown subcommand %q\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
}

func stub(name string) {
	fmt.Fprintf(os.Stderr, "bb-vpn %s: not yet implemented (lands in Phase 2 PR D)\n", name)
	os.Exit(exitUnavailable)
}

func usage(w *os.File) {
	fmt.Fprintln(w, `usage: bb-vpn <subcommand> [args]

subcommands:
  sync     fetch bundle, render configs, kickstart services (root)
  enroll   handle bb-vpn://enroll?uuid=... (user)
  status   print current state (user)
  recover  clear runtime_blackhole circuit breaker (sudo)
  render   render configs from a bundle (no root; for debug/goldens)

render flags:
  bb-vpn render --bundle FILE --uuid UUID --out-dir DIR
                [--home PATH] [--tailscale-auth-key K] [--tailscale-hostname H]
                [--internal-dns-1 IP] [--company-domain DOMAIN]
                [--flow VAL] [--fingerprint VAL]`)
}

// renderFlags holds the parsed `bb-vpn render` flag values. Kept as a
// named struct (rather than parsed inline in renderCmd) so tests can
// drive renderCmd with custom argv slices via parseRenderFlags without
// going through os.Args.
type renderFlags struct {
	bundle            string
	uuid              string
	outDir            string
	home              string
	tailscaleAuthKey  string
	tailscaleHostname string
	internalDNS1      string
	companyDomain     string
	flow              string
	fingerprint       string
}

func parseRenderFlags(args []string) (renderFlags, error) {
	f := renderFlags{}
	fs := flag.NewFlagSet("bb-vpn render", flag.ContinueOnError)
	fs.StringVar(&f.bundle, "bundle", "", "path to bundle.json")
	fs.StringVar(&f.uuid, "uuid", "", "VLESS user UUID")
	fs.StringVar(&f.outDir, "out-dir", "", "output directory for rendered configs")
	fs.StringVar(&f.home, "home", "", "HOME path for $HOME envsubst")
	fs.StringVar(&f.tailscaleAuthKey, "tailscale-auth-key", "", "tsnet auth key")
	fs.StringVar(&f.tailscaleHostname, "tailscale-hostname", "", "tsnet hostname")
	fs.StringVar(&f.internalDNS1, "internal-dns-1", "", "corp DNS resolver IP")
	fs.StringVar(&f.companyDomain, "company-domain", "", "corp DNS suffix")
	fs.StringVar(&f.flow, "flow", "", "VLESS flow (default xtls-rprx-vision)")
	fs.StringVar(&f.fingerprint, "fingerprint", "", "uTLS fingerprint (default chrome)")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	return f, nil
}
