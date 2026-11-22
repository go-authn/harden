//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || windows)

package memlock

import "unsafe"

func pageSize() uintptr { return 4096 }

// lockRange: this platform has no page lock this package calls. The error is
// typed, never a silent success.
func lockRange(unsafe.Pointer, uintptr) error { return &UnsupportedError{Op: "lock memory"} }

func unlockRange(unsafe.Pointer, uintptr) {}
