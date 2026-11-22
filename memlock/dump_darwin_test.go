//go:build darwin

package memlock

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestDisableDumpsCallsRlimitAndDenyAttach(t *testing.T) {
	sr, sd := setrlimit, denyAttach
	defer func() { setrlimit, denyAttach = sr, sd }()
	var gotRes int
	var gotLim unix.Rlimit
	denied := 0
	setrlimit = func(res int, l *unix.Rlimit) error { gotRes, gotLim = res, *l; return nil }
	denyAttach = func() error { denied++; return nil }
	if err := DisableDumps(); err != nil {
		t.Fatal(err)
	}
	if gotRes != unix.RLIMIT_CORE || gotLim != (unix.Rlimit{}) || denied != 1 {
		t.Fatalf("calls: %d %v %d", gotRes, gotLim, denied)
	}
	setrlimit = func(int, *unix.Rlimit) error { return errors.New("rlimit refused") }
	denyAttach = func() error { return errors.New("deny refused") }
	err := DisableDumps()
	if err == nil || !strings.Contains(err.Error(), "rlimit refused") || !strings.Contains(err.Error(), "deny refused") {
		t.Fatalf("both failures must be joined: %v", err)
	}
}

// childChecks: PT_DENY_ATTACH has no read-back, so the real call returning no
// error is the evidence.
func childChecks() error { return nil }
