package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EnrollRequest is the wire shape of inbox/enroll-*.json. Written
// by `bb-vpn enroll` or the menu-bar app (user-space), consumed by
// the root daemon during sync.
type EnrollRequest struct {
	UUID string `json:"uuid"`
	// Source is informational ("cli" or "menubar"); not load-bearing.
	Source string `json:"source,omitempty"`
}

// EnrollFilename returns the canonical "inbox/enroll-<%020d ns>.json"
// for a fresh request. Zero-padded nanosecond timestamp so lexical
// directory order matches enrollment order — pkg/launchctl drains
// inbox in lex order.
func EnrollFilename() string {
	ns := time.Now().UTC().UnixNano()
	return fmt.Sprintf("enroll-%020d.json", ns)
}

// WriteEnroll drops a freshly-named request into inbox/. Writes
// via a dotfile + rename so the daemon never sees a partial JSON.
// inbox/ is mode 1733, so a user-space caller can write but only
// root can delete arbitrary files.
func WriteEnroll(req EnrollRequest) error {
	if err := ValidateUUID(req.UUID); err != nil {
		return err
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	name := EnrollFilename()
	final := Path(InboxDir, name)
	tmp := Path(InboxDir, "."+name+".tmp")

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("state: open inbox tmp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("state: write inbox tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("state: fsync inbox tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("state: rename inbox tmp -> %s: %w", final, err)
	}
	return nil
}

// DrainInboxResult describes a single processed enrollment.
type DrainInboxResult struct {
	Path string
	UUID string
	UID  int
	GID  int
}

// DrainInbox processes every enroll-*.json under inbox/ in lex order.
// For each: lstat (reject symlinks), 4KB size cap, parse, UUID
// validation, audit-log, delete. Returns the LAST validated request
// (winner) plus all processed entries for downstream logging.
//
// Returns nil winner + nil err when inbox is empty.
//
// Errors during processing of a single file are logged via the
// caller's logger (via the returned []DrainInboxResult entries having
// a non-empty .UUID == "") but do not abort the drain — the daemon
// must keep going.
func DrainInbox() (winner *DrainInboxResult, processed []DrainInboxResult, err error) {
	entries, err := os.ReadDir(Path(InboxDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("state: read inbox: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		n := e.Name()
		if !strings.HasPrefix(n, "enroll-") || !strings.HasSuffix(n, ".json") {
			// Drain sync-now.touch by deletion if it appears — used
			// as an "ask for an immediate sync tick" signal.
			if n == "sync-now.touch" {
				_ = os.Remove(filepath.Join(Path(InboxDir), n))
			}
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names) // lex order == ns-timestamp order

	for _, n := range names {
		path := filepath.Join(Path(InboxDir), n)
		req, uid, gid, perr := processEnrollFile(path)
		// Delete the file regardless — we don't want repeated
		// processing of a malformed entry.
		_ = os.Remove(path)
		if perr != nil {
			processed = append(processed, DrainInboxResult{Path: path, UUID: ""})
			continue
		}
		result := DrainInboxResult{Path: path, UUID: req.UUID, UID: uid, GID: gid}
		processed = append(processed, result)
		_ = AppendIdentityLog(req.UUID, path, uid, gid)
		winner = &DrainInboxResult{Path: path, UUID: req.UUID, UID: uid, GID: gid}
	}
	return winner, processed, nil
}

func processEnrollFile(path string) (EnrollRequest, int, int, error) {
	// lstat first — reject symlinks (caller could point at a sensitive
	// file via symlink given inbox/ is world-writable).
	info, err := os.Lstat(path)
	if err != nil {
		return EnrollRequest{}, 0, 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return EnrollRequest{}, 0, 0, fmt.Errorf("state: inbox %s: symlink rejected", path)
	}
	if info.Size() > 4096 {
		return EnrollRequest{}, 0, 0, fmt.Errorf("state: inbox %s: exceeds 4KB", path)
	}
	uid, gid := statOwner(info)

	data, err := os.ReadFile(path)
	if err != nil {
		return EnrollRequest{}, uid, gid, err
	}
	var req EnrollRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return EnrollRequest{}, uid, gid, fmt.Errorf("state: inbox %s: %w", path, err)
	}
	if err := ValidateUUID(req.UUID); err != nil {
		return EnrollRequest{}, uid, gid, err
	}
	return req, uid, gid, nil
}
