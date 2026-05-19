package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"bb-dpi/client/bb-vpn/pkg/bundle"
	"bb-dpi/client/bb-vpn/pkg/render"
)

// renderCmd implements `bb-vpn render`. Pure, no root, no /Library
// paths. The canonical rendering entry point for dev/debug + the
// internal harness; pkg/launchctl invokes pkg/render directly
// without going through this CLI.
//
// Returns process exit code.
func renderCmd(args []string) int {
	f, err := parseRenderFlags(args)
	if err != nil {
		// -h / --help prints usage and returns flag.ErrHelp; treat as
		// success so `bb-vpn render --help` exits 0, not exitUsage.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		// flag package already printed the error.
		return exitUsage
	}
	if f.bundle == "" || f.uuid == "" || f.outDir == "" {
		fmt.Fprintln(os.Stderr, "bb-vpn render: --bundle, --uuid, --out-dir all required")
		return exitUsage
	}

	data, err := os.ReadFile(f.bundle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn render: read bundle: %v\n", err)
		return exitSoftware
	}
	b, err := bundle.Parse(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn render: %v\n", err)
		return exitSoftware
	}
	if err := b.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn render: %v\n", err)
		return exitSoftware
	}

	// --home defaults to $HOME so common invocations (without explicit
	// --home) produce a usable config — sing-box state_directory paths
	// substitute ${HOME} and would otherwise be empty.
	home := f.home
	if home == "" {
		home = os.Getenv("HOME")
	}
	env := render.Env{
		HOME:              home,
		UUID:              f.uuid,
		TailscaleAuthKey:  f.tailscaleAuthKey,
		TailscaleHostname: f.tailscaleHostname,
		InternalDNS1:      f.internalDNS1,
		CompanyDomain:     f.companyDomain,
		Flow:              f.flow,
		Fingerprint:       f.fingerprint,
	}
	singBox, xray, err := render.Render(b, env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn render: %v\n", err)
		return exitSoftware
	}

	if err := os.MkdirAll(f.outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn render: mkdir %s: %v\n", f.outDir, err)
		return exitSoftware
	}
	if err := writeFile(filepath.Join(f.outDir, "sing-box.json"), singBox); err != nil {
		fmt.Fprintf(os.Stderr, "bb-vpn render: %v\n", err)
		return exitSoftware
	}
	if len(xray) > 0 {
		if err := writeFile(filepath.Join(f.outDir, "xray.json"), xray); err != nil {
			fmt.Fprintf(os.Stderr, "bb-vpn render: %v\n", err)
			return exitSoftware
		}
	}
	return 0
}

// writeFile uses atomic write (tmp + rename) so partial output never
// leaves a half-written file under out-dir. Mode 0600 because rendered
// configs can contain Tailscale auth_key and other operator secrets —
// world-readable on a shared Mac would expose them to other users.
// (macOS-only project; Windows os.Rename-fails-on-existing isn't a
// concern in scope.)
func writeFile(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("fsync %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
