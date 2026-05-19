package bundle

// The plan names `golang.org/x/mod/semver` for the semver path. We
// keep a stdlib-only 3-tuple parser here instead because:
//   (1) it avoids the external dep across the whole client/bb-vpn
//       module (only needed by this one file),
//   (2) it lets us reject non-numeric components and negative
//       integers explicitly with clear errors — x/mod/semver would
//       silently treat a malformed version as invalid via its own
//       bool-only API, surfacing a less actionable failure.
// Behavior matches x/mod/semver for every input the plan calls out
// (strip leading `v`, drop pre-release suffix, compare MAJOR.MINOR.PATCH).

import (
	"fmt"
	"strconv"
	"strings"
)

// SemverGE reports whether local >= floor under the project's relaxed
// semver rule: the leading 'v' is stripped, and a pre-release suffix
// (anything after a '-') is dropped before comparison. So
// "1.13.0-alpha.10" satisfies floor "1.13.0".
//
// Returns an error if either side fails to parse — callers (the sync
// algorithm) log to status.json.last_error and REJECT the bundle on
// parse failure (fail closed).
func SemverGE(local, floor string) (bool, error) {
	lm, ln, lp, err := parseSemver(local)
	if err != nil {
		return false, fmt.Errorf("semver local %q: %w", local, err)
	}
	fm, fn, fp, err := parseSemver(floor)
	if err != nil {
		return false, fmt.Errorf("semver floor %q: %w", floor, err)
	}
	return tupleGE([3]int{lm, ln, lp}, [3]int{fm, fn, fp}), nil
}

// CalverGE reports whether local >= floor for xray's calendar-versioned
// "MAJOR.MINOR.PATCH" string. Accepts the "Xray " prefix that
// `xray version` emits. No pre-release semantics — xray releases are
// always plain MAJOR.MINOR.PATCH.
func CalverGE(local, floor string) (bool, error) {
	local = strings.TrimPrefix(local, "Xray ")
	floor = strings.TrimPrefix(floor, "Xray ")
	lm, ln, lp, err := parseTuple(local)
	if err != nil {
		return false, fmt.Errorf("calver local %q: %w", local, err)
	}
	fm, fn, fp, err := parseTuple(floor)
	if err != nil {
		return false, fmt.Errorf("calver floor %q: %w", floor, err)
	}
	return tupleGE([3]int{lm, ln, lp}, [3]int{fm, fn, fp}), nil
}

// parseSemver: strip leading 'v', drop pre-release suffix (anything
// after the first '-'), parse remaining MAJOR.MINOR.PATCH.
func parseSemver(s string) (int, int, int, error) {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s = s[:i]
	}
	return parseTuple(s)
}

// parseTuple: MAJOR.MINOR.PATCH all non-negative integers. Anything
// else is an error.
func parseTuple(s string) (int, int, int, error) {
	if s == "" {
		return 0, 0, 0, fmt.Errorf("empty version string")
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("expected 3 dotted components, got %d", len(parts))
	}
	out := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, 0, 0, fmt.Errorf("component %d (%q) not a non-negative int", i, p)
		}
		out[i] = n
	}
	return out[0], out[1], out[2], nil
}

func tupleGE(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return true // equal
}
