//go:build unix

package privfile

// protectCases: on Unix protect is a chmod, so a file that refuses it fails.
func protectCases(boom error) map[string]*fake {
	return map[string]*fake{"protect": {cherr: boom}}
}
