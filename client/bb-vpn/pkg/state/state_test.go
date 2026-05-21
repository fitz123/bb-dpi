package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// withRoot points the package at a tempdir for the duration of t.
// Cleanup restores the prior value so parallel tests don't bleed.
func withRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("BB_VPN_STATE_ROOT", dir)
	// Pre-create subdirs the production .pkg postinstall would.
	for _, sub := range []string{InboxDir, BundlesDir, StagingDir, ConfigsDir, BinDir} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	return dir
}

func TestValidateUUID(t *testing.T) {
	good := []string{
		"00000000-0000-0000-0000-000000000000",
		"deadbeef-cafe-babe-1234-567890abcdef",
	}
	for _, u := range good {
		if err := ValidateUUID(u); err != nil {
			t.Errorf("ValidateUUID(%q): unexpected error: %v", u, err)
		}
	}
	bad := []string{
		"",
		"not-a-uuid",
		"00000000-0000-0000-0000-00000000000",    // 35 chars
		"00000000-0000-0000-0000-0000000000000",  // 37 chars
		"GGGGGGGG-0000-0000-0000-000000000000",   // non-hex
		"00000000-0000-0000-0000-000000000000\n", // trailing newline
		"DEADBEEF-CAFE-BABE-1234-567890ABCDEF",   // uppercase
	}
	for _, u := range bad {
		if err := ValidateUUID(u); err == nil {
			t.Errorf("ValidateUUID(%q): want error, got nil", u)
		}
	}
}

func TestIdentityRoundTrip(t *testing.T) {
	withRoot(t)
	want := Identity{UUID: "00000000-0000-0000-0000-000000000000"}
	if err := WriteIdentity(want); err != nil {
		t.Fatalf("WriteIdentity: %v", err)
	}
	got, err := ReadIdentity()
	if err != nil {
		t.Fatalf("ReadIdentity: %v", err)
	}
	if got != want {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, want)
	}
	// Mode 0600 enforced.
	info, err := os.Stat(Path(IdentityFile))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode() & 0o777; got != 0o600 {
		t.Errorf("identity.json mode = %o, want 0600", got)
	}
}

func TestIdentityWrite_RejectsBadUUID(t *testing.T) {
	withRoot(t)
	if err := WriteIdentity(Identity{UUID: "bogus"}); err == nil {
		t.Errorf("WriteIdentity with bogus UUID: want error, got nil")
	}
}

func TestStatusRoundTrip(t *testing.T) {
	withRoot(t)
	want := Status{
		LastSync:           "2026-05-19T10:00:00Z",
		LastError:          "",
		LastFetchError:     "fetch_failed_using_cached",
		CurrentIssuedAt:    "2026-05-19T09:00:00Z",
		CurrentServerCount: 2,
		LastIdentityChange: "2026-05-01T00:00:00Z",
		SingBoxRunning:     true,
		XrayRunning:        false,
		XrayNeeded:         false,
	}
	if err := WriteStatus(want); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	got, err := ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if got != want {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestStatusRead_MissingFileReturnsZero(t *testing.T) {
	withRoot(t)
	got, err := ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus missing: %v", err)
	}
	if (got != Status{}) {
		t.Errorf("missing status.json should return zero-value Status, got %+v", got)
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := withRoot(t)
	path := filepath.Join(dir, "test.json")
	data := []byte(`{"hello":"world"}`)
	if err := WriteAtomic(path, data, 0o644); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content mismatch: got %q want %q", got, data)
	}
	// Verify no leftover .tmp file.
	matches, _ := filepath.Glob(filepath.Join(dir, ".test.json.tmp.*"))
	if len(matches) != 0 {
		t.Errorf("leftover tmp files: %v", matches)
	}
}

func TestInbox_WriteRead(t *testing.T) {
	withRoot(t)
	req := EnrollRequest{UUID: "00000000-0000-0000-0000-000000000000", Source: "cli"}
	if err := WriteEnroll(req); err != nil {
		t.Fatalf("WriteEnroll: %v", err)
	}

	winner, processed, err := DrainInbox()
	if err != nil {
		t.Fatalf("DrainInbox: %v", err)
	}
	if winner == nil {
		t.Fatalf("DrainInbox: nil winner, want %+v", req)
	}
	if winner.UUID != req.UUID {
		t.Errorf("winner UUID = %q, want %q", winner.UUID, req.UUID)
	}
	if len(processed) != 1 {
		t.Errorf("processed count = %d, want 1", len(processed))
	}
	// Inbox should be empty post-drain.
	entries, _ := os.ReadDir(Path(InboxDir))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "enroll-") {
			t.Errorf("inbox still has %s after drain", e.Name())
		}
	}
}

func TestInbox_LexOrderWinner(t *testing.T) {
	withRoot(t)
	// Manually write three enroll files with explicit ordered names so
	// we control the "winner" deterministically. WriteEnroll uses
	// nanosecond timestamps which can collide on fast machines.
	for i, uuid := range []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
		"00000000-0000-0000-0000-000000000003",
	} {
		name := "enroll-" + strings.Repeat("0", 18) + string(rune('0'+i)) + "0.json"
		path := filepath.Join(Path(InboxDir), name)
		data, _ := json.Marshal(EnrollRequest{UUID: uuid, Source: "test"})
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	winner, processed, err := DrainInbox()
	if err != nil {
		t.Fatalf("DrainInbox: %v", err)
	}
	if len(processed) != 3 {
		t.Errorf("processed = %d, want 3", len(processed))
	}
	if winner == nil || winner.UUID != "00000000-0000-0000-0000-000000000003" {
		t.Errorf("winner = %+v, want UUID ending in 03 (last in lex order)", winner)
	}
}

func TestInbox_RejectsOversized(t *testing.T) {
	withRoot(t)
	// 5KB payload — over the 4KB cap.
	path := filepath.Join(Path(InboxDir), "enroll-00000000000000000000.json")
	huge := strings.Repeat("a", 5000)
	if err := os.WriteFile(path, []byte(huge), 0o600); err != nil {
		t.Fatal(err)
	}
	_, processed, err := DrainInbox()
	if err != nil {
		t.Fatalf("DrainInbox: %v", err)
	}
	if len(processed) != 1 || processed[0].UUID != "" {
		t.Errorf("oversized should be processed with empty UUID, got %+v", processed)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("oversized file should be deleted; stat err: %v", err)
	}
}

func TestArchiveBundleBlackhole_CapsRetention(t *testing.T) {
	withRoot(t)
	bdir := Path(BundlesDir)

	// Pre-create 8 fake blackhole archives with sortable timestamped
	// names. Lex order == chronological order; the smallest names are
	// "oldest" and should be evicted first.
	preExisting := []string{
		"blackhole-20260101T000001Z.json",
		"blackhole-20260101T000002Z.json",
		"blackhole-20260101T000003Z.json",
		"blackhole-20260101T000004Z.json",
		"blackhole-20260101T000005Z.json",
		"blackhole-20260101T000006Z.json",
		"blackhole-20260101T000007Z.json",
		"blackhole-20260101T000008Z.json",
	}
	for _, n := range preExisting {
		if err := os.WriteFile(filepath.Join(bdir, n), []byte(`{"v":0}`), 0o600); err != nil {
			t.Fatalf("seed %s: %v", n, err)
		}
	}

	// Seed a current.json so ArchiveBundleBlackhole has something to
	// archive (and thus runs the retention pass).
	if err := os.WriteFile(Path(CurrentBundle), []byte(`{"v":9}`), 0o600); err != nil {
		t.Fatalf("seed current: %v", err)
	}

	if err := ArchiveBundleBlackhole(); err != nil {
		t.Fatalf("ArchiveBundleBlackhole: %v", err)
	}

	entries, err := os.ReadDir(bdir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var bh []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, "blackhole-") && strings.HasSuffix(n, ".json") {
			bh = append(bh, n)
		}
	}
	if len(bh) != 5 {
		t.Fatalf("expected 5 blackhole archives after retention, got %d: %v", len(bh), bh)
	}

	// Newest pre-existing must survive; oldest must be gone.
	sort.Strings(bh)
	wantPresent := "blackhole-20260101T000008Z.json"
	wantAbsent := "blackhole-20260101T000001Z.json"
	found := map[string]bool{}
	for _, n := range bh {
		found[n] = true
	}
	if !found[wantPresent] {
		t.Errorf("expected newest pre-existing %q to survive, got %v", wantPresent, bh)
	}
	if found[wantAbsent] {
		t.Errorf("expected oldest pre-existing %q to be evicted, got %v", wantAbsent, bh)
	}

	// current.json should be gone (renamed into an archive).
	if _, err := os.Stat(Path(CurrentBundle)); !os.IsNotExist(err) {
		t.Errorf("current.json should be archived, stat err: %v", err)
	}
}

func TestArchiveBundleBlackhole_NoCurrentIsNoop(t *testing.T) {
	withRoot(t)
	// No current.json present → returns nil, no archives written.
	if err := ArchiveBundleBlackhole(); err != nil {
		t.Fatalf("ArchiveBundleBlackhole on empty state: %v", err)
	}
	entries, _ := os.ReadDir(Path(BundlesDir))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "blackhole-") {
			t.Errorf("unexpected blackhole archive created: %s", e.Name())
		}
	}
}

func TestPromoteBundle_Rotates(t *testing.T) {
	withRoot(t)
	if err := PromoteBundle([]byte(`{"v":1}`)); err != nil {
		t.Fatalf("PromoteBundle 1: %v", err)
	}
	if err := PromoteBundle([]byte(`{"v":2}`)); err != nil {
		t.Fatalf("PromoteBundle 2: %v", err)
	}
	cur, err := ReadBundle()
	if err != nil {
		t.Fatalf("ReadBundle: %v", err)
	}
	if string(cur) != `{"v":2}` {
		t.Errorf("current = %q, want v:2", cur)
	}
	prev, err := ReadPreviousBundle()
	if err != nil {
		t.Fatalf("ReadPreviousBundle: %v", err)
	}
	if string(prev) != `{"v":1}` {
		t.Errorf("previous = %q, want v:1", prev)
	}
}

// TestPromoteBundle_FileModes guards the asymmetric file-mode contract:
// current.json must be 0o644 so the menubar (console-user context) can
// read it for the tag→host map used by clash-api "exit server" display;
// previous.json must stay 0o600 because it's a forensic snapshot, not
// for live UI consumption. Future broad-stroke "tighten all bundle
// files" changes will trip this test.
func TestPromoteBundle_FileModes(t *testing.T) {
	withRoot(t)
	if err := PromoteBundle([]byte(`{"v":1}`)); err != nil {
		t.Fatalf("PromoteBundle 1: %v", err)
	}
	// After first promote, only current.json exists.
	curInfo, err := os.Stat(Path(CurrentBundle))
	if err != nil {
		t.Fatalf("stat current: %v", err)
	}
	if got := curInfo.Mode().Perm(); got != 0o644 {
		t.Errorf("current.json mode = %o, want 0644 (menubar needs read access)", got)
	}

	// Trigger a rotation so previous.json gets created from current.
	if err := PromoteBundle([]byte(`{"v":2}`)); err != nil {
		t.Fatalf("PromoteBundle 2: %v", err)
	}
	curInfo, err = os.Stat(Path(CurrentBundle))
	if err != nil {
		t.Fatalf("stat current after rotate: %v", err)
	}
	if got := curInfo.Mode().Perm(); got != 0o644 {
		t.Errorf("current.json mode after rotate = %o, want 0644", got)
	}
	prevInfo, err := os.Stat(Path(PreviousBundle))
	if err != nil {
		t.Fatalf("stat previous: %v", err)
	}
	if got := prevInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("previous.json mode = %o, want 0600 (forensic snapshot, not for menubar)", got)
	}
}
