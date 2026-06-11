package cphttp

import (
	"net/http"
	"net/http/httptest"
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

// TestLoadConfig_URLTestOptional locks in backward compatibility:
// a control-plane.json with no url_test anywhere (every file shipped
// before targets existed) must load exactly as before, and a present
// https url_test must be preserved.
func TestLoadConfig_URLTestOptional(t *testing.T) {
	p := writeConfig(t, `{
		"endpoints": [
			{"label": "old", "url": "https://cp.example.com/control/bundle.json"},
			{"label": "new", "url": "https://cp2.example.com/control/bundle.json",
			 "url_test": "https://cp2.example.com/control/test/bundle.json"}
		],
		"token": "t"
	}`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: unexpected error: %v", err)
	}
	if cfg.Endpoints[0].URLTest != "" {
		t.Errorf("endpoint 0 url_test = %q, want empty", cfg.Endpoints[0].URLTest)
	}
	if cfg.Endpoints[1].URLTest != "https://cp2.example.com/control/test/bundle.json" {
		t.Errorf("endpoint 1 url_test not preserved: %q", cfg.Endpoints[1].URLTest)
	}
}

// TestLoadConfig_HTTPURLTestRejected extends the cleartext-token guard
// to url_test: a tampered config could leave url https-clean but point
// url_test at http://, leaking the bearer token the moment the client
// flips to target=test.
func TestLoadConfig_HTTPURLTestRejected(t *testing.T) {
	p := writeConfig(t, `{
		"endpoints": [
			{"label": "primary", "url": "https://cp.example.com/control/bundle.json",
			 "url_test": "http://cp.example.com/control/test/bundle.json"}
		],
		"token": "t"
	}`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("LoadConfig: expected error on http:// url_test, got nil")
	}
	if !strings.Contains(err.Error(), "url_test") || !strings.Contains(err.Error(), "https") {
		t.Errorf("LoadConfig error = %v, want mention of url_test + https", err)
	}
}

// bundleServer spins up a test HTTP server that serves body on any
// path after checking the bearer token, and returns it. Fetch does
// not re-validate schemes (LoadConfig owns that), so plain-http
// httptest servers are fine for routing tests.
func bundleServer(t *testing.T, body, wantToken string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+wantToken {
			t.Errorf("Authorization = %q, want Bearer %s", got, wantToken)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestFetch_ProdUsesURL is the unchanged-behavior anchor: with both
// urls present, target=prod must hit url and never url_test.
func TestFetch_ProdUsesURL(t *testing.T) {
	prod := bundleServer(t, "prod-bytes", "tok")
	test := bundleServer(t, "test-bytes", "tok")
	cfg := &Config{
		Endpoints: []Endpoint{{Label: "ep", URL: prod.URL, URLTest: test.URL}},
		Token:     "tok",
	}
	body, ep, err := Fetch(cfg, TargetProd)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if string(body) != "prod-bytes" {
		t.Errorf("body = %q, want prod-bytes", body)
	}
	if ep.Label != "ep" {
		t.Errorf("endpoint label = %q, want ep", ep.Label)
	}
}

// TestFetch_TestUsesURLTest: target=test must hit url_test on an
// endpoint that carries one.
func TestFetch_TestUsesURLTest(t *testing.T) {
	prod := bundleServer(t, "prod-bytes", "tok")
	test := bundleServer(t, "test-bytes", "tok")
	cfg := &Config{
		Endpoints: []Endpoint{{Label: "ep", URL: prod.URL, URLTest: test.URL}},
		Token:     "tok",
	}
	body, _, err := Fetch(cfg, TargetTest)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if string(body) != "test-bytes" {
		t.Errorf("body = %q, want test-bytes", body)
	}
}

// TestFetch_TestSkipsEndpointWithoutURLTest: url_test is optional per
// endpoint — target=test must skip an endpoint that lacks it and fail
// over to the next one that has it, without ever touching the first
// endpoint's prod url.
func TestFetch_TestSkipsEndpointWithoutURLTest(t *testing.T) {
	var firstProdHit bool
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstProdHit = true
		w.Write([]byte("first-prod"))
	}))
	t.Cleanup(first.Close)
	second := bundleServer(t, "second-test", "tok")
	cfg := &Config{
		Endpoints: []Endpoint{
			{Label: "first", URL: first.URL}, // no url_test
			{Label: "second", URL: "https://unused.invalid/", URLTest: second.URL},
		},
		Token: "tok",
	}
	body, ep, err := Fetch(cfg, TargetTest)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if string(body) != "second-test" {
		t.Errorf("body = %q, want second-test", body)
	}
	if ep.Label != "second" {
		t.Errorf("endpoint label = %q, want second", ep.Label)
	}
	if firstProdHit {
		t.Error("target=test must not fall back to an endpoint's prod url")
	}
}

// TestFetch_TestNoURLTestAnywhere: when no endpoint carries url_test,
// target=test has nothing to try and must return a no-endpoint error
// rather than silently fetching prod.
func TestFetch_TestNoURLTestAnywhere(t *testing.T) {
	prod := bundleServer(t, "prod-bytes", "tok")
	cfg := &Config{
		Endpoints: []Endpoint{{Label: "ep", URL: prod.URL}},
		Token:     "tok",
	}
	_, _, err := Fetch(cfg, TargetTest)
	if err == nil {
		t.Fatal("Fetch: expected error when no endpoint has url_test, got nil")
	}
	if !strings.Contains(err.Error(), "test") {
		t.Errorf("Fetch error = %v, want mention of the test target", err)
	}
}

// TestFetch_ProdFailoverPreserved locks in the pre-target failover
// semantics: a failing first endpoint advances to the next, and the
// winner is reported back.
func TestFetch_ProdFailoverPreserved(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(broken.Close)
	good := bundleServer(t, "good-bytes", "tok")
	cfg := &Config{
		Endpoints: []Endpoint{
			{Label: "broken", URL: broken.URL},
			{Label: "good", URL: good.URL},
		},
		Token: "tok",
	}
	body, ep, err := Fetch(cfg, TargetProd)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if string(body) != "good-bytes" {
		t.Errorf("body = %q, want good-bytes", body)
	}
	if ep.Label != "good" {
		t.Errorf("endpoint label = %q, want good", ep.Label)
	}
}
