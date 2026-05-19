// Package cphttp fetches /control/bundle.json from the bb-dpi
// control-plane endpoints with sequential failover. Reads the
// endpoint list + bearer token from the .pkg-shipped
// /Library/Application Support/bb-dpi/control-plane.json.
package cphttp

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	// perEndpointTimeout matches the plan's "5s timeout per endpoint"
	// budget. With 2 baked-in endpoints this caps a worst-case fetch
	// at 10s before the sync loop declares "no control plane reachable".
	perEndpointTimeout = 5 * time.Second
	maxBundleBytes     = 4 << 20 // 4 MiB sanity cap on bundle response
)

// Endpoint mirrors one entry of control-plane.json. Field names match
// the operator's config/control-plane/endpoints.json schema so the
// .pkg build can scp the file verbatim.
//
// TODO(phase-4): HostIP + SNI are reserved for DNS-poisoning resilience
// — when set, fetchOne will dial HostIP directly and present SNI in the
// TLS ClientHello, bypassing the system resolver. Not yet wired (we
// rely on DNS-over-HTTPS at the system layer for now); leave the fields
// in so .pkg-shipped control-plane.json files don't need a schema bump
// when the feature lands.
type Endpoint struct {
	Label       string `json:"label"`
	URL         string `json:"url"`
	HostIP      string `json:"host_ip,omitempty"`
	SNI         string `json:"sni,omitempty"`
	Placeholder bool   `json:"placeholder,omitempty"`
}

// Config is the wire shape of control-plane.json.
type Config struct {
	Endpoints []Endpoint `json:"endpoints"`
	Token     string     `json:"token"`
}

// LoadConfig reads control-plane.json from the given path. The path
// is parameterized so the sync loop can pull it from state.Path() and
// tests can use a tempdir.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cphttp: read %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("cphttp: parse %s: %w", path, err)
	}
	if c.Token == "" {
		return nil, errors.New("cphttp: control-plane.json: token missing")
	}
	if len(c.Endpoints) == 0 {
		return nil, errors.New("cphttp: control-plane.json: endpoints empty")
	}
	// Reject any endpoint URL that doesn't use HTTPS. fetchOne attaches
	// the bearer token to every request — a misconfigured or tampered
	// control-plane.json with an http:// (or other) scheme would leak
	// the token in cleartext to whatever upstream is listening. Validate
	// at load time so the daemon refuses to start with a bad config
	// rather than discovering the leak at request time.
	for i, ep := range c.Endpoints {
		if ep.Placeholder {
			// Placeholder entries are skipped at fetch time and exist
			// only so the .pkg-shipped control-plane.json keeps a
			// stable shape until the real endpoint is provisioned.
			continue
		}
		if ep.URL == "" {
			return nil, fmt.Errorf("cphttp: control-plane.json: endpoint %d (%q): empty url", i, ep.Label)
		}
		u, perr := url.Parse(ep.URL)
		if perr != nil {
			return nil, fmt.Errorf("cphttp: control-plane.json: endpoint %d (%q): parse url: %w", i, ep.Label, perr)
		}
		if u.Scheme != "https" {
			return nil, fmt.Errorf("cphttp: control-plane.json: endpoint %d (%q): scheme %q is not https (refusing to send bearer token in cleartext)", i, ep.Label, u.Scheme)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("cphttp: control-plane.json: endpoint %d (%q): missing host", i, ep.Label)
		}
	}
	return &c, nil
}

// Fetch tries every non-placeholder endpoint in order, returning the
// first successful response body. An endpoint failure (DNS, TLS,
// timeout, 4xx, 5xx, parse-fail) advances to the next. Returns the
// last error when every endpoint fails.
//
// The bundle body itself is NOT parsed here — pkg/bundle owns parse
// + structural validation. cphttp just gates on HTTP-level success
// + size cap.
func Fetch(cfg *Config) ([]byte, Endpoint, error) {
	if cfg == nil {
		return nil, Endpoint{}, errors.New("cphttp: nil config")
	}
	var lastErr error
	for _, ep := range cfg.Endpoints {
		if ep.Placeholder {
			continue
		}
		body, err := fetchOne(ep, cfg.Token)
		if err == nil {
			return body, ep, nil
		}
		lastErr = fmt.Errorf("cphttp: %s: %w", ep.Label, err)
	}
	if lastErr == nil {
		return nil, Endpoint{}, errors.New("cphttp: no non-placeholder endpoints")
	}
	return nil, Endpoint{}, lastErr
}

func fetchOne(ep Endpoint, token string) ([]byte, error) {
	client := &http.Client{
		Timeout: perEndpointTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				// REALITY mimics the cover SNI's real cert; system
				// CA roots validate fine — no InsecureSkipVerify.
			},
		},
	}
	req, err := http.NewRequest(http.MethodGet, ep.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBundleBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBundleBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxBundleBytes)
	}
	return body, nil
}
