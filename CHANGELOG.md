# Changelog

## v0.3.0

- New package `memlock`, moved from go-theca/theca `internal/memlock` (theca#68)
  with no theca names: `Lock` and `Region` (page-counted mlock, and VirtualLock
  on Windows, which theca never had), `Locker`/`NewLocker`/`Syscalls` for tests,
  `DisableDumps` (RLIMIT_CORE, PR_SET_DUMPABLE on Linux, PT_DENY_ATTACH on
  macOS), and the typed `*UnsupportedError`.
- A refused lock is an error. The one tolerant call is named `LockBestEffort`.
  theca's mode policy (`--mlock auto|off|required`) stays in theca.

## v0.2.0

- `privfile.Write` on Windows makes its temporary file with a protected DACL
  holding one allow entry for the current account (FILE_ALL_ACCESS), set by
  CreateFile at creation, as go-authn/kinit v0.3.2 does; the rename keeps it.
  Before, the file inherited the folder's access list. The module now requires
  `golang.org/x/sys` (Windows builds only use it).

## v0.1.0

First release: the two security primitives that were copied across go-authn
repositories, in one dependency-free module.

- `redirect`: `CheckRedirect`, `HTTPSOrLoopback`, `Loopback` (from go-authn/oidc,
  compared with the copies in rp, awscred, saml and wireguard).
- `privfile`: `Write`, a 0600 temporary file in the target's directory, synced
  and renamed over it, in a directory only its owner can write.
