package state

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// blackholeRetention caps the number of bundles/blackhole-*.json
// archives kept on disk. The retention pass runs after each new
// archive is written so a persistently broken bundle that keeps being
// re-fetched can't accumulate archives indefinitely.
const blackholeRetention = 5

// PromoteBundle rotates current → previous and writes data as new current.
// data is the raw bundle.json bytes (already validated by pkg/bundle).
//
// File modes are intentionally asymmetric:
//   - current.json is 0o644 (world-readable) so the menubar app,
//     which runs in the console-user context, can read it to build
//     the outbound-tag → server-host map for clash-api display.
//   - previous.json and blackhole-*.json stay 0o600 (root-only) —
//     they're forensic snapshots, not for live UI consumption, so
//     keep their blast radius minimal. Note: previous.json inherits
//     0o600 from os.Rename below, since current.json was previously
//     written at 0o600. After this change, current.json starts at
//     0o644 but previous.json is created via Rename which preserves
//     whatever mode current.json had at promotion time. To keep
//     previous.json strictly 0o600 we re-chmod after the rename.
//
// No secrets live in the bundle itself: it carries server hosts,
// REALITY public keys, xhttp paths, skeletons, and render flags. The
// per-client auth token lives in control-plane.json (separate file,
// stays 0o600). The rendered sing-box config — which contains all
// the same server hosts/keys derived from the bundle — already lives
// at ~/.config/sing-box/config-auto.json under the console user, so
// loosening current.json to 0o644 adds no new exposure.
//
// No-op short-circuit: if current.json's bytes equal data exactly,
// PromoteBundle returns nil WITHOUT rotating. This protects the
// previous.json "last-known-good" anchor across no-change ticks (and
// across re-fetches of a known-broken bundle that the sync loop rolled
// back from). Without this, a sequence of identical fetches would
// promote the same bytes into previous, overwriting the actually-good
// rollback target.
//
// We chmod on every promote-with-equal-bytes to heal a possible
// 0o600-from-old-binary upgrade case: pre-PR clients wrote
// current.json at 0o600, and without the chmod the first post-upgrade
// sync hits this short-circuit and leaves the mode at 0o600, keeping
// the menubar (console user) locked out indefinitely. The chmod is a
// no-op when the file is already at 0o644.
func PromoteBundle(data []byte) error {
	current := Path(CurrentBundle)
	previous := Path(PreviousBundle)

	if existing, err := os.ReadFile(current); err == nil && bytes.Equal(existing, data) {
		// Same content already on disk — no rotation needed.
		// Heal the mode in case an older binary wrote it at 0o600.
		_ = os.Chmod(current, 0o644)
		return nil
	}

	if _, err := os.Stat(current); err == nil {
		// Move existing current → previous. Rename preserves the
		// source's mode (currently 0o644 for current.json), so
		// re-chmod previous.json back to 0o600 — forensic snapshot,
		// not for live menubar consumption.
		_ = os.Remove(previous) // ignore not-exist
		if err := os.Rename(current, previous); err != nil {
			return fmt.Errorf("state: rotate bundles: %w", err)
		}
		_ = os.Chmod(previous, 0o600)
	}
	// current.json: 0o644 so the menubar (console user) can read it
	// to build the tag→host map for clash-api "exit server" display.
	// Bundle contents are non-secret (server hosts, public keys,
	// REALITY params, skeletons, render flags); the auth token lives
	// in control-plane.json at 0o600; and the same server data is
	// already exposed in ~/.config/sing-box/config-auto.json.
	return WriteAtomic(current, data, 0o644)
}

// ArchiveBundleBlackhole renames bundles/current.json to
// bundles/blackhole-<ts>.json, leaving previous.json untouched. Used
// by the runtime_blackhole recovery path so the next sync starts from
// scratch instead of retrying the bad bundle.
//
// After writing the new archive, retainNewestBlackholes is called to
// cap the on-disk archive count at blackholeRetention. Errors from the
// retention pass are ignored — it's best-effort cleanup, never fatal.
func ArchiveBundleBlackhole() error {
	current := Path(CurrentBundle)
	if _, err := os.Stat(current); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	archive := Path(BundlesDir, "blackhole-"+ts+".json")
	if err := os.Rename(current, archive); err != nil {
		return err
	}
	// Rename preserves the source's mode (current.json is 0o644 so
	// the menubar can read it), but blackhole-*.json are forensic
	// snapshots — keep them root-only at 0o600 per the PromoteBundle
	// godoc contract. Best-effort: a chmod failure here doesn't
	// invalidate the archive, and the retention pass below is also
	// best-effort.
	_ = os.Chmod(archive, 0o600)
	retainNewestBlackholes(blackholeRetention)
	return nil
}

// retainNewestBlackholes keeps at most `keep` blackhole-*.json files
// in BundlesDir and removes the rest. Filenames embed a UTC timestamp
// so lexicographic sort == chronological sort, with the newest last.
// Errors are swallowed — this is best-effort cleanup.
func retainNewestBlackholes(keep int) {
	entries, err := os.ReadDir(Path(BundlesDir))
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "blackhole-") && strings.HasSuffix(name, ".json") {
			names = append(names, name)
		}
	}
	if len(names) <= keep {
		return
	}
	sort.Strings(names)
	// Remove all but the last `keep` (newest).
	for _, n := range names[:len(names)-keep] {
		_ = os.Remove(Path(BundlesDir, n))
	}
}

// ReadBundle returns the raw bytes of bundles/current.json. Returns
// os.ErrNotExist when no bundle has been promoted yet.
func ReadBundle() ([]byte, error) {
	return os.ReadFile(Path(CurrentBundle))
}

// ReadPreviousBundle returns the raw bytes of bundles/previous.json.
// Used by the circuit breaker to roll back when the new bundle
// produces a config that crashes sing-box.
func ReadPreviousBundle() ([]byte, error) {
	return os.ReadFile(Path(PreviousBundle))
}
