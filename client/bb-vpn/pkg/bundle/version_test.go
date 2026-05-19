package bundle

import "testing"

// The plan's "Version comparison contract" calls out specific
// representative inputs for fuzz coverage:
//   1.13.0, v1.13.0, 1.13.0-alpha.10, 25.12.8, Xray 25.12.8, empty, malformed
// The test cases below enumerate each.

func TestSemverGE(t *testing.T) {
	cases := []struct {
		local, floor string
		want         bool
		wantErr      bool
	}{
		// exact match
		{"1.13.0", "1.13.0", true, false},
		// leading 'v' on local
		{"v1.13.0", "1.13.0", true, false},
		// leading 'v' on floor
		{"1.13.0", "v1.13.0", true, false},
		// pre-release suffix on local satisfies plain floor
		{"1.13.0-alpha.10", "1.13.0", true, false},
		// pre-release suffix stripped on both sides
		{"1.13.0-rc.1", "1.13.0-alpha.10", true, false},
		// higher patch
		{"1.13.1", "1.13.0", true, false},
		// lower patch
		{"1.13.0", "1.13.1", false, false},
		// higher minor
		{"1.14.0", "1.13.99", true, false},
		// lower minor
		{"1.12.99", "1.13.0", false, false},
		// higher major
		{"2.0.0", "1.99.99", true, false},
		// lower major
		{"1.99.99", "2.0.0", false, false},
	}
	for _, c := range cases {
		got, err := SemverGE(c.local, c.floor)
		if (err != nil) != c.wantErr {
			t.Errorf("SemverGE(%q,%q): err=%v wantErr=%v", c.local, c.floor, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("SemverGE(%q,%q) = %v, want %v", c.local, c.floor, got, c.want)
		}
	}
}

func TestSemverGE_ParseFailures(t *testing.T) {
	// The plan: parse failure on EITHER side → caller (sync algorithm)
	// logs to status.json.last_error and rejects the bundle. We surface
	// the error; we don't silently default to false.
	cases := []struct{ local, floor string }{
		{"", "1.13.0"},
		{"1.13.0", ""},
		{"abc", "1.13.0"},
		{"1.13", "1.13.0"},      // 2 components
		{"1.13.0.4", "1.13.0"},  // 4 components
		{"1.13.x", "1.13.0"},    // non-numeric component
		{"-1.0.0", "1.13.0"},    // leading '-' starts a pre-release suffix → strip empties the string
		{"1.13.0", "garbage"},
	}
	for _, c := range cases {
		if _, err := SemverGE(c.local, c.floor); err == nil {
			t.Errorf("SemverGE(%q,%q): want error, got nil", c.local, c.floor)
		}
	}
}

func TestCalverGE(t *testing.T) {
	cases := []struct {
		local, floor string
		want         bool
		wantErr      bool
	}{
		// exact
		{"25.12.8", "25.12.8", true, false},
		// "Xray " prefix on local
		{"Xray 25.12.8", "25.12.8", true, false},
		// "Xray " prefix on floor too
		{"Xray 25.12.8", "Xray 25.12.8", true, false},
		// higher patch
		{"25.12.9", "25.12.8", true, false},
		// lower patch
		{"25.12.7", "25.12.8", false, false},
		// higher minor
		{"25.13.0", "25.12.99", true, false},
		// across year (major) boundaries
		{"26.1.0", "25.12.99", true, false},
		{"24.12.99", "25.1.0", false, false},
	}
	for _, c := range cases {
		got, err := CalverGE(c.local, c.floor)
		if (err != nil) != c.wantErr {
			t.Errorf("CalverGE(%q,%q): err=%v wantErr=%v", c.local, c.floor, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("CalverGE(%q,%q) = %v, want %v", c.local, c.floor, got, c.want)
		}
	}
}

func TestCalverGE_ParseFailures(t *testing.T) {
	cases := []struct{ local, floor string }{
		{"", "25.12.8"},
		{"25.12.8", ""},
		{"Xray ", "25.12.8"}, // empty after prefix strip
		{"abc", "25.12.8"},
		{"25.12", "25.12.8"},
		{"25.12.8.1", "25.12.8"},
		// Calver path does NOT strip '-' (no pre-release semantics),
		// so a negative integer reaches parseTuple's `n < 0` guard.
		// This is the case that exercises that branch.
		{"25.12.-8", "25.12.8"},
		{"-25.12.8", "25.12.8"},
	}
	for _, c := range cases {
		if _, err := CalverGE(c.local, c.floor); err == nil {
			t.Errorf("CalverGE(%q,%q): want error, got nil", c.local, c.floor)
		}
	}
}
