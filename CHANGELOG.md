# Changelog

## v0.1.0

First release: the two security primitives that were copied across go-authn
repositories, in one dependency-free module.

- `redirect`: `CheckRedirect`, `HTTPSOrLoopback`, `Loopback` (from go-authn/oidc,
  compared with the copies in rp, awscred, saml and wireguard).
- `privfile`: `Write`, a 0600 temporary file in the target's directory, synced
  and renamed over it, in a directory only its owner can write.
