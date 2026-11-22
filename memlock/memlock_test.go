package memlock

import (
	"errors"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

const fakePage = 256

type call struct {
	p unsafe.Pointer
	n uintptr
}

// fake records the calls and refuses the pages listed in refuse (by index of
// the lock call that makes it).
type fake struct {
	locks, unlocks []call
	refuseCall     map[int]error
}

func (f *fake) sys() Syscalls {
	return Syscalls{
		Lock: func(p unsafe.Pointer, n uintptr) error {
			i := len(f.locks)
			f.locks = append(f.locks, call{p, n})
			return f.refuseCall[i]
		},
		Unlock: func(p unsafe.Pointer, n uintptr) { f.unlocks = append(f.unlocks, call{p, n}) },
	}
}

// pages returns a buffer whose first byte starts a fakePage-sized page and that
// holds two such pages, so ranges can be placed on page boundaries exactly.
func pages(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 3*fakePage)
	off := (fakePage - uintptr(unsafe.Pointer(&b[0]))%fakePage) % fakePage
	return b[off : off+2*fakePage]
}

func at(b []byte, i int) unsafe.Pointer { return unsafe.Pointer(&b[i]) }

func TestPagesSharedByTwoRegionsStayLockedUntilTheLastRelease(t *testing.T) {
	b := pages(t)
	f := &fake{}
	l := NewLocker(f.sys(), fakePage)
	r1, err := l.Lock(at(b, 0), 10)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := l.Lock(at(b, 20), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.locks) != 1 {
		t.Fatalf("the second region on the same page called mlock again: %d calls", len(f.locks))
	}
	r1.Release()
	r1.Release() // a second call does nothing
	if len(f.unlocks) != 0 {
		t.Fatal("the page was unlocked while a region still held it")
	}
	r2.Release()
	if len(f.unlocks) != 1 || f.unlocks[0] != (call{at(b, 20), 10}) {
		t.Fatalf("unlocks %v", f.unlocks)
	}
}

func TestARangeOverTwoPagesLocksEachPartOnce(t *testing.T) {
	b := pages(t)
	f := &fake{}
	l := NewLocker(f.sys(), fakePage)
	r, err := l.Lock(at(b, 100), 300)
	if err != nil {
		t.Fatal(err)
	}
	want := []call{{at(b, 100), fakePage - 100}, {at(b, fakePage), 144}}
	if len(f.locks) != 2 || f.locks[0] != want[0] || f.locks[1] != want[1] {
		t.Fatalf("locks %v want %v", f.locks, want)
	}
	r.Release()
	if len(f.unlocks) != 2 {
		t.Fatalf("unlocks %v", f.unlocks)
	}
}

func TestAZeroLengthLocksNothing(t *testing.T) {
	b := pages(t)
	f := &fake{}
	r, err := NewLocker(f.sys(), fakePage).Lock(at(b, 0), 0)
	if r == nil || err != nil || len(f.locks) != 0 {
		t.Fatalf("%v %v %v", r, err, f.locks)
	}
	r.Release()
}

func TestARefusalIsAnErrorAndHoldsNothing(t *testing.T) {
	b := pages(t)
	f := &fake{refuseCall: map[int]error{1: syscall.ENOMEM}}
	l := NewLocker(f.sys(), fakePage)
	r, err := lockRefused(t, l, at(b, 0), 2*fakePage)
	if r != nil || !errors.Is(err, syscall.ENOMEM) || !strings.HasPrefix(err.Error(), "memlock: ") {
		t.Fatalf("region %v error %v", r, err)
	}
	if len(f.locks) != 2 || len(f.unlocks) != 1 || f.unlocks[0].p != at(b, 0) {
		t.Fatalf("locks %v unlocks %v: the first page must be given back", f.locks, f.unlocks)
	}
	if len(l.counts) != 0 {
		t.Fatalf("counts left: %v", l.counts)
	}
}

func TestARefusalDoesNotUnlockAPageAnotherRegionHolds(t *testing.T) {
	b := pages(t)
	f := &fake{refuseCall: map[int]error{1: syscall.EPERM}}
	l := NewLocker(f.sys(), fakePage)
	r1, err := l.Lock(at(b, 0), 1) // call 0
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockRefused(t, l, at(b, 0), 2*fakePage); !errors.Is(err, syscall.EPERM) { // page 0 shared, page 1 refused (call 1)
		t.Fatal(err)
	}
	if len(f.unlocks) != 0 {
		t.Fatalf("the shared page was unlocked: %v", f.unlocks)
	}
	r1.Release()
	if len(f.unlocks) != 1 {
		t.Fatalf("unlocks %v", f.unlocks)
	}
}

func TestBestEffortReportsEachRefusalAndHoldsTheRest(t *testing.T) {
	b := pages(t)
	f := &fake{refuseCall: map[int]error{0: syscall.ENOMEM}}
	l := NewLocker(f.sys(), fakePage)
	var got []error
	r := l.LockBestEffort(at(b, 0), 2*fakePage, func(err error) { got = append(got, err) })
	if r == nil || len(got) != 1 || !errors.Is(got[0], syscall.ENOMEM) {
		t.Fatalf("region %v reports %v", r, got)
	}
	r.Release()
	if len(f.unlocks) != 1 || f.unlocks[0].p != at(b, fakePage) {
		t.Fatalf("only the locked page is unlocked: %v", f.unlocks)
	}
}

func TestARangeThatWrapsIsRefused(t *testing.T) {
	b := pages(t)
	l := NewLocker((&fake{}).sys(), fakePage)
	top := ^uintptr(0) - uintptr(unsafe.Pointer(&b[0])) // p+top is the last address: rounds up past it
	for _, n := range []uintptr{^uintptr(0), top} {
		if r, err := l.Lock(at(b, 0), n); r != nil || !errors.Is(err, ErrRange) {
			t.Fatalf("n=%d: %v %v", n, r, err)
		}
		var got error
		if r := l.LockBestEffort(at(b, 0), n, func(err error) { got = err }); r == nil || !errors.Is(got, ErrRange) {
			t.Fatalf("best effort n=%d: %v %v", n, r, got)
		}
	}
}

// lockRefused is l.Lock for a call that must be refused, failing the test with
// a message (not a crash) when the refusal path misbehaves.
func lockRefused(t *testing.T, l *Locker, p unsafe.Pointer, n uintptr) (r *Region, err error) {
	t.Helper()
	mustNotPanic(t, "a refused Lock", func() { r, err = l.Lock(p, n) })
	return r, err
}

func TestRangeEndEdges(t *testing.T) {
	const max = ^uintptr(0)
	for _, c := range []struct {
		start, n, ps uintptr
		ok           bool
	}{
		{0x1000, 16, 4096, true},
		{0x1000, 0, 4096, true},
		{max - 4096 - 10, 10, 4096, true},  // end is exactly the last allowed address
		{max - 4096 - 10, 11, 4096, false}, // one past it
		{0x1000, max, 4096, false},         // wraps
		{max, 1, 4096, false},              // wraps to zero
	} {
		if _, ok := rangeEnd(c.start, c.n, c.ps); ok != c.ok {
			t.Errorf("rangeEnd(%#x, %#x, %d) ok=%v want %v", c.start, c.n, c.ps, ok, c.ok)
		}
	}
}

// mustNotPanic fails the test with what f panicked with: a guard that is held
// only by a crash is not held by an assertion.
func mustNotPanic(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s crashed: %v", what, r)
		}
	}()
	f()
}

func TestNilsAreSafe(t *testing.T) {
	b := pages(t)
	mustNotPanic(t, "Release of a nil region", func() { (*Region)(nil).Release() })
	f := &fake{refuseCall: map[int]error{0: syscall.ENOMEM}}
	mustNotPanic(t, "a refusal with a nil reporter", func() {
		// A nil reporter drops the refusal and still returns a region.
		if r := NewLocker(f.sys(), fakePage).LockBestEffort(at(b, 0), 1, nil); r == nil {
			t.Error("nil region")
		}
	})
}

func TestNewLockerNeedsAPowerOfTwo(t *testing.T) {
	for _, size := range []uintptr{0, 3, 100} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("page size %d accepted", size)
				}
			}()
			NewLocker(Syscalls{}, size)
		}()
	}
}

func TestUnsupportedErrorIsTyped(t *testing.T) {
	var err error = &UnsupportedError{Op: "lock memory"}
	var u *UnsupportedError
	if !errors.As(err, &u) || !errors.Is(err, errors.ErrUnsupported) || errors.Is(err, ErrRange) {
		t.Fatal("not typed")
	}
	if !strings.Contains(err.Error(), "lock memory") || !strings.Contains(err.Error(), runtime.GOOS) {
		t.Fatal(err)
	}
}
