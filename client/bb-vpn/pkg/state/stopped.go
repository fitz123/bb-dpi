package state

import "os"

// ManuallyStopped reports whether the operator has asked the daemon
// to stay shut down (via `sudo bb-vpn stop`). sync.Tick respects
// this flag and skips kickstart at step 7 so the VPN stays down
// across reboots until `sudo bb-vpn start` clears the flag.
func ManuallyStopped() bool {
	_, err := os.Stat(Path(ManuallyStoppedFlag))
	return err == nil
}

// SetManuallyStopped creates the flag. Idempotent — repeated `stop`
// invocations are no-ops once the flag exists.
func SetManuallyStopped() error {
	f, err := os.OpenFile(Path(ManuallyStoppedFlag), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	return f.Close()
}

// ClearManuallyStopped removes the flag. Idempotent — missing-file
// is not an error.
func ClearManuallyStopped() error {
	err := os.Remove(Path(ManuallyStoppedFlag))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
