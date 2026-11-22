//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || windows)

package memlock

import (
	"errors"
	"testing"
)

func TestLockIsUnsupportedHere(t *testing.T) {
	b := make([]byte, 16)
	r, err := Lock(pointerTo(b), uintptr(len(b)))
	var u *UnsupportedError
	if r != nil || !errors.As(err, &u) || !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("want a typed unsupported error, got %v, %v", r, err)
	}
	unlockRange(pointerTo(b), 1) // nothing to undo, and no crash
}
