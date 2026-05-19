package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"
)

// uuidPattern matches the canonical 8-4-4-4-12 hex UUID form the
// xray REALITY layer accepts. Filter at the daemon boundary so a
// malformed enrollment never reaches identity.json.
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Identity is the wire shape of identity.json.
type Identity struct {
	UUID string `json:"uuid"`
}

// ValidateUUID is exported so cmd/bb-vpn/enroll.go can reject a bad
// URI before writing to inbox/.
func ValidateUUID(u string) error {
	if u == "" {
		return errors.New("uuid: empty")
	}
	if !uuidPattern.MatchString(u) {
		return fmt.Errorf("uuid %q: must match 8-4-4-4-12 lowercase hex", u)
	}
	return nil
}

// ReadIdentity returns the current UUID. Returns os.ErrNotExist
// when identity.json is missing — callers (the sync loop) treat
// "no identity yet" as "wait for enrollment".
func ReadIdentity() (Identity, error) {
	data, err := os.ReadFile(Path(IdentityFile))
	if err != nil {
		return Identity{}, err
	}
	var id Identity
	if err := json.Unmarshal(data, &id); err != nil {
		return Identity{}, fmt.Errorf("state: parse identity.json: %w", err)
	}
	if err := ValidateUUID(id.UUID); err != nil {
		return Identity{}, fmt.Errorf("state: identity.json invalid: %w", err)
	}
	return id, nil
}

// WriteIdentity writes identity.json atomically. Mode 0600 so only
// root can read the UUID secret.
func WriteIdentity(id Identity) error {
	if err := ValidateUUID(id.UUID); err != nil {
		return err
	}
	data, err := json.Marshal(id)
	if err != nil {
		return fmt.Errorf("state: marshal identity: %w", err)
	}
	return WriteAtomic(Path(IdentityFile), data, 0o600)
}

// IdentityLogEntry is one line of the JSONL audit log. The UUID is
// stored as the first 8 hex chars of its sha256 — avoids leaking the
// full secret into a 0600 file while still giving forensics enough
// to correlate enroll events.
type IdentityLogEntry struct {
	Timestamp  string `json:"ts"`
	UUIDPrefix string `json:"uuid_sha256_prefix"`
	SourcePath string `json:"source_path"`
	SourceUID  int    `json:"source_uid"`
	SourceGID  int    `json:"source_gid"`
}

// AppendIdentityLog appends one JSONL line. Caller passes the raw
// UUID; AppendIdentityLog hashes it. 10MB rotation: when the log
// exceeds 10MB, current contents move to identity.log.1 and the
// fresh log starts empty.
func AppendIdentityLog(uuid, sourcePath string, sourceUID, sourceGID int) error {
	hashHex := hashUUID(uuid)
	entry := IdentityLogEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		UUIDPrefix: hashHex,
		SourcePath: sourcePath,
		SourceUID:  sourceUID,
		SourceGID:  sourceGID,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	logPath := Path(IdentityLogFile)
	if info, err := os.Stat(logPath); err == nil && info.Size() > 10*1024*1024 {
		// rotate
		rotated := logPath + ".1"
		_ = os.Remove(rotated)
		if err := os.Rename(logPath, rotated); err != nil {
			return fmt.Errorf("state: rotate identity.log: %w", err)
		}
	}
	f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("state: open identity.log: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("state: append identity.log: %w", err)
	}
	return nil
}

func hashUUID(uuid string) string {
	sum := sha256.Sum256([]byte(uuid))
	return hex.EncodeToString(sum[:4]) // 8 hex chars
}
