//go:build darwin

package memlock

import (
	"errors"

	"golang.org/x/sys/unix"
)

// The system calls, replaced by tests.
var (
	setrlimit  = unix.Setrlimit
	denyAttach = unix.PtraceDenyAttach
)

// DisableDumps removes the core file (RLIMIT_CORE 0) and refuses debuggers
// (PT_DENY_ATTACH: a later attach fails, and a debugger already attached is
// ended). Both are tried; the errors are joined. PT_DENY_ATTACH exists on
// darwin only.
func DisableDumps() error {
	return errors.Join(
		setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{Cur: 0, Max: 0}),
		denyAttach(),
	)
}
