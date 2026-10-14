package render

import (
	"encoding/json"
	"errors"
	"fmt"

	"bb-dpi/client/bb-vpn/pkg/bundle"
)

func renderSingBox(b *bundle.Bundle, env Env, proto string) ([]byte, error) {
	// Step 1: envsubst on the raw skeleton bytes. Mirrors the bash
	// flow exactly (envsubst on the skeleton string, before any
	// JSON parsing). Default values for Flow / Fingerprint match
	// the `--arg flow` / `--arg fingerprint` defaults in render_singbox
	// ("Step 2: jq") of scripts/render-config.
	flow := env.Flow
	if flow == "" {
		flow = "xtls-rprx-vision"
	}
	fingerprint := env.Fingerprint
	if fingerprint == "" {
		fingerprint = "chrome"
	}
	// envsubst is performed on the raw skeleton string before JSON
	// parsing — mirrors the bash flow byte-for-byte. Parity note:
	// neither bash nor this code JSON-escapes env values, so operator
	// must ensure substituted values are JSON-safe (no `"`, `\`, or
	// newlines). All current envsubst targets are paths / UUIDs /
	// hostnames where this constraint is naturally satisfied.
	subbed := envsubst(string(b.Skeletons.SingBox), map[string]string{
		"HOME":           env.HOME,
		"UUID":           env.UUID,
		"INTERNAL_DNS_1": env.InternalDNS1,
		"COMPANY_DOMAIN": env.CompanyDomain,
	})

	// Step 2: parse into a generic tree so we can manipulate.
	var tree map[string]any
	if err := json.Unmarshal([]byte(subbed), &tree); err != nil {
		return nil, fmt.Errorf("parse skeleton after envsubst: %w", err)
	}

	// Step 3: inbounds[0].route_exclude_address = [host/32 for host in servers]
	if err := setRouteExcludeAddress(tree, b.Servers); err != nil {
		return nil, err
	}

	// Step 4: rewrite .outbounds — replace auto-urltest.outbounds with
	// the proto-specific tag list, then prepend the per-server outbounds,
	// then preserve any other outbounds (e.g., direct).
	if err := rebuildSingBoxOutbounds(tree, env.UUID, flow, fingerprint, proto, b.Servers); err != nil {
		return nil, err
	}

	// Step 5: normalize legacy skeletons. Bundles published before the
	// embedded-Tailscale mode was removed still carry its endpoint,
	// magicdns server and tailscale-bound route rules; such a bundle can
	// still be rendered (rollback to previous.json, cached fallback, a
	// client upgraded before the new bundle is published). The mode no
	// longer exists, so strip it unconditionally — a no-op on current
	// skeletons, which keeps parity with scripts/render-config.
	stripLegacyTailscale(tree)

	// Step 6: corp-dns stripping (when !WithCorpDNS). The skeleton
	// already detours company-dns via the VLESS chain (auto).
	if !b.Render.WithCorpDNS {
		stripCorpDNS(tree)
	}

	return marshalCanonical(tree)
}

// stripLegacyTailscale removes the embedded-Tailscale pieces a pre-removal
// skeleton carries: the endpoints block, the magicdns server and its rules,
// route rules preferring tailscale, and rewrites any remaining
// tailscale-bound outbound or DNS detour to the VLESS chain (auto).
func stripLegacyTailscale(tree map[string]any) {
	delete(tree, "endpoints")

	if dns, ok := tree["dns"].(map[string]any); ok {
		dns["servers"] = filterObjects(dns["servers"], func(m map[string]any) bool {
			return m["tag"] != "magicdns"
		})
		dns["rules"] = filterObjects(dns["rules"], func(m map[string]any) bool {
			return m["server"] != "magicdns"
		})
		if servers, ok := dns["servers"].([]any); ok {
			for _, s := range servers {
				if sm, ok := s.(map[string]any); ok && sm["detour"] == "tailscale" {
					sm["detour"] = "auto"
				}
			}
		}
	}

	if route, ok := tree["route"].(map[string]any); ok {
		route["rules"] = filterObjects(route["rules"], func(m map[string]any) bool {
			pb, _ := m["preferred_by"].([]any)
			for _, p := range pb {
				if p == "tailscale" {
					return false
				}
			}
			return true
		})
		if rules, ok := route["rules"].([]any); ok {
			for _, r := range rules {
				if rm, ok := r.(map[string]any); ok && rm["outbound"] == "tailscale" {
					rm["outbound"] = "auto"
				}
			}
		}
	}
}

// filterObjects keeps the elements of a JSON array for which keep returns
// true; non-object elements are kept. A missing or non-array value is
// returned unchanged.
func filterObjects(v any, keep func(map[string]any) bool) any {
	arr, ok := v.([]any)
	if !ok {
		return v
	}
	out := make([]any, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok && !keep(m) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func setRouteExcludeAddress(tree map[string]any, servers []bundle.Server) error {
	inbounds, ok := tree["inbounds"].([]any)
	if !ok || len(inbounds) == 0 {
		return errors.New("skeleton missing inbounds[0]")
	}
	inb0, ok := inbounds[0].(map[string]any)
	if !ok {
		return errors.New("skeleton inbounds[0] is not an object")
	}
	excludes := make([]any, 0, len(servers))
	for _, s := range servers {
		excludes = append(excludes, s.Host+"/32")
	}
	inb0["route_exclude_address"] = excludes
	return nil
}

func rebuildSingBoxOutbounds(tree map[string]any, uuid, flow, fingerprint, proto string, servers []bundle.Server) error {
	outbounds, ok := tree["outbounds"].([]any)
	if !ok {
		return errors.New("skeleton missing outbounds")
	}

	// Find the "auto" urltest outbound and the rest, preserving order.
	var autoOutbound map[string]any
	var others []any
	for _, ob := range outbounds {
		om, ok := ob.(map[string]any)
		if !ok {
			others = append(others, ob)
			continue
		}
		if tag, _ := om["tag"].(string); tag == "auto" {
			autoOutbound = om
		} else {
			others = append(others, ob)
		}
	}
	if autoOutbound == nil {
		return errors.New("skeleton has no outbound with tag=auto")
	}

	// urltest_tags depends on proto.
	tags := buildURLTestTags(proto, servers)
	autoOutbound["outbounds"] = tags

	// Build per-server outbounds.
	serverOutbounds := buildServerOutbounds(proto, servers, uuid, flow, fingerprint)

	// New outbounds: [autoOutbound] + serverOutbounds + [others...]
	// Mirrors the `.outbounds = [auto] + server_outbounds + [others]`
	// assignment in render_singbox ("Step 2: jq") of scripts/render-config.
	combined := make([]any, 0, 1+len(serverOutbounds)+len(others))
	combined = append(combined, autoOutbound)
	combined = append(combined, serverOutbounds...)
	combined = append(combined, others...)
	tree["outbounds"] = combined
	return nil
}

func buildURLTestTags(proto string, servers []bundle.Server) []any {
	tags := make([]any, 0, 2*len(servers))
	for _, s := range servers {
		switch proto {
		case "tcp-vision":
			tags = append(tags, "tcp-"+s.Name)
		case "xhttp":
			tags = append(tags, "xhttp-"+s.Name)
		default: // "all"
			tags = append(tags, "xhttp-"+s.Name, "tcp-"+s.Name)
		}
	}
	return tags
}

func buildServerOutbounds(proto string, servers []bundle.Server, uuid, flow, fingerprint string) []any {
	out := make([]any, 0, 2*len(servers))
	for i, s := range servers {
		switch proto {
		case "tcp-vision":
			out = append(out, tcpOutbound(s, uuid, flow, fingerprint))
		case "xhttp":
			out = append(out, xhttpOutbound(i, s))
		default: // "all"
			out = append(out, xhttpOutbound(i, s), tcpOutbound(s, uuid, flow, fingerprint))
		}
	}
	return out
}

func xhttpOutbound(i int, s bundle.Server) map[string]any {
	return map[string]any{
		"type":            "socks",
		"tag":             "xhttp-" + s.Name,
		"server":          "127.0.0.1",
		"server_port":     float64(1080 + i),
		"connect_timeout": "10s",
	}
}

func tcpOutbound(s bundle.Server, uuid, flow, fingerprint string) map[string]any {
	return map[string]any{
		"type":            "vless",
		"tag":             "tcp-" + s.Name,
		"server":          s.Host,
		"server_port":     float64(8443),
		"uuid":            uuid,
		"flow":            flow,
		"connect_timeout": "10s",
		"tcp_fast_open":   false,
		"tls": map[string]any{
			"enabled":     true,
			"server_name": s.SNI,
			"utls": map[string]any{
				"enabled":     true,
				"fingerprint": fingerprint,
			},
			"reality": map[string]any{
				"enabled":    true,
				"public_key": s.PublicKey,
				"short_id":   s.ShortID,
			},
		},
	}
}

func stripCorpDNS(tree map[string]any) {
	dns, ok := tree["dns"].(map[string]any)
	if !ok {
		return
	}
	if servers, ok := dns["servers"].([]any); ok {
		filtered := make([]any, 0, len(servers))
		for _, s := range servers {
			sm, ok := s.(map[string]any)
			if !ok {
				filtered = append(filtered, s)
				continue
			}
			if sm["tag"] == "company-dns" {
				continue
			}
			filtered = append(filtered, s)
		}
		dns["servers"] = filtered
	}
	if rules, ok := dns["rules"].([]any); ok {
		filtered := make([]any, 0, len(rules))
		for _, r := range rules {
			rm, ok := r.(map[string]any)
			if !ok {
				filtered = append(filtered, r)
				continue
			}
			if rm["server"] == "company-dns" {
				continue
			}
			filtered = append(filtered, r)
		}
		dns["rules"] = filtered
	}
}
