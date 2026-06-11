package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// WriteAtomic writes data to path via the standard tmp + fsync +
// rename + dir-fsync dance. mode applies to the final file.
//
// The temp filename is `<base>.tmp.<pid>` (per-process unique) so
// concurrent writers under the same dir do not race on the same
// dotfile name. Production has a single root sync daemon, so contention
// only matters during dev/test, but the cost is one syscall.
//
// Replaces the cmd/bb-vpn/render.go writeFile from PR C — the
// daemon side needs the dir-fsync that the debug CLI omitted.
func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp := filepath.Join(dir, "."+base+".tmp."+strconv.Itoa(os.Getpid()))

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("state: open %s: %w", tmp, err)
	}
	// OpenFile's mode is masked by the process umask, so a restrictive
	// umask (e.g. 077 inherited by the LaunchDaemon) would silently create
	// a 0600 file when 0644 was requested — breaking the world-readable
	// state files (status.json, the target selector) that the user-space
	// menubar and un-sudo'd `bb-vpn` must read. fchmod the open fd to
	// enforce the exact requested mode regardless of umask.
	if err := f.Chmod(mode); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("state: chmod %s: %w", tmp, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("state: write %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("state: fsync %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("state: close %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("state: rename %s -> %s: %w", tmp, path, err)
	}
	// fsync the directory so the rename survives a crash. Best-effort
	// only — some filesystems (tmpfs in CI) do not support O_RDONLY on
	// dirs; surface the error but don't fail the write.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
