package cphttp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig drops a control-plane.json into a tempdir and returns
// the path. Kept inline to avoid touching cphttp's exported surface
// just for tests.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "control-plane.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

// TestLoadConfig_HTTPSPasses is the happy path. A well-formed
// control-plane.json with an https endpoint URL must load cleanly so
// the daemon can fetch its bundle.
func TestLoadConfig_HTTPSPasses(t *testing.T) {
	p := writeConfig(t, `{
		"endpoints": [
			{"label": "primary", "url": "https://cp.example.com/control/bundle.json"}
		],
		"token": "test-token"
	}`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: unexpected error: %v", err)
	}
	if len(cfg.Endpoints) != 1 || cfg.Endpoints[0].URL != "https://cp.example.com/control/bundle.json" {
		t.Errorf("endpoint not preserved: %+v", cfg.Endpoints)
	}
	if cfg.Token != "test-token" {
		t.Errorf("token = %q, want test-token", cfg.Token)
	}
}

// TestLoadConfig_HTTPRejected locks in the must-fix: an http:// URL
// would leak the bearer token in cleartext. LoadConfig must refuse
// to return a Config for such a file so the daemon never reaches
// fetchOne with a plaintext endpoint.
func TestLoadConfig_HTTPRejected(t *testing.T) {
	p := writeConfig(t, `{
		"endpoints": [
			{"label": "primary", "url": "http://cp.example.com/control/bundle.json"}
		],
		"token": "test-token"
	}`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("LoadConfig: expected error on http:// endpoint, got nil")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Errorf("LoadConfig error = %v, want mention of https", err)
	}
}

// TestLoadConfig_NonHTTPSchemesRejected covers other plaintext or
// inappropriate schemes a tampered config could carry — file://,
// ftp://, gopher://, etc. The guard is "must be https", not "must not
// be http".
func TestLoadConfig_NonHTTPSchemesRejected(t *testing.T) {
	for _, scheme := range []string{"file", "ftp", "ws", "data"} {
		body := `{
			"endpoints": [{"label": "primary", "url": "` + scheme + `://cp.example.com/x"}],
			"token": "t"
		}`
		p := writeConfig(t, body)
		if _, err := LoadConfig(p); err == nil {
			t.Errorf("LoadConfig: expected error on scheme %q, got nil", scheme)
		}
	}
}

// TestLoadConfig_MalformedURLRejected covers a tampered or hand-edited
// config with garbage in the URL field. net/url.Parse is fairly
// permissive so this exercises the post-parse scheme check on a
// realistic broken value.
func TestLoadConfig_MalformedURLRejected(t *testing.T) {
	p := writeConfig(t, `{
		"endpoints": [
			{"label": "primary", "url": "://no-scheme"}
		],
		"token": "t"
	}`)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("LoadConfig: expected error on malformed url, got nil")
	}
}

// TestLoadConfig_EmptyURLRejected covers the case where the URL
// field is present but blank — a common shape for a half-provisioned
// config file. The fetch path would treat this as "advance to next
// endpoint" only if we let it through; better to fail-closed.
func TestLoadConfig_EmptyURLRejected(t *testing.T) {
	p := writeConfig(t, `{
		"endpoints": [
			{"label": "primary", "url": ""}
		],
		"token": "t"
	}`)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("LoadConfig: expected error on empty url, got nil")
	}
}

// TestLoadConfig_PlaceholderSkippedFromSchemeCheck locks in that
// placeholder=true entries bypass the scheme guard. The .pkg-shipped
// control-plane.json keeps placeholder rows around until the real
// endpoint is provisioned; rejecting them on scheme would break that
// bootstrap flow.
func TestLoadConfig_PlaceholderSkippedFromSchemeCheck(t *testing.T) {
	p := writeConfig(t, `{
		"endpoints": [
			{"label": "ph", "url": "http://placeholder.invalid/", "placeholder": true},
			{"label": "real", "url": "https://cp.example.com/control/bundle.json"}
		],
		"token": "t"
	}`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: unexpected error: %v", err)
	}
	if len(cfg.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(cfg.Endpoints))
	}
}
