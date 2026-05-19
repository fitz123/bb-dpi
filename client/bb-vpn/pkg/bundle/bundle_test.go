package bundle

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// validBundle returns a structurally valid bundle, suitable as a
// starting point that individual test cases mutate.
func validBundle() *Bundle {
	return &Bundle{
		SchemaVersion: 1,
		IssuedAt:      "2026-05-19T08:24:04Z",
		MinVersions: MinVersions{
			BBVPN:   "1.0.0",
			SingBox: "1.13.0",
			Xray:    "25.12.8",
		},
		Servers: []Server{
			{
				Name:      "alpha",
				Host:      "alpha.example.invalid",
				PublicKey: "test-public-key-alpha-fake-not-base64",
				ShortID:   "0123456789abcdef",
				SNI:       "cdn.example.invalid",
				XHTTPPath: "/abc",
				XHTTPSNI:  "cdn.example.invalid",
			},
		},
		Skeletons: Skeletons{
			SingBox:   json.RawMessage(`{"log":{"level":"info"}}`),
			XrayXHTTP: json.RawMessage(`{"inbounds":[]}`),
		},
		Render: Render{
			Proto:         "all",
			WithCorpDNS:   false,
			WithTailscale: false,
		},
	}
}

// mustMarshal encodes a bundle to JSON or fails the test.
func mustMarshal(t *testing.T, b *Bundle) []byte {
	t.Helper()
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func TestParse_Valid(t *testing.T) {
	data := mustMarshal(t, validBundle())
	b, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if b.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", b.SchemaVersion)
	}
	if len(b.Servers) != 1 {
		t.Errorf("Servers len = %d, want 1", len(b.Servers))
	}
	if err := b.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestParse_RejectsUnknownFields(t *testing.T) {
	// A field publish-bundle's allowlist forbids must not parse, so
	// schema-drift surfaces loudly instead of silently dropping data.
	data := []byte(`{"schema_version":1,"servers":[],"surprise":"oops"}`)
	if _, err := Parse(data); err == nil {
		t.Fatalf("Parse: want error on unknown field, got nil")
	}
}

func TestParse_RejectsTrailingData(t *testing.T) {
	// A misconfigured cover-site upstream could splice a second JSON
	// document or HTML onto the bundle response. Parse must reject;
	// silent-take-first-value would mask the misconfiguration.
	good := mustMarshal(t, validBundle())
	cases := [][]byte{
		append(append([]byte{}, good...), []byte(`{"junk":1}`)...),
		append(append([]byte{}, good...), []byte(`<!DOCTYPE html>`)...),
		append(append([]byte{}, good...), []byte(`null`)...),
	}
	for i, data := range cases {
		if _, err := Parse(data); err == nil {
			t.Errorf("case %d: want error on trailing data, got nil", i)
		}
	}
}

func TestValidate_RejectsBadSchemaVersion(t *testing.T) {
	cases := []int{0, -1, SupportedSchemaVersion + 1}
	for _, v := range cases {
		b := validBundle()
		b.SchemaVersion = v
		if err := b.Validate(); err == nil {
			t.Errorf("schema_version=%d: want error, got nil", v)
		}
	}
}

func TestValidate_RequiresMinVersions(t *testing.T) {
	cases := []func(*MinVersions){
		func(m *MinVersions) { m.BBVPN = "" },
		func(m *MinVersions) { m.SingBox = "" },
		func(m *MinVersions) { m.Xray = "" },
	}
	for i, mut := range cases {
		b := validBundle()
		mut(&b.MinVersions)
		if err := b.Validate(); err == nil {
			t.Errorf("case %d: want error on missing min_version, got nil", i)
		}
	}
}

func TestValidate_RequiresNonEmptyServers(t *testing.T) {
	b := validBundle()
	b.Servers = nil
	if err := b.Validate(); err == nil {
		t.Errorf("empty servers: want error, got nil")
	}
}

func TestValidate_RequiresServerFields(t *testing.T) {
	cases := []func(*Server){
		func(s *Server) { s.Host = "" },
		func(s *Server) { s.PublicKey = "" },
		func(s *Server) { s.ShortID = "" },
	}
	for i, mut := range cases {
		b := validBundle()
		mut(&b.Servers[0])
		if err := b.Validate(); err == nil {
			t.Errorf("case %d: want error on missing server field, got nil", i)
		}
	}
}

// TestValidate_IteratesAllServers guards against the loop iterating
// only servers[0]: the missing field is on servers[1].
func TestValidate_IteratesAllServers(t *testing.T) {
	b := validBundle()
	b.Servers = append(b.Servers, Server{
		Name: "beta", Host: "h", PublicKey: "pk", ShortID: "", // ShortID missing
	})
	err := b.Validate()
	if err == nil {
		t.Fatalf("want error on servers[1].ShortID empty, got nil")
	}
	if !strings.Contains(err.Error(), "servers[1]") {
		t.Errorf("error should reference servers[1], got: %v", err)
	}
}

// TestParseValidate_NullOptionalServerFields locks in the contract
// with publish-bundle's jq projection: `{name, host, ..., sni}` over
// a server lacking optionals emits explicit JSON null for each
// missing key. The omitempty tags must round-trip those nulls as
// empty strings without breaking Validate.
func TestParseValidate_NullOptionalServerFields(t *testing.T) {
	raw := []byte(`{
	  "schema_version": 1,
	  "issued_at": "2026-05-19T08:24:04Z",
	  "min_versions": {"bb_vpn":"1.0.0","sing_box":"1.13.0","xray":"25.12.8"},
	  "servers": [
	    {
	      "name": "alpha",
	      "host": "203.0.113.10",
	      "public_key": "test-public-key-alpha-fake-not-base64",
	      "short_id": "deadbeefcafebabe",
	      "xhttp_path": null,
	      "xhttp_sni": null,
	      "sni": null
	    }
	  ],
	  "skeletons": {
	    "sing_box": {"a":1},
	    "xray_xhttp": {"b":2}
	  },
	  "render": {"proto":"all","with_corp_dns":false,"with_tailscale":false}
	}`)
	b, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	s := b.Servers[0]
	if s.XHTTPPath != "" || s.XHTTPSNI != "" || s.SNI != "" {
		t.Errorf("null optionals should round-trip as empty string, got %+v", s)
	}
}

func TestValidate_RequiresSkeletons(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Skeletons)
	}{
		{"nil sing-box", func(s *Skeletons) { s.SingBox = nil }},
		{"nil xray", func(s *Skeletons) { s.XrayXHTTP = nil }},
		{"null sing-box", func(s *Skeletons) { s.SingBox = json.RawMessage(`null`) }},
		{"null xray", func(s *Skeletons) { s.XrayXHTTP = json.RawMessage(`null`) }},
		{"empty-string sing-box", func(s *Skeletons) { s.SingBox = json.RawMessage(`""`) }},
		{"array sing-box", func(s *Skeletons) { s.SingBox = json.RawMessage(`[]`) }},
		{"scalar sing-box", func(s *Skeletons) { s.SingBox = json.RawMessage(`42`) }},
		{"whitespace-only sing-box", func(s *Skeletons) { s.SingBox = json.RawMessage("   \n\t  ") }},
	}
	for _, c := range cases {
		b := validBundle()
		c.mut(&b.Skeletons)
		if err := b.Validate(); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

func TestValidate_RejectsBadProto(t *testing.T) {
	cases := []string{"", "tcp", "vless", "ALL", "TCP-VISION"}
	for _, p := range cases {
		b := validBundle()
		b.Render.Proto = p
		if err := b.Validate(); err == nil {
			t.Errorf("proto=%q: want error, got nil", p)
		}
	}
}

// TestValidate_AcceptsAllValidProtos guards against accidental
// allowlist tightening when someone touches the proto switch.
func TestValidate_AcceptsAllValidProtos(t *testing.T) {
	for _, p := range []string{"all", "tcp-vision", "xhttp"} {
		b := validBundle()
		b.Render.Proto = p
		if err := b.Validate(); err != nil {
			t.Errorf("proto=%q: unexpected error: %v", p, err)
		}
	}
}

// TestParseValidate_RealisticShape simulates the bundle shape that
// publish-bundle actually emits — JSON keys in arbitrary order,
// pretty-printed, with the full set of server fields the allowlist
// permits. Guards against tag drift between Go structs and the bash
// publish-bundle jq projection.
func TestParseValidate_RealisticShape(t *testing.T) {
	raw := strings.NewReader(`{
	  "schema_version": 1,
	  "issued_at": "2026-05-19T08:24:04Z",
	  "min_versions": {"bb_vpn":"1.0.0","sing_box":"1.13.0","xray":"25.12.8"},
	  "servers": [
	    {
	      "name": "alpha",
	      "host": "203.0.113.10",
	      "public_key": "test-public-key-alpha-fake-not-base64",
	      "short_id": "deadbeefcafebabe",
	      "xhttp_path": "/abc",
	      "xhttp_sni": "cdn.example.invalid",
	      "sni": "cdn.example.invalid"
	    },
	    {
	      "name": "beta",
	      "host": "203.0.113.20",
	      "public_key": "test-public-key-beta-fake-not-base64",
	      "short_id": "1234567890abcdef",
	      "xhttp_path": "/xyz",
	      "xhttp_sni": "edge.example.invalid",
	      "sni": "edge.example.invalid"
	    }
	  ],
	  "skeletons": {
	    "sing_box":   {"log":{"level":"info"},"inbounds":[],"outbounds":[]},
	    "xray_xhttp": {"log":{"loglevel":"warning"},"inbounds":[],"outbounds":[]}
	  },
	  "render": {"proto":"all","with_corp_dns":false,"with_tailscale":false}
	}`)
	buf, err := io.ReadAll(raw)
	if err != nil {
		t.Fatalf("readall: %v", err)
	}
	b, err := Parse(buf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(b.Servers) != 2 {
		t.Errorf("Servers len = %d, want 2", len(b.Servers))
	}
}

