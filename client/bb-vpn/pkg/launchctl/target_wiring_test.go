package launchctl

import (
	"testing"

	"bb-dpi/client/bb-vpn/pkg/cphttp"
	"bb-dpi/client/bb-vpn/pkg/state"
)

// TestTargetLiteralsAgree guards the deliberate duplication of the
// "test"/"prod" literals between pkg/state and pkg/cphttp. sync.Tick
// converts state.ActiveTarget() to a cphttp.Target by value at the Fetch
// call site (sync.go). If either package's literal ever drifts, that
// conversion would silently map the operator's "test" selection to prod
// behavior — cphttp.Fetch treats anything != TargetTest as prod, so the
// failure is a quiet fail-to-prod that masks the bug instead of surfacing
// it, and no other test would catch it. This pins the contract.
func TestTargetLiteralsAgree(t *testing.T) {
	if cphttp.Target(state.TargetTest) != cphttp.TargetTest {
		t.Errorf("literal drift: state.TargetTest %q != cphttp.TargetTest %q", state.TargetTest, cphttp.TargetTest)
	}
	if cphttp.Target(state.TargetProd) != cphttp.TargetProd {
		t.Errorf("literal drift: state.TargetProd %q != cphttp.TargetProd %q", state.TargetProd, cphttp.TargetProd)
	}
}
