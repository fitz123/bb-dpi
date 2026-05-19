// Package render is the Go port of scripts/render-config: turns a
// bundle (servers list + skeletons + render flags) into rendered
// sing-box and xray-xhttp client configs.
//
// Byte-equality contract with the bash flow is preserved by:
//
//   - reading skeletons from bundle.Skeletons (never from disk or
//     go:embed — the whole point of pull-based control plane is that
//     skeleton edits propagate via bundle);
//   - executing exactly the same envsubst + JSON-tree transforms the
//     bash script does;
//   - emitting via stdlib encoding/json with sorted keys + 2-space
//     indent + trailing newline + HTML escaping OFF, which matches
//     jq -S --indent 2's output verbatim.
//
// The golden corpus under internal/tests/golden/expected/ is generated
// by piping bash render-config's output through `jq -S --indent 2`, so
// the parity test can simply byte-compare against Render's output.
package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"bb-dpi/client/bb-vpn/pkg/bundle"
)

// Env is the operator-supplied environment variables that the bash
// script's envsubst step interpolates into the sing-box skeleton.
// Field set is a literal mirror of the envsubst whitelist on
// scripts/render-config:180 — only these names get substituted.
type Env struct {
	HOME              string
	UUID              string
	TailscaleAuthKey  string
	TailscaleHostname string
	InternalDNS1      string
	CompanyDomain     string
	// Flow, Fingerprint are passed to the jq script as --arg values
	// (not envsubst targets). They have bash defaults
	// "xtls-rprx-vision" / "chrome" if empty.
	Flow        string
	Fingerprint string
}

// Render produces canonical sing-box + xray-xhttp JSON for the given
// bundle and env. The bundle's Render.Proto selects which transports
// get emitted; b.Render.Proto must be one of "all", "tcp-vision", or
// "xhttp". env.UUID must be non-empty and b.Servers must be non-empty.
//
// xrayJSON is empty when b.Render.Proto == "tcp-vision" — the bash
// script skips the xray render in that case, and the parity contract
// is that pkg/render must match that "no xray output" behavior.
func Render(b *bundle.Bundle, env Env) (singBoxJSON, xrayJSON []byte, err error) {
	if b == nil {
		return nil, nil, errors.New("render: nil bundle")
	}
	if env.UUID == "" {
		return nil, nil, errors.New("render: UUID required")
	}
	proto := b.Render.Proto
	if proto != "all" && proto != "tcp-vision" && proto != "xhttp" {
		return nil, nil, fmt.Errorf("render: bundle.render.proto %q: must be all|tcp-vision|xhttp", proto)
	}

	if len(b.Servers) == 0 {
		return nil, nil, errors.New("render: bundle.servers must be non-empty")
	}

	// Mirrors bash render-config:166-173: WithCorpDNS without
	// INTERNAL_DNS_1 + COMPANY_DOMAIN produces an empty-server /
	// empty-domain-suffix DNS config that sing-box check might
	// accept but won't resolve. Hard-fail rather than ship a silent
	// dud.
	if b.Render.WithCorpDNS {
		if env.InternalDNS1 == "" {
			return nil, nil, errors.New("render: WithCorpDNS=true requires env.InternalDNS1")
		}
		if env.CompanyDomain == "" {
			return nil, nil, errors.New("render: WithCorpDNS=true requires env.CompanyDomain")
		}
	}

	singBoxJSON, err = renderSingBox(b, env, proto)
	if err != nil {
		return nil, nil, fmt.Errorf("render: sing-box: %w", err)
	}
	if proto != "tcp-vision" {
		xrayJSON, err = renderXray(b, env)
		if err != nil {
			return nil, nil, fmt.Errorf("render: xray: %w", err)
		}
	}
	return singBoxJSON, xrayJSON, nil
}

// marshalCanonical writes v as JSON matching `jq -S --indent 2`:
//   - 2-space indent
//   - sorted keys (stdlib's default for map[string]any)
//   - HTML escaping disabled (jq does not escape '<', '>', '&')
//   - trailing newline
func marshalCanonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder.Encode already appends "\n" — matches jq's
	// trailing-newline behavior.
	return buf.Bytes(), nil
}
