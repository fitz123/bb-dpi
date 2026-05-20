package main

// Version is the bb-vpn release tag baked into the binary by the
// release Makefile via `-ldflags=-X main.Version=<ver>`. Defaults to
// the dev sentinel so a `go build` without ldflags is obvious.
//
// The Phase 4 .pkg build script invokes `bb-vpn --version`, parses
// the numeric suffix, and asserts equality with
// config/control-plane/package-manifest.json's `bb_vpn` key — so the
// shipped binary, the manifest, and the bundle.min_versions row are
// always coupled.
var Version = "dev"
