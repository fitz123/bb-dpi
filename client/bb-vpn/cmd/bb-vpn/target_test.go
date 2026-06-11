package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"bb-dpi/client/bb-vpn/pkg/state"
)

// withStateRoot points pkg/state at a tempdir for the duration of t.
func withStateRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("BB_VPN_STATE_ROOT", dir)
	return dir
}

// withEUID swaps the geteuid seam so the root gate in targetCmd can be
// driven deterministically.
func withEUID(t *testing.T, uid int) {
	t.Helper()
	orig := geteuid
	geteuid = func() int { return uid }
	t.Cleanup(func() { geteuid = orig })
}

// captureStdout returns everything fn writes to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return string(data)
}

func TestParseTargetArg(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    state.Target
		wantErr bool
	}{
		{"no args reads", nil, "", false},
		{"test", []string{"test"}, state.TargetTest, false},
		{"prod", []string{"prod"}, state.TargetProd, false},
		{"unknown value", []string{"staging"}, "", true},
		{"wrong case", []string{"TEST"}, "", true},
		{"empty string arg", []string{""}, "", true},
		{"extra args", []string{"test", "prod"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTargetArg(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (got=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestTargetCmd_ReadNeedsNoRoot(t *testing.T) {
	withStateRoot(t)
	withEUID(t, 501)
	var code int
	out := captureStdout(t, func() { code = targetCmd(nil) })
	if code != 0 {
		t.Fatalf("targetCmd(nil) = %d, want 0", code)
	}
	if strings.TrimSpace(out) != "prod" {
		t.Errorf("default read printed %q, want \"prod\"", strings.TrimSpace(out))
	}
}

func TestTargetCmd_ReadReflectsSetValue(t *testing.T) {
	withStateRoot(t)
	withEUID(t, 501)
	if err := state.SetTarget(state.TargetTest); err != nil {
		t.Fatalf("SetTarget: %v", err)
	}
	var code int
	out := captureStdout(t, func() { code = targetCmd(nil) })
	if code != 0 {
		t.Fatalf("targetCmd(nil) = %d, want 0", code)
	}
	if strings.TrimSpace(out) != "test" {
		t.Errorf("read printed %q, want \"test\"", strings.TrimSpace(out))
	}
}

func TestTargetCmd_WriteRequiresRoot(t *testing.T) {
	withStateRoot(t)
	withEUID(t, 501)
	if code := targetCmd([]string{"test"}); code != exitUsage {
		t.Fatalf("non-root write = %d, want exitUsage %d", code, exitUsage)
	}
	if _, err := os.Stat(state.Path(state.TargetFile)); !os.IsNotExist(err) {
		t.Errorf("target file exists after rejected non-root write (stat err: %v)", err)
	}
}

func TestTargetCmd_WriteAsRootRoundTrip(t *testing.T) {
	withStateRoot(t)
	withEUID(t, 0)
	out := captureStdout(t, func() {
		if code := targetCmd([]string{"test"}); code != 0 {
			t.Errorf("targetCmd(test) = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "now test") {
		t.Errorf("write output %q missing confirmation", out)
	}
	if !strings.Contains(out, "sudo bb-vpn sync") {
		t.Errorf("write output %q missing sync hint", out)
	}
	if got := state.ActiveTarget(); got != state.TargetTest {
		t.Fatalf("ActiveTarget after write = %q, want test", got)
	}
	captureStdout(t, func() {
		if code := targetCmd([]string{"prod"}); code != 0 {
			t.Errorf("targetCmd(prod) = %d, want 0", code)
		}
	})
	if got := state.ActiveTarget(); got != state.TargetProd {
		t.Fatalf("ActiveTarget after revert = %q, want prod", got)
	}
}

func TestTargetCmd_InvalidValueUsageExit(t *testing.T) {
	withStateRoot(t)
	// Root euid so a usage exit can only come from arg validation,
	// not the root gate.
	withEUID(t, 0)
	for _, args := range [][]string{{"staging"}, {"TEST"}, {"test", "prod"}} {
		if code := targetCmd(args); code != exitUsage {
			t.Errorf("targetCmd(%v) = %d, want exitUsage %d", args, code, exitUsage)
		}
	}
	if _, err := os.Stat(state.Path(state.TargetFile)); !os.IsNotExist(err) {
		t.Errorf("target file exists after rejected writes (stat err: %v)", err)
	}
}

func TestStatusCmd_TargetFallsBackToSelector(t *testing.T) {
	withStateRoot(t)
	// No status.json at all: Target is empty in the zero-value Status,
	// so the printed line must fall back to state.ActiveTarget().
	out := captureStdout(t, func() {
		if code := statusCmd(nil); code != 0 {
			t.Errorf("statusCmd = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "target:               prod") {
		t.Errorf("status output missing prod fallback target line:\n%s", out)
	}
}

func TestStatusCmd_TargetFromStatusJSON(t *testing.T) {
	withStateRoot(t)
	if err := state.WriteStatus(state.Status{Target: "test"}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	out := captureStdout(t, func() {
		if code := statusCmd(nil); code != 0 {
			t.Errorf("statusCmd = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "target:               test") {
		t.Errorf("status output missing target line from status.json:\n%s", out)
	}
}
