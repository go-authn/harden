//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly || windows

package memlock

import (
	"errors"
	"os"
	"testing"
	"unsafe"
)

// These tests call the real system calls. They never skip: a runner that
// refuses a one-page lock fails them, which is what "fail loudly" means.

func TestLockRealPage(t *testing.T) {
	size := pageSize()
	b := make([]byte, size)
	r, err := Lock(unsafe.Pointer(&b[0]), size)
	if err != nil {
		t.Fatalf("the real lock of one page was refused: %v", err)
	}
	assertLocked(t, b, true)
	b[0] = 1 // the object is alive and usable while locked
	r.Release()
	assertLocked(t, b, false)
	r.Release()
}

func TestLockRealRefusalIsAnErrorNotAGrowth(t *testing.T) {
	// A page that was mapped and is gone: the system refuses to lock it. No
	// allocation of any size is made.
	p, n := unmappedPage(t)
	r, err := lockRefused(t, Default, p, n)
	if err == nil || r != nil {
		t.Fatalf("locking an unmapped page returned %v, %v", r, err)
	}
	if errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("a refusal must not read as unsupported: %v", err)
	}
	var reported error
	if r := Default.LockBestEffort(p, n, func(err error) { reported = err }); r == nil || reported == nil {
		t.Fatalf("best effort: %v, %v", r, reported)
	}
}

func TestDefaultPageSize(t *testing.T) {
	if got := pageSize(); got != uintptr(os.Getpagesize()) || got&(got-1) != 0 {
		t.Fatalf("page size %d", got)
	}
}
