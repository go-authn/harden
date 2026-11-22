//go:build !unix

package privfile

import (
	"os"
	"path/filepath"
	"testing"
)

func modeOK(os.FileMode) bool { return true } // no mode bits to read

func TestCheckDirOther(t *testing.T) {
	d := t.TempDir()
	if err := checkDir(d); err != nil {
		t.Fatal(err)
	}
	if err := checkDir(filepath.Join(d, "missing")); err == nil {
		t.Error("a missing directory passed")
	}
	f := filepath.Join(d, "f")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkDir(f); err == nil {
		t.Error("a file passed")
	}
}
