package render

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"bb-dpi/client/bb-vpn/pkg/bundle"
)

// A bundle published before the embedded-Tailscale mode was removed must
// still render to a config with no Tailscale endpoint and with corp traffic
// on the VLESS chain: such bundles are rendered on rollback to previous.json,
// on the cached-bundle fallback, and on clients upgraded before the new
// bundle is published.
func TestRenderSingBox_LegacySkeletonStripsTailscale(t *testing.T) {
	skel, err := os.ReadFile("testdata/legacy-sing-box-skeleton.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, corp := range []bool{false, true} {
		b := &bundle.Bundle{
			SchemaVersion: 1,
			Servers: []bundle.Server{{
				Name: "s1", Host: "192.0.2.10", SNI: "example.com",
				PublicKey: "pk", ShortID: "ab",
			}},
			Skeletons: bundle.Skeletons{SingBox: skel},
			Render: bundle.Render{
				Proto:         "tcp-vision",
				WithCorpDNS:   corp,
				WithTailscale: false,
			},
		}
		env := Env{
			HOME: "/home/test", UUID: "00000000-0000-0000-0000-000000000000",
			InternalDNS1: "10.0.0.53", CompanyDomain: "corp.example",
		}
		out, _, err := Render(b, env)
		if err != nil {
			t.Fatalf("corp=%v: render: %v", corp, err)
		}
		s := string(out)
		for _, banned := range []string{`"endpoints"`, `"magicdns"`, `"tailscale"`, "TAILSCALE_AUTH_KEY"} {
			if strings.Contains(s, banned) {
				t.Errorf("corp=%v: rendered legacy config still contains %s", corp, banned)
			}
		}

		var tree map[string]any
		if err := json.Unmarshal(out, &tree); err != nil {
			t.Fatal(err)
		}
		var corpRouteOutbound any
		for _, r := range tree["route"].(map[string]any)["rules"].([]any) {
			rm := r.(map[string]any)
			if cidrs, ok := rm["ip_cidr"].([]any); ok && len(cidrs) > 0 && cidrs[0] == "10.0.0.0/8" {
				corpRouteOutbound = rm["outbound"]
			}
		}
		if corpRouteOutbound != "auto" {
			t.Errorf("corp=%v: corp subnet route outbound = %v, want auto", corp, corpRouteOutbound)
		}

		var companyDetour any
		for _, sv := range tree["dns"].(map[string]any)["servers"].([]any) {
			sm := sv.(map[string]any)
			if sm["tag"] == "company-dns" {
				companyDetour = sm["detour"]
			}
		}
		if corp && companyDetour != "auto" {
			t.Errorf("company-dns detour = %v, want auto", companyDetour)
		}
		if !corp && companyDetour != nil {
			t.Errorf("company-dns present without corp DNS (detour %v)", companyDetour)
		}
	}
}
