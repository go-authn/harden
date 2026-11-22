//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package memlock

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func unmappedPage(t *testing.T) (unsafe.Pointer, uintptr) {
	t.Helper()
	size := int(pageSize())
	m, err := unix.Mmap(-1, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		t.Fatal(err)
	}
	p := unsafe.Pointer(&m[0])
	if err := unix.Munmap(m); err != nil {
		t.Fatal(err)
	}
	return p, uintptr(size)
}
