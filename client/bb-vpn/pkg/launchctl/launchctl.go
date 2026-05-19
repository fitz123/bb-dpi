// Package launchctl wraps the launchctl(1) subset bb-vpn needs:
// bootstrap, kickstart, print (for liveness), bootout, disable, enable.
//
// Functions return wrapped errors so the sync loop can distinguish
// "service does not exist yet" from "service crashed on smoke test".
package launchctl

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Service identifies a system-domain LaunchDaemon by label.
type Service string

const (
	SingBox Service = "com.sing-box-vpn"
	Xray    Service = "com.xray-xhttp"
	BBVPN   Service = "com.bb-dpi.bb-vpn-sync"
)

func (s Service) Target() string { return "system/" + string(s) }

// Print returns true when the service is loaded AND has an active
// pid > 0 (i.e., the daemon is actually running, not just registered).
//
// `launchctl print system/<label>` exits non-zero when the service is
// not bootstrapped at all — that's NotBootstrapped.
func Print(s Service) (running bool, err error) {
	out, runErr := exec.Command("launchctl", "print", s.Target()).CombinedOutput()
	if runErr != nil {
		// launchctl returns 113 for "couldn't find specified service".
		if isNotFoundOutput(out) {
			return false, ErrNotBootstrapped
		}
		return false, fmt.Errorf("launchctl print %s: %w (out: %s)", s, runErr, strings.TrimSpace(string(out)))
	}
	// Look for `state = running` or a non-zero `pid = N`.
	text := string(out)
	if strings.Contains(text, "state = running") {
		return true, nil
	}
	return false, nil
}

// ErrNotBootstrapped is returned by Print when the LaunchDaemon has
// not yet been bootstrap'd into the system domain. The sync loop
// uses this to decide whether to call Bootstrap before Kickstart.
var ErrNotBootstrapped = errors.New("launchctl: service not bootstrapped")

// Bootstrap loads a LaunchDaemon plist into the system domain.
// Path must be absolute (typically /Library/LaunchDaemons/<label>.plist).
func Bootstrap(plistPath string) error {
	return run("launchctl", "bootstrap", "system", plistPath)
}

// Kickstart starts a bootstrapped service (or restarts it if -k is
// passed). bb-vpn always uses -k so a re-render flips the running
// daemon onto the new config.
func Kickstart(s Service) error {
	return run("launchctl", "kickstart", "-k", s.Target())
}

// Bootout unloads a service from the system domain. Used when the
// rendered config no longer needs xray (proto=tcp-vision).
func Bootout(s Service) error {
	err := run("launchctl", "bootout", s.Target())
	if err == nil {
		return nil
	}
	// Idempotency: bootout of an already-unloaded service is fine.
	if strings.Contains(err.Error(), "No such process") {
		return nil
	}
	return err
}

// Disable persists a "do not start" flag for the service that
// survives reboot. Used by the runtime_blackhole circuit breaker
// — without an explicit Enable, the Mac stays dead-VPN forever.
func Disable(s Service) error {
	return run("launchctl", "disable", s.Target())
}

// Enable is the inverse of Disable. Called by `bb-vpn recover`.
func Enable(s Service) error {
	return run("launchctl", "enable", s.Target())
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w (out: %s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isNotFoundOutput(out []byte) bool {
	s := string(out)
	return strings.Contains(s, "Could not find service") ||
		strings.Contains(s, "Service is disabled") ||
		strings.Contains(s, "No such process")
}
