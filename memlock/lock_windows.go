//go:build windows

package memlock

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func pageSize() uintptr { return uintptr(os.Getpagesize()) }

// lockRange is VirtualLock: the pages stay in the working set. The system
// refuses it when the range is not committed memory or the process's working
// set quota is too small (ERROR_WORKING_SET_QUOTA).
func lockRange(p unsafe.Pointer, n uintptr) error {
	return windows.VirtualLock(uintptr(p), n)
}

func unlockRange(p unsafe.Pointer, n uintptr) {
	_ = windows.VirtualUnlock(uintptr(p), n)
}
