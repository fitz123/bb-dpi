package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// TestLogSync_MultilineMessage covers the regression Codex caught in
// the round-1 review: sing-box check / xray -test failures get folded
// into res.Err and contain embedded \n. The previous version of
// logSync prefixed only the FIRST physical line, leaving continuation
// lines un-timestamped in /Library/Logs/bb-dpi/bb-vpn-sync.log and
// breaking grep-by-timestamp diagnosis.
func TestLogSync_MultilineMessage(t *testing.T) {
	var buf bytes.Buffer
	orig := logSyncOut
	logSyncOut = &buf
	defer func() { logSyncOut = orig }()

	logSync("error duration_ms=42: first line\nsecond line\nthird line")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 physical lines, got %d: %q", len(lines), buf.String())
	}
	// Every line must start with a millisecond-precision RFC3339 UTC
	// timestamp followed by " bb-vpn sync: ".
	pattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z bb-vpn sync: `)
	expectedTails := []string{
		"error duration_ms=42: first line",
		"second line",
		"third line",
	}
	for i, line := range lines {
		if !pattern.MatchString(line) {
			t.Errorf("line %d missing timestamp prefix: %q", i, line)
		}
		if !strings.HasSuffix(line, expectedTails[i]) {
			t.Errorf("line %d wrong tail: got %q, want suffix %q", i, line, expectedTails[i])
		}
	}
}

// TestLogSync_SingleLineMessage is the happy path: a one-line message
// produces one prefixed line, no spurious empty trailing line.
func TestLogSync_SingleLineMessage(t *testing.T) {
	var buf bytes.Buffer
	orig := logSyncOut
	logSyncOut = &buf
	defer func() { logSyncOut = orig }()

	logSync("ok (duration_ms=123)")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 physical line, got %d: %q", len(lines), buf.String())
	}
	if !strings.HasSuffix(lines[0], "bb-vpn sync: ok (duration_ms=123)") {
		t.Errorf("unexpected output: %q", lines[0])
	}
}

// TestLogSync_TrailingNewlineDoesNotEmitEmptyLine guards against the
// strings.Split foot-gun where "msg\n" → ["msg", ""] would otherwise
// produce a spurious empty timestamped line.
func TestLogSync_TrailingNewlineDoesNotEmitEmptyLine(t *testing.T) {
	var buf bytes.Buffer
	orig := logSyncOut
	logSyncOut = &buf
	defer func() { logSyncOut = orig }()

	logSync("trailing newline\n")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 physical line, got %d: %q", len(lines), buf.String())
	}
}
