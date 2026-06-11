package state

import (
	"fmt"
	"os"
	"strings"
)

// Target selects which published bundle snapshot the sync loop fetches:
// the stable prod path or the staging test path. Any persisted value
// other than "test" reads back as prod — the safe default.
//
// NOTE: pkg/cphttp declares its own Target type with the same
// "prod"/"test" literals. The duplication is deliberate — it keeps
// cphttp (token-bearing, security-sensitive) free of a state
// dependency. Keep the literals in sync with pkg/cphttp/cphttp.go;
// sync converts at the call site.
type Target string

const (
	TargetProd Target = "prod"
	TargetTest Target = "test"
)

// ActiveTarget returns the persisted publish target. A missing, empty,
// or unrecognized target file all read as TargetProd so a default
// install (or a corrupted file) never fetches the staging bundle.
func ActiveTarget() Target {
	data, err := os.ReadFile(Path(TargetFile))
	if err != nil {
		return TargetProd
	}
	if Target(strings.TrimSpace(string(data))) == TargetTest {
		return TargetTest
	}
	return TargetProd
}

// SetTarget persists the publish target. Only "test" and "prod" are
// accepted; anything else is rejected so a typo can't silently park
// the client on the prod default while the operator believes otherwise.
func SetTarget(t Target) error {
	if t != TargetProd && t != TargetTest {
		return fmt.Errorf("invalid target %q: must be %q or %q", t, TargetProd, TargetTest)
	}
	// 0o644: the target value is not a secret, and user-space readers must
	// be able to read it — the menubar and the un-sudo'd `bb-vpn target` /
	// `bb-vpn status` both call ActiveTarget(). A 0o600 (root-only) file
	// would make every non-root reader silently fall back to the prod
	// default even when test is selected. Matches status.json's mode; only
	// root (the daemon / `sudo bb-vpn target …`) can write it.
	return WriteAtomic(Path(TargetFile), []byte(t+"\n"), 0o644)
}
