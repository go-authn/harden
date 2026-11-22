//go:build !unix

package privfile

import (
	"fmt"
	"os"
)

// checkDir: there is no owner or mode bit to read here. The directory must
// exist and be a directory.
func checkDir(dir string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return nil
}

// protect: the file inherits the directory's access list. An owner-only list
// belongs here (go-authn/kinit's Windows code, once it is released).
func protect(file) error { return nil }
