// bb-vpn is the bb-dpi client-side control-plane daemon and CLI.
//
// Subcommands:
//
//	sync     — fetch+render+kickstart loop (root LaunchDaemon)
//	enroll   — queue enrollment from bb-vpn://enroll?uuid=... URI or bare UUID (user-space)
//	status   — print state to stdout (user-space, read-only)
//	recover  — clear runtime_blackhole circuit breaker (sudo)
//	render   — render configs from a bundle for goldens/debug (no root)
package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	exitUsage    = 64 // EX_USAGE — bad cmdline (unknown / missing subcommand)
	exitSoftware = 70 // EX_SOFTWARE — internal error (file IO, parse fail, etc.)
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
	switch os.Args[1] {
	case "sync":
		os.Exit(syncCmd(os.Args[2:]))
	case "enroll":
		os.Exit(enrollCmd(os.Args[2:]))
	case "status":
		os.Exit(statusCmd(os.Args[2:]))
	case "recover":
		os.Exit(recoverCmd(os.Args[2:]))
	case "render":
		os.Exit(renderCmd(os.Args[2:]))
	case "-h", "--help", "help":
		usage(os.Stdout)
	case "-V", "--version", "version":
		fmt.Printf("bb-vpn version %s\n", Version)
	default:
		fmt.Fprintf(os.Stderr, "bb-vpn: unknown subcommand %q\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
}

func usage(w *os.File) {
	fmt.Fprintln(w, `usage: bb-vpn <subcommand> [args]

subcommands:
  sync       fetch bundle, render configs, kickstart services (root)
  enroll     queue enrollment from bb-vpn://enroll?uuid=UUID or bare UUID (user)
  status     print current state (user)
  recover    clear runtime_blackhole circuit breaker (sudo)
  render     render configs from a bundle (no root; for debug/goldens)
  --version  print bb-vpn version string and exit

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
	// Reject leftover positional args so typos like
	// `bb-vpn render --bundle b.json --uuid u --out-dir o extra`
	// don't get silently swallowed.
	if leftover := fs.Args(); len(leftover) > 0 {
		return f, fmt.Errorf("bb-vpn render: unexpected positional args: %v", leftover)
	}
	return f, nil
}
