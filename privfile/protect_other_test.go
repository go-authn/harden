//go:build !unix

package privfile

// protectCases: where protect does not touch the file there is nothing for it
// to refuse (the Windows owner-only list arrives with go-authn/kinit's code).
func protectCases(error) map[string]*fake { return nil }
