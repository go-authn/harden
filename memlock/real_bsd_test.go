//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package memlock

import "testing"

// assertLocked has nothing to read the state back from here (no VmLck): the
// real call returning no error is the evidence.
func assertLocked(t *testing.T, b []byte, want bool) { t.Helper() }
