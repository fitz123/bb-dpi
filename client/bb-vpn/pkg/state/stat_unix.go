//go:build darwin || linux

package state

import (
	"os"
	"syscall"
)

// statOwner extracts uid/gid from a FileInfo on Unix-like systems.
// Returns (0, 0) when the underlying syscall.Stat_t isn't available
// (shouldn't happen in production but keeps the helper safe).
func statOwner(info os.FileInfo) (int, int) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0
	}
	return int(st.Uid), int(st.Gid)
}
