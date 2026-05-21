package launchctl

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"bb-dpi/client/bb-vpn/pkg/state"
)

// TestExtractVersion locks in the regex behaviour against real
// `sing-box version` and `xray version` output. Without this, returning
// the raw first line (the original implementation) fed
// "sing-box version 1.13.11" into bundle.SemverGE's parseTuple, which
// blew up on the first non-numeric component and made every sync tick
// fail with incompatible_versions.
func TestExtractVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"sing-box version 1.13.11\n(go1.22.0 darwin/arm64)", "1.13.11"},
		{"Xray 26.3.27 (Xray, Penetrates Everything.) Custom (go1.26.1 darwin/arm64)", "26.3.27"},
		{"1.13.0", "1.13.0"},
		{"v1.13.0", "1.13.0"},
		{"", ""},
		{"no version here", ""},
	}
	for _, c := range cases {
		got := extractVersion(c.in)
		if got != c.want {
			t.Errorf("extractVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// withStateRoot points pkg/state at a tempdir for the duration of t and
// pre-creates the subdirs the .pkg postinstall would normally provision.
func withStateRoot(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("BB_VPN_STATE_ROOT", dir)
	for _, sub := range []string{state.BundlesDir, state.StagingDir, state.ConfigsDir} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
}

// TestFinalize_CachedFallbackKeepsLastErrorClean locks in the contract
// that a degraded-but-successful tick (cphttp fetch failed, cached
// bundle promoted instead) leaves LastError empty so `bb-vpn status`
// renders the daemon as healthy. Only LastFetchError carries the
// degradation signal. Regression guard for review iter 3.
func TestFinalize_CachedFallbackKeepsLastErrorClean(t *testing.T) {
	withStateRoot(t)
	res := Result{}
	// fetchSucceeded=false: the cached-fallback path runs precisely
	// when the fresh fetch failed, so fetchSucceeded is always false
	// for errKey="fetch_failed_using_cached".
	finalize(res, "fetch_failed_using_cached", false, false)
	s, err := state.ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if s.LastError != "" {
		t.Errorf("LastError = %q, want empty (cached fallback is a healthy tick)", s.LastError)
	}
	if s.LastFetchError != "fetch_failed_using_cached" {
		t.Errorf("LastFetchError = %q, want fetch_failed_using_cached", s.LastFetchError)
	}
}

// TestFinalize_ManuallyStoppedPropagates locks in the contract that
// `sudo bb-vpn stop` surfaces on status.LastError as the sentinel
// "manually_stopped" so the menubar can distinguish a deliberate
// shutdown (grey "stopped") from a crashed daemon (yellow "degraded").
// Clearing LastError here — as the original phase-1 fix did — collapses
// both into the same yellow degraded state in the UI. Regression guard
// for review iter 1 of phase 3.
func TestFinalize_ManuallyStoppedPropagates(t *testing.T) {
	withStateRoot(t)
	res := Result{}
	// fetchSucceeded=true: the bundle was fetched fine, services just
	// weren't kickstarted because the operator stopped them.
	finalize(res, "manually_stopped", false, true)
	s, err := state.ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if s.LastError != "manually_stopped" {
		t.Errorf("LastError = %q, want manually_stopped (menubar needs the sentinel to render grey-stopped instead of yellow-degraded)", s.LastError)
	}
}

// TestFinalize_HardFetchFailureSurfacesOnLastError covers the
// no-cache-available path: cphttp.Fetch failed AND state.ReadBundle
// failed, so the tick errors out with errKey="fetch_failed". Operators
// must see this on LastError (it's fatal — daemon has no usable config
// this tick), and LastFetchError mirrors it for granular reporting.
func TestFinalize_HardFetchFailureSurfacesOnLastError(t *testing.T) {
	withStateRoot(t)
	res := Result{Err: errors.New("fetch and cache both failed")}
	finalize(res, "fetch_failed", false, false)
	s, err := state.ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if s.LastError != "fetch_failed" {
		t.Errorf("LastError = %q, want fetch_failed", s.LastError)
	}
	if s.LastFetchError != "fetch_failed" {
		t.Errorf("LastFetchError = %q, want fetch_failed", s.LastFetchError)
	}
}

// TestFinalize_DoesNotStampCurrentIssuedAtWhenLiveDiffers locks in the
// review iter 4 fix: status.CurrentIssuedAt must only advance when the
// live sing-box/xray configs on disk actually match the bundle whose
// IssuedAt is res.BundleIssuedAt. Without the LiveMatchesBundle gate,
// every post-parse failure (validate_*, promote_*, kickstart_*) would
// quietly advance CurrentIssuedAt to a bundle that never reached the
// live configs — `bb-vpn status` would lie about what the services are
// running.
func TestFinalize_DoesNotStampCurrentIssuedAtWhenLiveDiffers(t *testing.T) {
	withStateRoot(t)
	// Seed status.json with a known "old" IssuedAt to detect any
	// silent overwrite by the failure path.
	const oldIssuedAt = "2026-01-01T00:00:00Z"
	if err := state.WriteStatus(state.Status{CurrentIssuedAt: oldIssuedAt, CurrentServerCount: 3}); err != nil {
		t.Fatalf("seed status: %v", err)
	}
	// Simulate a mid-pipeline failure: bundle parsed (BundleIssuedAt set,
	// ServerCount set) but render/promote/kickstart failed
	// (LiveMatchesBundle stays false).
	res := Result{
		BundleIssuedAt:    "2026-05-19T12:00:00Z",
		ServerCount:       5,
		LiveMatchesBundle: false,
		Err:               errors.New("simulated mid-pipeline failure"),
	}
	// fetchSucceeded=true: the fresh fetch worked, only later
	// pipeline steps failed. LastFetchError must be cleared.
	finalize(res, "promote_failed", false, true)
	s, err := state.ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if s.CurrentIssuedAt != oldIssuedAt {
		t.Errorf("CurrentIssuedAt = %q, want %q (must not advance when live configs don't match bundle)", s.CurrentIssuedAt, oldIssuedAt)
	}
	if s.CurrentServerCount != 3 {
		t.Errorf("CurrentServerCount = %d, want 3 (must not advance when live configs don't match bundle)", s.CurrentServerCount)
	}
	if s.LastError != "promote_failed" {
		t.Errorf("LastError = %q, want promote_failed", s.LastError)
	}
}

// TestFinalize_StampsCurrentIssuedAtWhenLiveMatches is the positive
// case: when LiveMatchesBundle is true (Step 6 promote succeeded OR
// meaningful-change short-circuit), CurrentIssuedAt must advance so
// operators see the bundle the daemon is actually running.
func TestFinalize_StampsCurrentIssuedAtWhenLiveMatches(t *testing.T) {
	withStateRoot(t)
	const newIssuedAt = "2026-05-19T12:00:00Z"
	res := Result{
		BundleIssuedAt:    newIssuedAt,
		ServerCount:       5,
		LiveMatchesBundle: true,
	}
	finalize(res, "", false, true)
	s, err := state.ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if s.CurrentIssuedAt != newIssuedAt {
		t.Errorf("CurrentIssuedAt = %q, want %q", s.CurrentIssuedAt, newIssuedAt)
	}
	if s.CurrentServerCount != 5 {
		t.Errorf("CurrentServerCount = %d, want 5", s.CurrentServerCount)
	}
}

// TestFinalize_FetchSucceededClearsLastFetchError covers the regression
// surfaced by copilot review: when a tick fetches a fresh bundle but
// fails at a later stage (validate/render/promote), LastFetchError
// must be cleared because the control-plane fetch path itself worked.
// Without this, a stale LastFetchError from an earlier degraded tick
// would persist forever once the control plane came back up.
func TestFinalize_FetchSucceededClearsLastFetchError(t *testing.T) {
	withStateRoot(t)
	// Seed status.json so we can detect that LastFetchError gets
	// cleared on a fresh successful fetch even though the overall
	// tick still ends in failure.
	if err := state.WriteStatus(state.Status{LastFetchError: "fetch_failed_using_cached"}); err != nil {
		t.Fatalf("seed status: %v", err)
	}
	res := Result{Err: errors.New("validate failed")}
	finalize(res, "validate_singbox_failed", false, true)
	s, err := state.ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if s.LastFetchError != "" {
		t.Errorf("LastFetchError = %q, want empty (fresh fetch succeeded; CP is reachable)", s.LastFetchError)
	}
	if s.LastError != "validate_singbox_failed" {
		t.Errorf("LastError = %q, want validate_singbox_failed", s.LastError)
	}
}

// TestFinalize_InboxDrainErrSurfaces locks in that a non-fatal
// DrainInbox failure (unreadable inbox dir, perms accident, transient
// FS error) is surfaced on `bb-vpn status` so the operator notices
// new enrollment requests aren't being processed. Without this, the
// daemon would happily run on its existing identity while silently
// dropping every new enroll request.
func TestFinalize_InboxDrainErrSurfaces(t *testing.T) {
	withStateRoot(t)
	res := Result{InboxDrainErr: errors.New("permission denied")}
	finalize(res, "", false, true)
	s, err := state.ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if s.LastError != "inbox_drain_failed" {
		t.Errorf("LastError = %q, want inbox_drain_failed", s.LastError)
	}
}

// TestFinalize_PipelineErrorTakesPriorityOverInboxDrain ensures the
// inbox-drain surfacing doesn't mask a real pipeline failure. Any
// non-empty errKey wins over InboxDrainErr.
func TestFinalize_PipelineErrorTakesPriorityOverInboxDrain(t *testing.T) {
	withStateRoot(t)
	res := Result{
		InboxDrainErr: errors.New("permission denied"),
		Err:           errors.New("validate failed"),
	}
	finalize(res, "validate_singbox_failed", false, true)
	s, err := state.ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if s.LastError != "validate_singbox_failed" {
		t.Errorf("LastError = %q, want validate_singbox_failed (pipeline error must win over drain)", s.LastError)
	}
}
