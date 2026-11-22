# harden

Small security primitives that go-authn repositories had each copied, in one
module whose only dependency is `golang.org/x/sys`.

```
go get github.com/go-authn/harden
```

## `redirect`

The rule applied before a token, a key set or a signed assertion crosses a
link: **the URL is https, or it names this machine.**

```go
import "github.com/go-authn/harden/redirect"

client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: redirect.CheckRedirect}

if err := redirect.HTTPSOrLoopback(issuer); err != nil { ... }
```

| Function | |
|---|---|
| `CheckRedirect(req, via) error` | an `http.Client` policy: stops after 10 redirects and refuses a target that is not https and not loopback (https to http on another host is the case it exists for). Chain your own policy after it. |
| `HTTPSOrLoopback(rawURL) error` | nil for `https://host/...`, or `http://` with a loopback host. Refuses no host, other schemes and cleartext. The error names scheme and host, never the whole URL. |
| `Loopback(host) bool` | `host` as `url.URL.Hostname()` returns it. |

`Loopback` accepts an IP literal that is loopback (127.0.0.0/8, `::1`,
IPv4-mapped `::ffff:127.0.0.1`), and `localhost` or any name under
`.localhost` (RFC 6761 section 6.3), in any letter case, with or without the
trailing dot. It never matches a prefix, so `127.evil.example` is refused, as
are `0x7f.1`, `127.1`, `2130706433`, an IPv6 zone and the bracketed `[::1]`
(brackets are URL syntax, not part of a host).

`Loopback` and `HTTPSOrLoopback` are fuzzed against a second reading of the
RFC; the corpus is committed under `redirect/testdata/fuzz`.

## `privfile`

```go
import "github.com/go-authn/harden/privfile"

err := privfile.Write(path, ticket)
```

`Write` puts data at `path` with mode 0600: a temporary file in the same
directory, `fsync`, then `rename`. So:

- a reader sees the old file or the new one, never half of one;
- a symbolic link left at `path` is **replaced, not followed** (what it pointed
  at is untouched), and a file of looser mode does not keep its mode;
- on Unix, a directory that is a symbolic link, belongs to someone else or is
  writable by group or others is refused before anything is written.

It does not create the directory. On Windows there is no mode to read; the
file is made with a protected access list holding one entry, the current
account, set by CreateFile (go-authn/kinit v0.3.2's list), so the folder's
entries are never inherited.

## `memlock`

Keeps the pages that hold key material out of swap, and keeps the process out of
core files and debuggers' hands. It moved here from go-theca/theca
(`internal/memlock`, theca#68).

```go
import "github.com/go-authn/harden/memlock"

region, err := memlock.Lock(unsafe.Pointer(&key[0]), uintptr(len(key)))
if err != nil { ... } // refused: RLIMIT_MEMLOCK, EPERM, a working-set quota
defer region.Release()

err = memlock.DisableDumps()
```

| | |
|---|---|
| `Lock(p, n) (*Region, error)` | pins the pages holding `[p, p+n)` (`mlock`; `VirtualLock` on Windows). **A refusal is an error**, wrapped so `errors.Is` sees the errno, and nothing stays locked. |
| `(*Region).Release()` | unlocks the pages no other region holds. Safe on nil, idempotent. Does not wipe. |
| `NewLocker(Syscalls, pageSize)` | a Locker with its own page counts and system calls, for tests that must make a refusal happen on any machine. `Default` is the one `Lock` uses. |
| `(*Locker).LockBestEffort(p, n, report)` | the only call that tolerates refusals: each one goes to `report`, the page is skipped, the region holds the rest. |
| `DisableDumps() error` | `RLIMIT_CORE` 0 on Linux and macOS; also `PR_SET_DUMPABLE` 0 on Linux, `PT_DENY_ATTACH` on macOS only. |
| `*UnsupportedError` | typed, matches `errors.ErrUnsupported`: returned by `Lock` on a platform with no page lock, and by `DisableDumps` on Windows and every system but Linux and macOS. |

mlock and VirtualLock do not count: one unlock of a page undoes every lock of
it. Two small secrets often share a page, so each page carries a count here and
is unlocked only when the last region that locked it is released.

**What it guarantees.** While a region is held, every page overlapping the
range is resident in RAM; a failure to guarantee that is an error, not a no-op.

**What it does not.** It locks an object that already exists and never copies or
wipes it. The Go garbage collector, a stack growth, an append or a copy made
*before* the call may have left other copies of the bytes in pages that are not
locked, possibly in swap already. Allocate the secret once, lock it before
filling it, never copy it, wipe it yourself before `Release`, and lock heap
memory only (a stack object can be moved by the runtime). Locking does not stop
hibernation, nor a debugger on Windows (no call makes a Windows process
undumpable) or one run by root.

CI runs the real system calls on Linux, macOS and Windows with `go test -v`;
the Windows run proves the lock with `VirtualUnlock` (it succeeds only on a
locked page and fails with `ERROR_NOT_LOCKED` after `Release`).

## Floors

go-authn/wireguard and go-authn/revocation both keep an ordering floor across a
restart, but they do not share a floor: wireguard keeps a bare version and
issue time in a small file of its own, revocation keeps the whole signed list
and recomputes the floor by verifying it (falling back to an unverified read
of the damaged copy). Only the file write is common, and that is `privfile`.

## License

BSD 3-Clause.
