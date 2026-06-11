package state

import (
	"os"
	"testing"
)

func TestActiveTarget_DefaultProd(t *testing.T) {
	withRoot(t)
	if got := ActiveTarget(); got != TargetProd {
		t.Errorf("ActiveTarget with no file = %q, want %q", got, TargetProd)
	}
}

func TestSetTarget_RoundTrip(t *testing.T) {
	withRoot(t)
	if err := SetTarget(TargetTest); err != nil {
		t.Fatalf("SetTarget(test): %v", err)
	}
	if got := ActiveTarget(); got != TargetTest {
		t.Errorf("ActiveTarget after SetTarget(test) = %q, want %q", got, TargetTest)
	}
	if err := SetTarget(TargetProd); err != nil {
		t.Fatalf("SetTarget(prod): %v", err)
	}
	if got := ActiveTarget(); got != TargetProd {
		t.Errorf("ActiveTarget after SetTarget(prod) = %q, want %q", got, TargetProd)
	}
}

func TestSetTarget_Mode0644(t *testing.T) {
	withRoot(t)
	if err := SetTarget(TargetTest); err != nil {
		t.Fatalf("SetTarget: %v", err)
	}
	info, err := os.Stat(Path(TargetFile))
	if err != nil {
		t.Fatalf("stat target file: %v", err)
	}
	// 0o644 so non-root readers (the menubar, un-sudo'd `bb-vpn target` /
	// `bb-vpn status`) can read the selector. A 0o600 (root-only) file
	// would make ActiveTarget() silently fall back to prod for every
	// non-root caller. The value is not secret.
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("target file mode = %o, want 0644", perm)
	}
}

func TestActiveTarget_EmptyFile(t *testing.T) {
	withRoot(t)
	if err := os.WriteFile(Path(TargetFile), nil, 0o600); err != nil {
		t.Fatalf("write empty target file: %v", err)
	}
	if got := ActiveTarget(); got != TargetProd {
		t.Errorf("ActiveTarget with empty file = %q, want %q", got, TargetProd)
	}
}

func TestActiveTarget_GarbageReadsProd(t *testing.T) {
	withRoot(t)
	for _, garbage := range []string{"staging", "TEST", "prod test", "test\nprod"} {
		if err := os.WriteFile(Path(TargetFile), []byte(garbage), 0o600); err != nil {
			t.Fatalf("write target file: %v", err)
		}
		if got := ActiveTarget(); got != TargetProd {
			t.Errorf("ActiveTarget(%q) = %q, want %q", garbage, got, TargetProd)
		}
	}
}

func TestActiveTarget_TrimsWhitespace(t *testing.T) {
	withRoot(t)
	if err := os.WriteFile(Path(TargetFile), []byte("test\n"), 0o600); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	if got := ActiveTarget(); got != TargetTest {
		t.Errorf("ActiveTarget(\"test\\n\") = %q, want %q", got, TargetTest)
	}
}

func TestSetTarget_RejectsInvalid(t *testing.T) {
	withRoot(t)
	for _, bad := range []Target{"", "staging", "TEST", "Prod"} {
		if err := SetTarget(bad); err == nil {
			t.Errorf("SetTarget(%q): want error, got nil", bad)
		}
	}
	// A rejected write must not create or alter the target file.
	if _, err := os.Stat(Path(TargetFile)); !os.IsNotExist(err) {
		t.Errorf("target file exists after rejected SetTarget calls (stat err: %v)", err)
	}
}
