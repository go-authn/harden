// Package memlock keeps the pages that hold key material in RAM, so the
// operating system cannot write them to swap, and (DisableDumps) keeps the
// process from leaving its memory in a core file or in a debugger's hands.
//
// # Failing loudly
//
// Lock returns an error when the system refuses (RLIMIT_MEMLOCK, EPERM, a
// Windows working-set quota) and an *UnsupportedError, matching
// errors.ErrUnsupported, on a platform that has no page lock. It never turns a
// refusal into a silent success. The one function that tolerates refusals,
// Locker.LockBestEffort, says so in its name and hands each refusal to a
// callback.
//
// # What is guaranteed
//
// While a Region is held, every page that overlaps the locked range is resident
// in RAM, and a page shared by several regions stays locked until the last of
// them is released (mlock and VirtualLock do not count: one munlock of a page
// undoes every mlock of it, so the count is kept here).
//
// # What is not guaranteed
//
// The package locks the pages of an object that already exists and never copies,
// moves or wipes it. So:
//
//   - The Go garbage collector, a stack growth, an append or a copy made BEFORE
//     the call may have left other copies of the bytes in pages that are not
//     locked, and so possibly in swap already. Lock sees none of them. Allocate
//     the secret once, lock it before it is filled, and never copy it.
//   - Lock only heap or mapped memory. A stack object can be moved by the
//     runtime, which would leave the locked page behind.
//   - Releasing a Region does not wipe the bytes. Wipe them yourself, before
//     Release.
//   - Locking does not stop hibernation, a debugger, or a core dump; see
//     DisableDumps for the last two.
//
// A page address is kept as a number, never as a pointer: the page that holds an
// object also holds the end of its neighbour, and a pointer to a page start can
// point into a freed object, which the garbage collector refuses. The system
// calls are given pointers INSIDE the object that is being locked.
package memlock

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// UnsupportedError reports an operation this platform cannot do at all. It
// matches errors.ErrUnsupported.
type UnsupportedError struct {
	Op string // what was asked, such as "lock memory"
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("memlock: %s is not supported on %s/%s", e.Op, runtime.GOOS, runtime.GOARCH)
}

// Is makes errors.Is(err, errors.ErrUnsupported) true.
func (e *UnsupportedError) Is(target error) bool { return target == errors.ErrUnsupported }

// ErrRange is returned by Lock for a range that wraps around the address space.
var ErrRange = errors.New("memlock: range wraps around the address space")

// Syscalls are the two platform calls a Locker makes. NewLocker takes them so a
// test can make a refusal happen on any machine.
type Syscalls struct {
	// Lock pins the pages holding [p, p+n); p is inside a live object.
	Lock func(p unsafe.Pointer, n uintptr) error
	// Unlock is its inverse. A failure to unlock leaves nothing to do.
	Unlock func(p unsafe.Pointer, n uintptr)
}

// Locker owns the page counts. Callers that must share pages share one Locker;
// Default is the one the package-level functions use.
type Locker struct {
	sys      Syscalls
	pageSize uintptr

	mu     sync.Mutex
	counts map[uintptr]int
}

// NewLocker makes a Locker over sys with the given page size, which must be a
// power of two (1 makes every object its own page, so a test can make the lock
// of any allocation reach the system call). It panics otherwise.
func NewLocker(sys Syscalls, pageSize uintptr) *Locker {
	if pageSize == 0 || pageSize&(pageSize-1) != 0 {
		panic("memlock: page size must be a power of two")
	}
	return &Locker{sys: sys, pageSize: pageSize, counts: map[uintptr]int{}}
}

// Default is the Locker of the package-level Lock, over the real system calls
// and the real page size.
var Default = NewLocker(Syscalls{Lock: lockRange, Unlock: unlockRange}, pageSize())

// span is the part of one page that an object covers.
type span struct {
	page uintptr
	p    unsafe.Pointer // inside the object, which it keeps alive
	n    uintptr
}

// Region is the set of pages one Lock call holds. It keeps the object alive.
type Region struct {
	owner *Locker
	spans []span
}

// Release unlocks the pages that no other region holds. It is safe on nil, and
// a second call does nothing. It does not wipe the bytes.
func (r *Region) Release() {
	if r == nil {
		return
	}
	r.owner.mu.Lock()
	defer r.owner.mu.Unlock()
	r.owner.release(r)
}

// release drops one count on each span of r; mu is held.
func (l *Locker) release(r *Region) {
	for _, s := range r.spans {
		c := l.counts[s.page] - 1
		if c <= 0 {
			delete(l.counts, s.page)
			l.sys.Unlock(s.p, s.n)
		} else {
			l.counts[s.page] = c
		}
	}
	r.spans = nil
}

// Lock pins the pages holding [p, p+n), p being inside a live object, with
// Default. See Locker.Lock.
func Lock(p unsafe.Pointer, n uintptr) (*Region, error) { return Default.Lock(p, n) }

// Lock pins the pages holding [p, p+n), p being inside a live object. Either
// every page is locked and the Region holds them, or the first refusal is
// returned (wrapped, so errors.Is sees the errno) and nothing stays locked.
// n == 0 locks nothing and succeeds.
func (l *Locker) Lock(p unsafe.Pointer, n uintptr) (*Region, error) {
	return l.lock(p, n, nil)
}

// LockBestEffort locks the pages it can. Each refusal is passed to report
// (nil: dropped, which is the point of the name) and the page is skipped; the
// returned Region holds the pages that were locked and is never nil. Use Lock
// when a refusal must stop the caller.
func (l *Locker) LockBestEffort(p unsafe.Pointer, n uintptr, report func(error)) *Region {
	if report == nil {
		report = func(error) {}
	}
	r, _ := l.lock(p, n, report)
	return r
}

// rangeEnd is start+n, and whether the range can be walked a page at a time: it
// must neither wrap nor reach the last page of the address space, where the
// next page's address would wrap.
func rangeEnd(start, n, pageSize uintptr) (end uintptr, ok bool) {
	end = start + n
	return end, end >= start && end <= ^uintptr(0)-pageSize
}

// lock is Lock when report is nil and LockBestEffort otherwise. A range that
// wraps is an error in both: nothing was asked that could be skipped.
func (l *Locker) lock(p unsafe.Pointer, n uintptr, report func(error)) (*Region, error) {
	r := &Region{owner: l}
	start := uintptr(p)
	end, ok := rangeEnd(start, n, l.pageSize)
	if !ok {
		if report != nil {
			report(ErrRange)
			return r, nil
		}
		return nil, ErrRange
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for pg := start &^ (l.pageSize - 1); pg < end; pg += l.pageSize {
		lo, hi := max(start, pg), min(end, pg+l.pageSize)
		s := span{page: pg, p: unsafe.Add(p, lo-start), n: hi - lo}
		if l.counts[pg] == 0 {
			if err := l.sys.Lock(s.p, s.n); err != nil {
				err = fmt.Errorf("memlock: %w", err)
				if report == nil {
					l.release(r)
					return nil, err
				}
				report(err)
				continue
			}
		}
		l.counts[pg]++
		r.spans = append(r.spans, s)
	}
	return r, nil
}
