//go:build unix

package state

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestWriteAtomic_EnforcesModeUnderRestrictiveUmask verifies the mode
// passed to WriteAtomic lands on disk verbatim even when the process
// umask would otherwise mask it down. World-readable state (status.json,
// the target selector) must be 0644 regardless of the umask the
// LaunchDaemon inherits — a 0600 result would make every non-root reader
// (the menubar, un-sudo'd `bb-vpn`) silently fail to read it.
//
// Not parallel: syscall.Umask mutates process-global state.
func TestWriteAtomic_EnforcesModeUnderRestrictiveUmask(t *testing.T) {
	dir := t.TempDir()
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)

	path := filepath.Join(dir, "world-readable")
	if err := WriteAtomic(path, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("mode under umask 077 = %o, want 0644 (umask must not mask the requested mode)", perm)
	}
}
