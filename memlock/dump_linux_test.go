//go:build linux

package memlock

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestDisableDumpsCallsRlimitAndDumpable(t *testing.T) {
	sr, sp := setrlimit, prctl
	defer func() { setrlimit, prctl = sr, sp }()
	var gotRes int
	var gotLim unix.Rlimit
	var gotOpt int
	var gotArg uintptr
	setrlimit = func(res int, l *unix.Rlimit) error { gotRes, gotLim = res, *l; return nil }
	prctl = func(opt int, a2, a3, a4, a5 uintptr) error { gotOpt, gotArg = opt, a2; return nil }
	if err := DisableDumps(); err != nil {
		t.Fatal(err)
	}
	if gotRes != unix.RLIMIT_CORE || gotLim != (unix.Rlimit{}) || gotOpt != unix.PR_SET_DUMPABLE || gotArg != 0 {
		t.Fatalf("calls: %d %v %d %d", gotRes, gotLim, gotOpt, gotArg)
	}
	setrlimit = func(int, *unix.Rlimit) error { return errors.New("rlimit refused") }
	prctl = func(int, uintptr, uintptr, uintptr, uintptr) error { return errors.New("prctl refused") }
	err := DisableDumps()
	if err == nil || !strings.Contains(err.Error(), "rlimit refused") || !strings.Contains(err.Error(), "prctl refused") {
		t.Fatalf("both failures must be joined: %v", err)
	}
}

// childChecks is what the re-executed test binary verifies after the real
// DisableDumps, beyond the core limit.
func childChecks() error {
	if d, err := unix.PrctlRetInt(unix.PR_GET_DUMPABLE, 0, 0, 0, 0); err != nil || d != 0 {
		return errors.New("PR_GET_DUMPABLE is not 0")
	}
	return nil
}
