package launchctl

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"bb-dpi/client/bb-vpn/pkg/bundle"
	"bb-dpi/client/bb-vpn/pkg/cphttp"
	"bb-dpi/client/bb-vpn/pkg/render"
	"bb-dpi/client/bb-vpn/pkg/state"
)

// SyncOptions configure one tick of the sync loop. The production
// LaunchDaemon constructs these from env + state defaults; tests
// inject custom values to drive specific failure paths.
type SyncOptions struct {
	// BinDir is the directory holding sing-box + xray + bb-vpn.
	// Production: state.Path(state.BinDir). Dev override via
	// $BB_VPN_BIN_DIR (e.g. /opt/homebrew/bin on macold).
	BinDir string
	// DevMode disables kickstart calls so a dev/test run on a
	// macold-style host doesn't disrupt the legacy bash flow.
	DevMode bool
	// SkipPrint disables the post-restart smoke test (Print +
	// circuit breaker). Tests set this to keep the harness
	// deterministic — the smoke test exec's launchctl which
	// isn't sane in unit tests.
	SkipPrint bool
}

// Result describes the outcome of one Tick. Errors at any step are
// surfaced as Result.Err; status.json is updated correspondingly so
// `bb-vpn status` shows the most recent failure mode.
type Result struct {
	BundleIssuedAt   string
	ServerCount      int
	XrayNeeded       bool
	Rendered         bool
	Promoted         bool
	Kickstarted      bool
	BlackholeEntered bool
	// LiveMatchesBundle is true iff the live sing-box/xray configs on
	// disk are guaranteed to have been rendered from the bundle whose
	// IssuedAt is BundleIssuedAt. Two paths set it:
	//   - meaningful-change short-circuit (live == staging == bundle render)
	//   - successful PromoteBundle + live promote in Step 6
	// finalize() only stamps status.CurrentIssuedAt when this is true —
	// otherwise a mid-pipeline failure (validate_*, promote_*, kickstart_*)
	// would make `bb-vpn status` lie about which bundle is actually
	// running.
	LiveMatchesBundle bool
	Err               error
	// InboxDrainErr captures a non-fatal failure to read inbox/ (perms
	// accident on the dir, transient FS error, etc.). The daemon
	// continues the tick with the existing identity rather than wedging
	// — but finalize surfaces this on status.LastError when no other
	// error supersedes so the operator sees the problem.
	InboxDrainErr error
}

// Tick runs one iteration of the sync algorithm:
//
//  0. Drain inbox/.
//  1. Fetch bundle (or read current.json if cphttp fails).
//  2. Validate bundle compatibility (min_versions).
//  3. Render to staging/.
//  4. Pre-restart validation (sing-box check, xray -test).
//  5. Meaningful-change diff vs configs/.
//  6. Atomic promote staging → configs.
//  7. Bootstrap-or-kickstart sing-box + xray (unless DevMode).
//  8. Post-restart smoke test + circuit breaker.
//  9. Write status.json.
//
// Returns a non-nil Result.Err on any failure that prevents promotion;
// the daemon stays running and retries on the next tick.
func Tick(opts SyncOptions) Result {
	res := Result{}
	identityChanged := false

	// Step 0: drain inbox/ — process any pending enrollment requests.
	// A drain error (unreadable inbox dir, perms accident, etc.) is
	// non-fatal: existing identity is still on disk, so the rest of the
	// tick continues. The error is captured and surfaced via finalize
	// when no other failure supersedes — operators must see this on
	// `bb-vpn status` or new enrollment requests would vanish silently.
	winner, _, drainErr := state.DrainInbox()
	if drainErr != nil {
		res.InboxDrainErr = drainErr
	} else if winner != nil {
		if werr := state.WriteIdentity(state.Identity{UUID: winner.UUID}); werr != nil {
			res.Err = fmt.Errorf("sync: write identity: %w", werr)
			return finalize(res, "identity_write_failed", false, false)
		}
		identityChanged = true
	}

	id, err := state.ReadIdentity()
	if err != nil {
		res.Err = fmt.Errorf("sync: no identity yet — waiting for enrollment (%w)", err)
		return finalize(res, "no_identity", identityChanged, false)
	}

	// Step 1: load control-plane config + fetch bundle.
	cpCfg, err := cphttp.LoadConfig(state.Path(state.ControlPlaneFile))
	if err != nil {
		res.Err = fmt.Errorf("sync: load control-plane.json: %w", err)
		return finalize(res, "control_plane_unreadable", identityChanged, false)
	}
	usedCachedBundle := false
	fetchSucceeded := false
	bundleBytes, _, err := cphttp.Fetch(cpCfg)
	if err != nil {
		// Fall back to last-known-good bundle if cphttp is unreachable.
		// LastFetchError surfaces the upstream outage to operators even
		// though LastError stays clean on a successful tick.
		if cached, cerr := state.ReadBundle(); cerr == nil {
			bundleBytes = cached
			usedCachedBundle = true
		} else {
			res.Err = fmt.Errorf("sync: fetch bundle: %w", err)
			return finalize(res, "fetch_failed", identityChanged, false)
		}
	} else {
		fetchSucceeded = true
	}

	b, err := bundle.Parse(bundleBytes)
	if err != nil {
		res.Err = fmt.Errorf("sync: parse bundle: %w", err)
		return finalize(res, "parse_failed", identityChanged, fetchSucceeded)
	}
	if err := b.Validate(); err != nil {
		res.Err = fmt.Errorf("sync: validate bundle: %w", err)
		return finalize(res, "validate_failed", identityChanged, fetchSucceeded)
	}
	res.BundleIssuedAt = b.IssuedAt
	res.ServerCount = len(b.Servers)

	// Step 2: compatibility check (min_versions vs local binaries).
	if err := checkBinaryVersions(opts.BinDir, b); err != nil {
		res.Err = fmt.Errorf("sync: incompatible bundle: %w", err)
		return finalize(res, "incompatible_versions", identityChanged, fetchSucceeded)
	}

	// Step 3: render to staging/.
	env, err := buildSyncEnv(b, id.UUID)
	if err != nil {
		res.Err = fmt.Errorf("sync: build render env: %w", err)
		return finalize(res, "render_env_invalid", identityChanged, fetchSucceeded)
	}
	singBox, xray, err := render.Render(b, env)
	if err != nil {
		res.Err = fmt.Errorf("sync: render: %w", err)
		return finalize(res, "render_failed", identityChanged, fetchSucceeded)
	}
	res.Rendered = true
	res.XrayNeeded = len(xray) > 0

	stagingSB := state.Path(state.StagingSingBox)
	stagingXR := state.Path(state.StagingXray)
	// 0o600: rendered configs may contain a Tailscale auth_key and the
	// VLESS UUID. LaunchDaemons run as root so 0600 is readable; matches
	// the PR C decision to lock rendered configs to root-only.
	if err := state.WriteAtomic(stagingSB, singBox, 0o600); err != nil {
		res.Err = fmt.Errorf("sync: write staging sing-box: %w", err)
		return finalize(res, "staging_write_failed", identityChanged, fetchSucceeded)
	}
	if res.XrayNeeded {
		if err := state.WriteAtomic(stagingXR, xray, 0o600); err != nil {
			res.Err = fmt.Errorf("sync: write staging xray: %w", err)
			return finalize(res, "staging_write_failed", identityChanged, fetchSucceeded)
		}
	} else {
		_ = os.Remove(stagingXR)
	}

	// Step 4: pre-restart validation — MUST gate before promote.
	if err := validateSingBox(opts.BinDir, stagingSB); err != nil {
		res.Err = fmt.Errorf("sync: sing-box check failed: %w", err)
		return finalize(res, "validate_singbox_failed", identityChanged, fetchSucceeded)
	}
	if res.XrayNeeded {
		if err := validateXray(opts.BinDir, stagingXR); err != nil {
			res.Err = fmt.Errorf("sync: xray -test failed: %w", err)
			return finalize(res, "validate_xray_failed", identityChanged, fetchSucceeded)
		}
	}

	// Step 5: meaningful-change check.
	liveSB := state.Path(state.SingBoxConfig)
	liveXR := state.Path(state.XrayConfig)
	if !changed(stagingSB, liveSB) && (!res.XrayNeeded || !changed(stagingXR, liveXR)) {
		// No-op tick — bundle is identical to running config. We still
		// call state.PromoteBundle: its inner bytes-equal short-circuit
		// detects no real change AND heals current.json's mode to 0o644
		// in the same path. Without this call, an upgrade-installed
		// client whose bundle bytes AND render bytes both byte-match
		// the previously-running state would never trigger the chmod
		// heal — leaving current.json at 0o600 from the pre-PR binary
		// and locking the menubar out indefinitely.
		//
		// The short-circuit also keeps previous.json safe: it does NOT
		// rotate on byte-equal bundles, so this preserves the
		// last-known-good rollback anchor across no-change ticks (and
		// across re-fetches of a known-broken bundle that the sync loop
		// rolled back from).
		if err := state.PromoteBundle(bundleBytes); err != nil {
			res.Err = fmt.Errorf("sync: promote bundle (no-op path): %w", err)
			return finalize(res, "promote_bundle_failed", identityChanged, fetchSucceeded)
		}
		// Live configs == staging render means the running configs
		// were produced from this bundle (or one with byte-identical
		// render output), so it's safe to stamp CurrentIssuedAt.
		res.LiveMatchesBundle = true
		if state.ManuallyStopped() {
			// Self-heal a stop-vs-tick race: if stopCmd ran SetManuallyStopped()
			// + Bootout() between this tick's earlier observation and now, the
			// daemons are already down (Bootout is idempotent). If the race went
			// the other way (tick re-kickstarted between stopCmd's SetManually-
			// Stopped() and its Bootout calls), THIS tick now sees the flag and
			// tears them back down. Either way, post-condition matches the flag.
			_ = Bootout(SingBox)
			_ = Bootout(Xray)
			return finalize(res, "manually_stopped", identityChanged, fetchSucceeded)
		}
		// Reboot-recovery: sing-box/xray plists have RunAtLoad=false (bb-vpn-sync
		// drives their lifecycle). After a reboot, launchd reloads the plists but
		// doesn't start them, and the staging-vs-live diff is empty (configs/
		// survived the reboot) so we land here. Without an explicit kickstart the
		// daemons would stay down until the next config change or manual `bb-vpn
		// start`. The operator's intent is captured by the ManuallyStopped flag —
		// if it's false (handled just above), daemons should be up. This is NOT
		// the rejected "auto-revival on external bootout" behavior: the trigger
		// is the same (daemon down on a no-op tick) but the "stop daemon externally
		// to debug" path is not supported (operator must use `sudo bb-vpn stop`).
		if !opts.SkipPrint && !opts.DevMode {
			// xray before sing-box (see Step 7): sing-box's urltest probes
			// xray's SOCKS on start, so xray must be serving them first.
			if res.XrayNeeded {
				if running, _ := Print(Xray); !running {
					if err := KickstartService(Xray); err != nil {
						res.Err = err
						return finalize(res, "kickstart_xray_failed", identityChanged, fetchSucceeded)
					}
					res.Kickstarted = true
				}
			}
			if running, _ := Print(SingBox); !running {
				if err := KickstartService(SingBox); err != nil {
					res.Err = err
					return finalize(res, "kickstart_singbox_failed", identityChanged, fetchSucceeded)
				}
				res.Kickstarted = true
			}
			// Reboot-recovery smoke test. If we kickstarted at least one
			// daemon back up after a reboot, verify sing-box actually
			// stayed running. Without this guard, a daemon that exits
			// shortly after kickstart (transient launchd glitch, a
			// previously-good but now-unhappy cached config edge case)
			// would be finalized clean and the menubar would briefly
			// show green while the tunnel is in fact down.
			//
			// Unlike the step-8 (bundle-change) smoke test, we do NOT
			// trigger rollback or runtime_blackhole here: the bundle is
			// byte-identical to what was running pre-reboot, so the
			// failure is a daemon/launchd runtime issue, not a bundle
			// regression. Surface a soft kickstart_singbox_failed and
			// let the next tick retry; an operator can rerun
			// `sudo bb-vpn start` or wait for the next 15-min tick.
			if res.Kickstarted {
				time.Sleep(5 * time.Second)
				if running, _ := Print(SingBox); !running {
					res.Err = errors.New("sync: reboot-recovery kickstart failed smoke test")
					return finalize(res, "kickstart_singbox_failed", identityChanged, fetchSucceeded)
				}
			}
		}
		return finalize(res, fetchErrKey(usedCachedBundle), identityChanged, fetchSucceeded)
	}

	// Step 6: atomic promote.
	//
	// Order matters: PromoteBundle FIRST, then live configs. PromoteBundle
	// is a single atomic write on current.json — it either fully succeeds
	// or leaves current.json untouched. If it fails, live configs are
	// unchanged so the next tick sees changed()==true again and retries
	// cleanly. The previous order (live first, then bundle) created a
	// stuck state on PromoteBundle failure: live == staging on next tick
	// → meaningful-change short-circuit → no kickstart → services keep
	// running the OLD bundle's last live render while current.json still
	// points at the older bundle.
	if err := state.PromoteBundle(bundleBytes); err != nil {
		res.Err = fmt.Errorf("sync: promote bundle: %w", err)
		return finalize(res, "promote_bundle_failed", identityChanged, fetchSucceeded)
	}
	if err := promote(stagingSB, liveSB); err != nil {
		res.Err = err
		return finalize(res, "promote_failed", identityChanged, fetchSucceeded)
	}
	if res.XrayNeeded {
		if err := promote(stagingXR, liveXR); err != nil {
			res.Err = err
			return finalize(res, "promote_failed", identityChanged, fetchSucceeded)
		}
	} else {
		_ = os.Remove(liveXR)
	}
	res.Promoted = true
	res.LiveMatchesBundle = true

	// Step 7: kickstart services (unless DevMode or user-stopped).
	if opts.DevMode {
		return finalize(res, fetchErrKey(usedCachedBundle), identityChanged, fetchSucceeded)
	}
	// Respect the menubar "Stop" flag — bundle update lands but
	// daemons stay down until the user clicks Start.
	//
	// Bootout() defensively ensures the post-condition (daemons down)
	// even if a stop racing this tick lost the SetManuallyStopped()/
	// Bootout() ordering window. Idempotent — already-bootouted services
	// exit cleanly.
	if state.ManuallyStopped() {
		_ = Bootout(SingBox)
		_ = Bootout(Xray)
		return finalize(res, "manually_stopped", identityChanged, fetchSucceeded)
	}
	// Order matters: xray FIRST, then sing-box. xray serves the SOCKS
	// proxies (127.0.0.1:1080+i) that sing-box's xhttp-* outbounds point
	// at, and sing-box's urltest probes them immediately on start. Starting
	// sing-box first makes its first probe hit a not-yet-restarted xray, so
	// a newly-added relay outbound is marked dead on first impression — and
	// with the urltest interrupt_exist_connections tuning a client can latch
	// onto that bad impression instead of recovering. (Mirrors deploy.sh's
	// pull-before-validate ordering rule on the server side.)
	if res.XrayNeeded {
		if err := KickstartService(Xray); err != nil {
			res.Err = err
			return finalize(res, "kickstart_xray_failed", identityChanged, fetchSucceeded)
		}
	} else {
		_ = Bootout(Xray)
	}
	if err := KickstartService(SingBox); err != nil {
		res.Err = err
		return finalize(res, "kickstart_singbox_failed", identityChanged, fetchSucceeded)
	}
	res.Kickstarted = true

	// Step 8: post-restart smoke test (skip in tests).
	if opts.SkipPrint {
		return finalize(res, fetchErrKey(usedCachedBundle), identityChanged, fetchSucceeded)
	}
	time.Sleep(5 * time.Second)
	running, _ := Print(SingBox)
	if !running {
		// Try rolling back to previous.json.
		if prev, perr := state.ReadPreviousBundle(); perr == nil {
			if rollback(prev, opts) == nil {
				// Recovered — archive the broken bundle so the next
				// sync re-fetches from the control plane instead of
				// replaying the same failure forever.
				_ = state.ArchiveBundleBlackhole()
				// Live configs now reflect the PREVIOUS bundle, not
				// res.BundleIssuedAt — clear the LiveMatchesBundle
				// flag so finalize doesn't stamp CurrentIssuedAt with
				// the broken-bundle IssuedAt.
				res.LiveMatchesBundle = false
				return finalize(res, "rolled_back", identityChanged, fetchSucceeded)
			}
		}
		// Both new and rollback failed → circuit breaker.
		res.BlackholeEntered = true
		_ = Disable(SingBox)
		_ = Disable(Xray)
		_ = state.ArchiveBundleBlackhole()
		// Bundle is archived as broken — don't advance CurrentIssuedAt
		// to claim the daemon is running this bundle.
		res.LiveMatchesBundle = false
		res.Err = errors.New("sync: runtime_blackhole entered")
		return finalize(res, "runtime_blackhole", identityChanged, fetchSucceeded)
	}
	return finalize(res, fetchErrKey(usedCachedBundle), identityChanged, fetchSucceeded)
}

// fetchErrKey returns the LastError key to surface a degraded-fetch
// outcome on an otherwise-successful tick. Empty string means "no
// error" (clear LastError); the dedicated LastFetchError field carries
// the more granular cphttp signal.
func fetchErrKey(usedCachedBundle bool) string {
	if usedCachedBundle {
		return "fetch_failed_using_cached"
	}
	return ""
}

// buildSyncEnv assembles the render.Env from the LaunchDaemon's
// EnvironmentVariables. Root LaunchDaemons get a sanitized env, so
// HOME=/var/root by default — every variable that the render path can
// touch must be explicitly forwarded by the plist (or BB_VPN_HOME for
// HOME).
//
// Hard-fails when neither HOME nor BB_VPN_HOME is set, since
// envsubst-into-rendered-configs paths would silently bake "/var/root"
// (or empty) into ${HOME}-based file paths under TUN routing, leaving
// the menu-bar app + log directories pointing at a nonexistent user
// tree. The operator-facing fix is to declare BB_VPN_HOME in the
// LaunchDaemon plist's EnvironmentVariables.
//
// Corp-DNS values (InternalDNS1 + CompanyDomain) are sourced bundle-first:
// b.Render.InternalDNS1 / b.Render.CompanyDomain take precedence, with
// the legacy BB_VPN_INTERNAL_DNS_1 / BB_VPN_COMPANY_DOMAIN env-vars used
// only when the bundle field is empty. This lets a single
// `make publish-bundle` propagate corp-DNS to the fleet without per-Mac
// plist edits, while still supporting clients that haven't been
// re-bundled yet.
//
// buildSyncEnv itself does NOT check whether WithCorpDNS demands these
// values be set — that check belongs to render.Render(), which is the
// single source of truth (load-bearing: a duplicate check here would
// short-circuit the env-var fallback and break legacy bundles).
func buildSyncEnv(b *bundle.Bundle, uuid string) (render.Env, error) {
	home := os.Getenv("BB_VPN_HOME")
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" || home == "/var/root" {
		return render.Env{}, fmt.Errorf("HOME not set (BB_VPN_HOME or HOME env required — declare in LaunchDaemon EnvironmentVariables)")
	}
	internalDNS1 := ""
	companyDomain := ""
	if b != nil {
		internalDNS1 = b.Render.InternalDNS1
		companyDomain = b.Render.CompanyDomain
	}
	if internalDNS1 == "" {
		internalDNS1 = os.Getenv("BB_VPN_INTERNAL_DNS_1")
	}
	if companyDomain == "" {
		companyDomain = os.Getenv("BB_VPN_COMPANY_DOMAIN")
	}
	return render.Env{
		HOME:              home,
		UUID:              uuid,
		Flow:              "xtls-rprx-vision",
		Fingerprint:       "chrome",
		TailscaleAuthKey:  os.Getenv("BB_VPN_TAILSCALE_AUTH_KEY"),
		TailscaleHostname: os.Getenv("BB_VPN_TAILSCALE_HOSTNAME"),
		InternalDNS1:      internalDNS1,
		CompanyDomain:     companyDomain,
	}, nil
}

func finalize(res Result, errKey string, identityChanged bool, fetchSucceeded bool) Result {
	s, _ := state.ReadStatus()
	now := time.Now().UTC().Format(time.RFC3339)
	s.LastSync = now
	// A drain-inbox failure is non-fatal but must surface on
	// `bb-vpn status` so the operator notices that new enrollment
	// requests aren't being processed. It only wins when no other
	// errKey is set (any pipeline failure takes priority).
	if res.InboxDrainErr != nil && errKey == "" {
		errKey = "inbox_drain_failed"
	}
	// "fetch_failed_using_cached" is a degraded-but-successful tick:
	// the control plane was unreachable but we promoted a cached
	// bundle. Operators reading `bb-vpn status` should see this as
	// healthy (LastError clean) with the degradation visible only on
	// LastFetchError. A hard fetch failure ("fetch_failed") with no
	// cache available is fatal and still surfaces on LastError.
	//
	// "manually_stopped" is NOT cleared here — it must propagate to
	// status.json so the menubar can distinguish an intentional `sudo
	// bb-vpn stop` (grey "stopped" state) from a crashed daemon
	// (yellow "degraded" state). The Swift StatusModel branches on
	// this exact sentinel value to pick the correct color/header.
	if errKey != "" && errKey != "fetch_failed_using_cached" {
		s.LastError = errKey
	} else {
		s.LastError = ""
	}
	// LastFetchError tracks the fetch path independently of LastError.
	// Priority: a fresh successful fetch wins (clears the signal) even
	// if a later step in the pipeline failed — the control plane is
	// reachable, that's what this field tracks. Otherwise mirror the
	// fetch-related errKeys, or clear on a clean tick.
	switch {
	case fetchSucceeded:
		s.LastFetchError = ""
	case errKey == "fetch_failed_using_cached" || errKey == "fetch_failed":
		s.LastFetchError = errKey
	case res.Err == nil || errKey == "rolled_back":
		// Successful or recovered tick implies the fetch path was
		// either fine or didn't run — clear the lingering signal.
		s.LastFetchError = ""
	}
	// Only stamp CurrentIssuedAt when the live configs on disk are
	// actually rendered from this bundle. Without the LiveMatchesBundle
	// gate, every post-parse failure (validate_singbox_failed,
	// promote_failed, kickstart_*_failed, runtime_blackhole, …) would
	// quietly advance CurrentIssuedAt to a bundle that never made it to
	// the live configs — making `bb-vpn status` lie about what the
	// services are actually running.
	if res.BundleIssuedAt != "" && res.LiveMatchesBundle {
		s.CurrentIssuedAt = res.BundleIssuedAt
	}
	if res.ServerCount > 0 && res.LiveMatchesBundle {
		s.CurrentServerCount = res.ServerCount
	}
	if identityChanged {
		s.LastIdentityChange = now
	}
	// Snapshot daemon liveness for the menubar. The menubar runs as the
	// console user and can't `launchctl print system/<label>` — only the
	// root daemon (us) can. Stale up to next tick (~15min), refreshed
	// every Tick.
	s.SingBoxRunning, _ = Print(SingBox)
	s.XrayRunning, _ = Print(Xray)
	// XrayNeeded only flips when Render succeeded this tick — only then
	// does res.XrayNeeded reflect a real bundle render. On pre-render
	// failure paths (no_identity, fetch_failed without cache, parse_failed,
	// etc.) we leave the previous value intact so the menubar doesn't
	// flicker into "tcp-vision-only" when the daemon couldn't even decide
	// which proto to render.
	if res.Rendered {
		s.XrayNeeded = res.XrayNeeded
	}
	_ = state.WriteStatus(s)
	return res
}

func checkBinaryVersions(binDir string, b *bundle.Bundle) error {
	sb, err := readBinaryVersion(filepath.Join(binDir, "sing-box"), "version")
	if err != nil {
		return fmt.Errorf("sing-box version: %w", err)
	}
	if ok, err := bundle.SemverGE(sb, b.MinVersions.SingBox); err != nil {
		return fmt.Errorf("sing-box version parse: %w", err)
	} else if !ok {
		return fmt.Errorf("sing-box %s < min %s", sb, b.MinVersions.SingBox)
	}
	xr, err := readBinaryVersion(filepath.Join(binDir, "xray"), "version")
	if err != nil {
		return fmt.Errorf("xray version: %w", err)
	}
	if ok, err := bundle.CalverGE(xr, b.MinVersions.Xray); err != nil {
		return fmt.Errorf("xray version parse: %w", err)
	} else if !ok {
		return fmt.Errorf("xray %s < min %s", xr, b.MinVersions.Xray)
	}
	return nil
}

// versionPattern matches a MAJOR.MINOR.PATCH triple preceded by either
// start-of-string, a non-letter+non-digit, or the literal "v" prefix.
// The negative-lookbehind isn't supported by RE2, so we use a (?:^|...)
// alternation with a capture group for the version itself.
//
// Why not just `\d+\.\d+\.\d+`: "go1.26.1" embedded in the xray banner
// ("... Custom (go1.26.1 darwin/arm64)") would match and outrank the
// actual "Xray 26.3.27" triple on lines that contain both. The
// "boundary OR literal 'v'" rule rejects the "go" prefix while accepting
// the conventional "v1.13.0" form.
var versionPattern = regexp.MustCompile(`(?:^|[^A-Za-z0-9])v?(\d+\.\d+\.\d+)`)

// readBinaryVersion runs `<bin> version` and returns the first
// MAJOR.MINOR.PATCH string found in the output. Real output shapes:
//
//	sing-box: "sing-box version 1.13.11\n(go1.22.0 darwin/arm64)"
//	xray:     "Xray 26.3.27 (Xray, Penetrates Everything.) Custom (go1.26.1 ...)"
//
// Both bury the actual version in a banner line, so the raw first-line
// value would fail bundle.{SemverGE,CalverGE}'s parseTuple on the
// non-numeric prefix. extractVersion pulls just the version triple.
func readBinaryVersion(bin, sub string) (string, error) {
	out, err := exec.Command(bin, sub).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", bin, sub, err)
	}
	v := extractVersion(string(out))
	if v == "" {
		return "", fmt.Errorf("no version in %q", bytes.TrimSpace(out))
	}
	return v, nil
}

// extractVersion returns the first MAJOR.MINOR.PATCH triple found in
// s, scanning line-by-line. Strips a leading "v" (so "v1.13.0" →
// "1.13.0"), and refuses to match a triple that's part of a longer
// identifier ("go1.26.1") so xray's banner doesn't surface the build
// tag's Go version instead of xray's own.
//
// Returns "" if no triple is present (caller hard-fails).
func extractVersion(s string) string {
	if s == "" {
		return ""
	}
	for _, line := range strings.Split(s, "\n") {
		m := versionPattern.FindStringSubmatch(line)
		if len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

func validateSingBox(binDir, configPath string) error {
	bin := filepath.Join(binDir, "sing-box")
	out, err := exec.Command(bin, "check", "-c", configPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s check -c: %w (out: %s)", bin, err, bytes.TrimSpace(out))
	}
	return nil
}

func validateXray(binDir, configPath string) error {
	bin := filepath.Join(binDir, "xray")
	out, err := exec.Command(bin, "-test", "-config", configPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s -test: %w (out: %s)", bin, err, bytes.TrimSpace(out))
	}
	return nil
}

func changed(stagingPath, livePath string) bool {
	st, sErr := os.ReadFile(stagingPath)
	lv, lErr := os.ReadFile(livePath)
	if sErr != nil || lErr != nil {
		return true // can't compare → safe default = re-promote
	}
	return !bytes.Equal(st, lv)
}

func promote(staging, live string) error {
	data, err := os.ReadFile(staging)
	if err != nil {
		return fmt.Errorf("sync: read staging %s: %w", staging, err)
	}
	// 0o600 matches PR C's writeFile decision: rendered configs may
	// embed Tailscale auth_key + the VLESS UUID. LaunchDaemons run
	// as root and read 0600 fine.
	if err := state.WriteAtomic(live, data, 0o600); err != nil {
		return fmt.Errorf("sync: promote %s: %w", staging, err)
	}
	return nil
}

func KickstartService(s Service) error {
	running, err := Print(s)
	if errors.Is(err, ErrNotBootstrapped) {
		plist := "/Library/LaunchDaemons/" + string(s) + ".plist"
		if berr := Bootstrap(plist); berr != nil {
			return fmt.Errorf("sync: bootstrap %s: %w", s, berr)
		}
		return Kickstart(s)
	}
	if err != nil {
		return fmt.Errorf("sync: print %s: %w", s, err)
	}
	_ = running // we always kickstart -k regardless
	return Kickstart(s)
}

// EnsureRunning brings a service up if it's not already running.
// Idempotent — if the service is already running, returns nil
// immediately without calling kickstart -k (which would tear down
// and restart, taking 10+ seconds for sing-box due to TUN teardown).
//
// Used by `bb-vpn start` (terminal + postinstall) so re-running it on
// a healthy install is fast. Use KickstartService when a config
// change requires a forced restart (sync.Tick step 7).
func EnsureRunning(s Service) error {
	running, err := Print(s)
	if errors.Is(err, ErrNotBootstrapped) {
		plist := "/Library/LaunchDaemons/" + string(s) + ".plist"
		if berr := Bootstrap(plist); berr != nil {
			return fmt.Errorf("bootstrap %s: %w", s, berr)
		}
		return Kickstart(s)
	}
	if err != nil {
		return fmt.Errorf("print %s: %w", s, err)
	}
	if running {
		return nil
	}
	return Kickstart(s)
}

func rollback(prevBundle []byte, opts SyncOptions) error {
	b, err := bundle.Parse(prevBundle)
	if err != nil {
		return err
	}
	if err := b.Validate(); err != nil {
		return err
	}
	id, err := state.ReadIdentity()
	if err != nil {
		return err
	}
	env, err := buildSyncEnv(b, id.UUID)
	if err != nil {
		return fmt.Errorf("rollback: build env: %w", err)
	}
	sb, xr, err := render.Render(b, env)
	if err != nil {
		return err
	}
	if err := state.WriteAtomic(state.Path(state.SingBoxConfig), sb, 0o600); err != nil {
		return err
	}
	if len(xr) > 0 {
		if err := state.WriteAtomic(state.Path(state.XrayConfig), xr, 0o600); err != nil {
			return err
		}
	} else {
		// Rollback bundle is proto=tcp-vision (no xray inbounds in
		// the bundle's render output). Mirror the Tick() main success
		// path: drop the stale live xray config and bootout the xray
		// LaunchDaemon. Without this, sing-box runs the rolled-back
		// config while xray keeps running the FAILED bundle's config
		// — mixed state where the auto outbound still tries the
		// XHTTP chain that triggered the rollback.
		_ = os.Remove(state.Path(state.XrayConfig))
	}
	if !opts.DevMode {
		// xray before sing-box (see Step 7) so urltest probes a live xray.
		if len(xr) > 0 {
			_ = Kickstart(Xray)
		} else {
			_ = Bootout(Xray)
		}
		_ = Kickstart(SingBox)
	}
	time.Sleep(5 * time.Second)
	running, _ := Print(SingBox)
	if !running {
		return errors.New("rollback smoke test failed")
	}
	return nil
}
