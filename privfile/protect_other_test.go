//go:build !unix

package privfile

// protectCases: where protect does not touch the file there is nothing for it
// to refuse (the Windows owner-only list is set at creation, and tested in
// temp_windows_test.go).
func protectCases(error) map[string]*fake { return nil }
