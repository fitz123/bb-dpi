// Package bundle defines the control-plane bundle.json schema and
// structural validation for it.
//
// Compatibility check (bundle.min_versions vs. the local binary
// versions) lives in pkg/launchctl's sync algorithm step 1; this
// package owns parse + structural shape only. The version comparator
// helpers used by that check live here (SemverGE, CalverGE) because
// the version-string format is part of the bundle contract.
package bundle

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// SupportedSchemaVersion is the highest schema_version this binary
// understands. Bundles with a higher schema_version are rejected.
const SupportedSchemaVersion = 1

// Bundle is the wire shape of /control/bundle.json.
//
// Skeletons are kept as raw JSON: pkg/render is the only consumer and
// it manipulates them as opaque JSON trees. Decoding them here would
// require duplicating the skeleton schema in Go for no gain.
type Bundle struct {
	SchemaVersion int         `json:"schema_version"`
	IssuedAt      string      `json:"issued_at"`
	MinVersions   MinVersions `json:"min_versions"`
	Servers       []Server    `json:"servers"`
	Skeletons     Skeletons   `json:"skeletons"`
	Render        Render      `json:"render"`
}

// MinVersions is the floor of local binary versions a client must
// have to accept this bundle. publish-bundle populates these from the
// committed config/control-plane/package-manifest.json.
type MinVersions struct {
	BBVPN   string `json:"bb_vpn"`
	SingBox string `json:"sing_box"`
	Xray    string `json:"xray"`
}

// Server is the allowlist-projected fields that publish-bundle emits.
// Anything outside this set is a leak; scripts/test-publish-bundle
// guards against it on the publish side.
type Server struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	PublicKey string `json:"public_key"`
	ShortID   string `json:"short_id"`
	XHTTPPath string `json:"xhttp_path,omitempty"`
	XHTTPSNI  string `json:"xhttp_sni,omitempty"`
	SNI       string `json:"sni,omitempty"`
}

// Skeletons holds the raw JSON of the sing-box and xray-xhttp config
// skeletons that pkg/render uses as templates.
type Skeletons struct {
	SingBox   json.RawMessage `json:"sing_box"`
	XrayXHTTP json.RawMessage `json:"xray_xhttp"`
}

// Render is the fleet-wide render flags published with each bundle.
//
// InternalDNS1 + CompanyDomain are optional carry-through values for
// the with_corp_dns rendering branch. They are NOT structurally
// validated here: pkg/render.Render() is the single check point for
// "WithCorpDNS requires non-empty values", and it runs AFTER
// buildSyncEnv merges in the legacy env-var fallback
// (BB_VPN_INTERNAL_DNS_1 / BB_VPN_COMPANY_DOMAIN). Adding a check
// here would short-circuit the fallback and break legacy bundles.
type Render struct {
	Proto         string `json:"proto"`
	WithCorpDNS   bool   `json:"with_corp_dns"`
	WithTailscale bool   `json:"with_tailscale"`
	InternalDNS1  string `json:"internal_dns_1,omitempty"`
	CompanyDomain string `json:"company_domain,omitempty"`
}

// Parse decodes raw bundle JSON. It does NOT validate semantics —
// call Validate() for that. The split matters because some call
// sites (e.g. forensic dumps) want to inspect malformed bundles.
func Parse(data []byte) (*Bundle, error) {
	var b Bundle
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields() // be strict at parse time; surface schema drift loudly
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("bundle: parse: %w", err)
	}
	// Reject trailing non-whitespace after the bundle object — a
	// concatenation like `<bundle><stale-bundle>` or `<bundle><html>`
	// from a misconfigured cover-site upstream should fail loudly,
	// not silently take the first value.
	if dec.More() {
		return nil, fmt.Errorf("bundle: parse: unexpected trailing data after bundle object")
	}
	return &b, nil
}

// Validate enforces structural invariants:
//   - schema_version is in [1, SupportedSchemaVersion]
//   - min_versions.{bb_vpn, sing_box, xray} are all non-empty
//   - servers is non-empty
//   - every server has non-empty {host, public_key, short_id}
//   - skeletons.{sing_box, xray_xhttp} are present (non-null JSON)
//   - render.proto is one of {"all", "tcp-vision", "xhttp"}
//
// Semantic compatibility (min_versions vs. local binaries) is
// pkg/launchctl's responsibility, not this package's.
func (b *Bundle) Validate() error {
	if b.SchemaVersion < 1 || b.SchemaVersion > SupportedSchemaVersion {
		return fmt.Errorf("bundle: schema_version %d out of range [1, %d]",
			b.SchemaVersion, SupportedSchemaVersion)
	}
	if b.MinVersions.BBVPN == "" || b.MinVersions.SingBox == "" || b.MinVersions.Xray == "" {
		return errors.New("bundle: min_versions: bb_vpn, sing_box, xray all required and non-empty")
	}
	if len(b.Servers) == 0 {
		return errors.New("bundle: servers: must be non-empty")
	}
	for i, s := range b.Servers {
		if s.Host == "" || s.PublicKey == "" || s.ShortID == "" {
			return fmt.Errorf("bundle: servers[%d] (%q): host, public_key, short_id all required",
				i, s.Name)
		}
	}
	if err := validateSkeleton("sing_box", b.Skeletons.SingBox); err != nil {
		return err
	}
	if err := validateSkeleton("xray_xhttp", b.Skeletons.XrayXHTTP); err != nil {
		return err
	}
	switch b.Render.Proto {
	case "all", "tcp-vision", "xhttp":
	default:
		return fmt.Errorf("bundle: render.proto %q: must be all|tcp-vision|xhttp", b.Render.Proto)
	}
	return nil
}

// validateSkeleton rejects missing, JSON null, or non-object skeleton
// payloads. publish-bundle pipes the skeleton files through `jq`, so
// `null` would only appear if someone hand-edited bundle.json or the
// skeleton file got truncated — but a downstream pkg/render panic on
// `null` traces back to here, so fail closed at the parse boundary.
func validateSkeleton(name string, raw []byte) error {
	if len(raw) == 0 {
		return fmt.Errorf("bundle: skeletons.%s: required", name)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return fmt.Errorf("bundle: skeletons.%s: required (empty or whitespace only)", name)
	}
	// json.RawMessage of literal `null` arrives as the 4 bytes "null".
	if bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("bundle: skeletons.%s: must not be null", name)
	}
	// First non-whitespace byte must be `{` — skeletons are JSON objects,
	// never arrays or scalars. Catches `""`, `42`, `[]` etc.
	if trimmed[0] != '{' {
		return fmt.Errorf("bundle: skeletons.%s: must be a JSON object", name)
	}
	return nil
}
