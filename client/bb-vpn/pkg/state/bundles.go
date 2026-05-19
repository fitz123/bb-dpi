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
// Both files mode 0600 — Servers carries REALITY public keys and xhttp
// paths, so keep the bundle root-only to match the rest of the state tree.
//
// No-op short-circuit: if current.json's bytes equal data exactly,
// PromoteBundle returns nil WITHOUT rotating. This protects the
// previous.json "last-known-good" anchor across no-change ticks (and
// across re-fetches of a known-broken bundle that the sync loop rolled
// back from). Without this, a sequence of identical fetches would
// promote the same bytes into previous, overwriting the actually-good
// rollback target.
func PromoteBundle(data []byte) error {
	current := Path(CurrentBundle)
	previous := Path(PreviousBundle)

	if existing, err := os.ReadFile(current); err == nil && bytes.Equal(existing, data) {
		// Same content already on disk — no rotation needed.
		return nil
	}

	if _, err := os.Stat(current); err == nil {
		// Move existing current → previous.
		_ = os.Remove(previous) // ignore not-exist
		if err := os.Rename(current, previous); err != nil {
			return fmt.Errorf("state: rotate bundles: %w", err)
		}
	}
	return WriteAtomic(current, data, 0o600)
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
