//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || windows)

package memlock

import "unsafe"

func pointerTo(b []byte) unsafe.Pointer { return unsafe.Pointer(&b[0]) }
