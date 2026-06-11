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
	URLTest     string `json:"url_test,omitempty"`
	HostIP      string `json:"host_ip,omitempty"`
	SNI         string `json:"sni,omitempty"`
	Placeholder bool   `json:"placeholder,omitempty"`
}

// Target selects which published bundle snapshot Fetch retrieves: the
// stable prod path (Endpoint.URL) or the staging test path
// (Endpoint.URLTest). Any value other than TargetTest behaves as prod
// — the safe default.
//
// NOTE: pkg/state declares its own Target type with the same
// "prod"/"test" literals. The duplication is deliberate — it keeps
// cphttp (token-bearing, security-sensitive) free of a state
// dependency. Keep the literals in sync with pkg/state/target.go;
// sync converts at the call site.
type Target string

const (
	TargetProd Target = "prod"
	TargetTest Target = "test"
)

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
		if err := validateEndpointURL(i, ep.Label, "url", ep.URL); err != nil {
			return nil, err
		}
		// url_test is optional (only endpoints that serve a staging
		// bundle carry it), but when present it gets the same
		// cleartext-token guard as url.
		if ep.URLTest != "" {
			if err := validateEndpointURL(i, ep.Label, "url_test", ep.URLTest); err != nil {
				return nil, err
			}
		}
	}
	return &c, nil
}

func validateEndpointURL(i int, label, field, rawURL string) error {
	u, perr := url.Parse(rawURL)
	if perr != nil {
		return fmt.Errorf("cphttp: control-plane.json: endpoint %d (%q): parse %s: %w", i, label, field, perr)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("cphttp: control-plane.json: endpoint %d (%q): %s scheme %q is not https (refusing to send bearer token in cleartext)", i, label, field, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("cphttp: control-plane.json: endpoint %d (%q): %s missing host", i, label, field)
	}
	return nil
}

// Fetch tries every non-placeholder endpoint in order, returning the
// first successful response body. An endpoint failure (DNS, TLS,
// timeout, 4xx, 5xx, parse-fail) advances to the next. Returns the
// last error when every endpoint fails.
//
// target picks the per-endpoint URL: TargetTest uses ep.URLTest and
// skips endpoints that don't carry one; anything else uses ep.URL
// (prod behavior, unchanged from before targets existed).
//
// The bundle body itself is NOT parsed here — pkg/bundle owns parse
// + structural validation. cphttp just gates on HTTP-level success
// + size cap.
func Fetch(cfg *Config, target Target) ([]byte, Endpoint, error) {
	if cfg == nil {
		return nil, Endpoint{}, errors.New("cphttp: nil config")
	}
	var lastErr error
	for _, ep := range cfg.Endpoints {
		if ep.Placeholder {
			continue
		}
		epURL := ep.URL
		if target == TargetTest {
			epURL = ep.URLTest
		}
		if epURL == "" {
			// url_test is optional — an endpoint without one simply
			// doesn't serve this target; advance to the next.
			continue
		}
		body, err := fetchOne(epURL, cfg.Token)
		if err == nil {
			return body, ep, nil
		}
		lastErr = fmt.Errorf("cphttp: %s: %w", ep.Label, err)
	}
	if lastErr == nil {
		return nil, Endpoint{}, fmt.Errorf("cphttp: no usable endpoint for target %q", target)
	}
	return nil, Endpoint{}, lastErr
}

func fetchOne(rawURL, token string) ([]byte, error) {
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
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
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
