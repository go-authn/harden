//go:build !linux && !darwin

package memlock

import (
	"errors"
	"testing"
)

func TestDisableDumpsIsUnsupportedHere(t *testing.T) {
	err := DisableDumps()
	var u *UnsupportedError
	if !errors.As(err, &u) || !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("want a typed unsupported error, got %v", err)
	}
}
