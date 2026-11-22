//go:build !linux && !darwin

package memlock

// DisableDumps is not available here: Windows has no call that makes a process
// undumpable (no PR_SET_DUMPABLE, no PT_DENY_ATTACH, no RLIMIT_CORE), and the
// other systems are not covered. The error is typed (*UnsupportedError), so a
// caller can tell "cannot" from "refused".
func DisableDumps() error { return &UnsupportedError{Op: "disable core dumps"} }
