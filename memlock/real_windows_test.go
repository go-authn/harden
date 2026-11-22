//go:build windows

package memlock

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func unmappedPage(t *testing.T) (unsafe.Pointer, uintptr) {
	t.Helper()
	size := pageSize()
	// Reserved and never committed, and kept reserved until the test ends: a
	// freed address can be handed back to the runtime between two calls (seen
	// on windows-latest), a reserved one cannot, and VirtualLock refuses it.
	addr, err := windows.VirtualAlloc(0, size, windows.MEM_RESERVE, windows.PAGE_NOACCESS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := windows.VirtualFree(addr, 0, windows.MEM_RELEASE); err != nil {
			t.Error(err)
		}
	})
	// The address is not Go memory, and converts as a word.
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr)), size
}

// assertLocked proves the state with the system itself: VirtualUnlock succeeds
// only on a page that is locked and fails with ERROR_NOT_LOCKED otherwise. It
// unlocks as a side effect, so a locked page is locked again afterwards.
func assertLocked(t *testing.T, b []byte, want bool) {
	t.Helper()
	addr, n := uintptr(unsafe.Pointer(&b[0])), uintptr(len(b))
	err := windows.VirtualUnlock(addr, n)
	switch {
	case want && err != nil:
		t.Fatalf("the page is not locked (VirtualUnlock: %v)", err)
	case want:
		if err := windows.VirtualLock(addr, n); err != nil {
			t.Fatal(err)
		}
		t.Logf("VirtualLock held %d bytes: VirtualUnlock succeeded on them", n)
	case err != windows.ERROR_NOT_LOCKED:
		t.Fatalf("after Release the page must be unlocked, VirtualUnlock said %v", err)
	default:
		t.Logf("after Release VirtualUnlock says ERROR_NOT_LOCKED, as it must")
	}
}
