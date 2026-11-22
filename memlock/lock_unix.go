//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package memlock

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

func pageSize() uintptr { return uintptr(os.Getpagesize()) }

// lockRange is mlock(2): RLIMIT_MEMLOCK or a missing privilege refuses it.
func lockRange(p unsafe.Pointer, n uintptr) error {
	return unix.Mlock(unsafe.Slice((*byte)(p), n))
}

func unlockRange(p unsafe.Pointer, n uintptr) {
	_ = unix.Munlock(unsafe.Slice((*byte)(p), n))
}
