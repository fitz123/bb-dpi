package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Status is the wire shape of status.json. World-readable (0644) so
// the menu-bar app and `bb-vpn status` can read it without sudo.
//
// LastFetchError captures the most recent control-plane fetch outcome
// independently of LastError. The sync loop falls back to the cached
// bundle when cphttp.Fetch fails, which would otherwise leave operators
// with no signal that the control plane is unreachable. Cleared on a
// successful fetch.
type Status struct {
	LastSync           string `json:"last_sync"`
	LastError          string `json:"last_error"`
	LastFetchError     string `json:"last_fetch_error,omitempty"`
	CurrentIssuedAt    string `json:"current_issued_at"`
	CurrentServerCount int    `json:"current_server_count"`
	LastIdentityChange string `json:"last_identity_change"`
}

// ReadStatus returns the current status. Returns a zero-value Status
// (not an error) when status.json is missing — fresh installs have
// no status yet.
func ReadStatus() (Status, error) {
	data, err := os.ReadFile(Path(StatusFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Status{}, nil
		}
		return Status{}, err
	}
	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		return Status{}, fmt.Errorf("state: parse status.json: %w", err)
	}
	return s, nil
}

// WriteStatus updates status.json atomically. Mode 0644 — must be
// world-readable for the user-space menu-bar app and `bb-vpn status`.
func WriteStatus(s Status) error {
	data, err := json.MarshalIndent(&s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return WriteAtomic(Path(StatusFile), data, 0o644)
}
