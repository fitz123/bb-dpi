// Package state manages bb-vpn's filesystem state machine under
// /Library/Application Support/bb-dpi/. All paths are root-owned and
// touched only by the bb-vpn sync daemon EXCEPT inbox/, which is
// world-writable (mode 1733 / sticky) so the menu-bar app can hand
// enrollment requests to the root daemon without privilege escalation.
//
// Layout:
//
//	/Library/Application Support/bb-dpi/
//	├── bin/                # bundled sing-box, xray, bb-vpn binaries (root:wheel 0755)
//	├── inbox/              # mode 1733 — drop-box for enroll-*.json
//	├── bundles/            # current.json + previous.json + blackhole-<ts>.json
//	├── staging/            # validate-before-promote tmp configs
//	├── configs/            # rendered sing-box.json + xray.json (read by LaunchDaemons)
//	├── identity.json       # { "uuid": "..." } (root:wheel 0600)
//	├── identity.log        # append-only JSONL audit (root:wheel 0600, 10MB rotate)
//	├── status.json         # { last_sync, last_error, ... } (root:wheel 0644, world-readable)
//	└── control-plane.json  # { endpoints, token } baked into the .pkg (root:wheel 0600)
//
// Single-user-Mac assumption: each of the operator's 7 fleet Macs is a
// personal device. Multi-user Mac scenarios are explicitly out of scope.
package state

import (
	"os"
	"path/filepath"
)

// DefaultRoot is the production location for the state tree on macOS.
// Dev mode overrides via env BB_VPN_STATE_ROOT.
const DefaultRoot = "/Library/Application Support/bb-dpi"

// Root returns the active state directory. Honors $BB_VPN_STATE_ROOT
// for dev/test use (e.g., on macold where the legacy bash flow owns
// /Library/Application Support/bb-dpi/).
func Root() string {
	if v := os.Getenv("BB_VPN_STATE_ROOT"); v != "" {
		return v
	}
	return DefaultRoot
}

// Path joins one or more components onto Root(). Use for any state
// file access so dev/prod paths follow the same code path.
func Path(parts ...string) string {
	all := append([]string{Root()}, parts...)
	return filepath.Join(all...)
}

// Canonical filenames + subdirs — names match the plan's layout.
const (
	BinDir           = "bin"
	InboxDir         = "inbox"
	BundlesDir       = "bundles"
	StagingDir       = "staging"
	ConfigsDir       = "configs"
	IdentityFile     = "identity.json"
	IdentityLogFile  = "identity.log"
	StatusFile       = "status.json"
	ControlPlaneFile = "control-plane.json"
	CurrentBundle    = "bundles/current.json"
	PreviousBundle   = "bundles/previous.json"
	SingBoxConfig    = "configs/sing-box.json"
	XrayConfig       = "configs/xray.json"
	StagingSingBox   = "staging/sing-box.json"
	StagingXray      = "staging/xray.json"
)
