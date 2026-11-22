//go:build unix

package privfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func modeOK(m os.FileMode) bool { return m.Perm() == 0o600 }

func TestCheckDir(t *testing.T) {
	d := t.TempDir()
	if err := checkDir(d); err != nil {
		t.Fatal(err)
	}
	if err := checkDir(filepath.Join(d, "missing")); err == nil {
		t.Error("a missing directory passed")
	}

	link := filepath.Join(d, "link")
	if err := os.Symlink(d, link); err != nil {
		t.Fatal(err)
	}
	if err := checkDir(link); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("a symlinked directory: %v", err)
	}

	f := filepath.Join(d, "f")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkDir(f); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("a file: %v", err)
	}

	for _, m := range []os.FileMode{0o770, 0o707, 0o777, 0o720, 0o702} {
		w := filepath.Join(d, "w"+m.String())
		if err := os.Mkdir(w, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(w, m); err != nil {
			t.Fatal(err)
		}
		if err := checkDir(w); err == nil || !strings.Contains(err.Error(), "writable by group or others") {
			t.Errorf("mode %v: %v", m, err)
		}
	}
	for _, m := range []os.FileMode{0o700, 0o750, 0o755, 0o500} {
		r := filepath.Join(d, "r"+m.String())
		if err := os.Mkdir(r, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(r, m); err != nil {
			t.Fatal(err)
		}
		if err := checkDir(r); err != nil {
			t.Errorf("mode %v: %v", m, err)
		}
	}

	old := getuid
	defer func() { getuid = old }()
	getuid = func() int { return os.Getuid() + 1 }
	if err := checkDir(d); err == nil || !strings.Contains(err.Error(), "another user") {
		t.Errorf("another user's directory: %v", err)
	}
}

func TestWriteRefusesALooseDirectoryBeforeWriting(t *testing.T) {
	d := t.TempDir()
	if err := os.Chmod(d, 0o775); err != nil {
		t.Fatal(err)
	}
	if err := Write(filepath.Join(d, "cred"), []byte("secret")); err == nil || !strings.Contains(err.Error(), "writable by group or others") {
		t.Fatalf("got %v", err)
	}
	if es, _ := os.ReadDir(d); len(es) != 0 {
		t.Errorf("something was written: %v", es)
	}
}

// A link left at the target is replaced, never followed: what it points at is
// untouched.
func TestWriteDoesNotFollowASymlink(t *testing.T) {
	d := t.TempDir()
	victim := filepath.Join(d, "victim")
	if err := os.WriteFile(victim, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(d, "cred")
	if err := os.Symlink(victim, target); err != nil {
		t.Fatal(err)
	}
	if err := Write(target, []byte("secret")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "precious" {
		t.Errorf("the link was followed: victim is %q", b)
	}
	if st, err := os.Lstat(target); err != nil || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm() != 0o600 {
		t.Errorf("target: %v %v", st, err)
	}
	if st, _ := os.Stat(victim); st.Mode().Perm() != 0o644 {
		t.Errorf("the link's target changed mode: %v", st.Mode())
	}
}

// A file of loose mode that is already there does not keep its mode.
func TestWriteTightensALooseFile(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "cred")
	if err := os.WriteFile(p, []byte("old"), 0o666); err != nil {
		t.Fatal(err)
	}
	os.Chmod(p, 0o666)
	if err := Write(p, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", st.Mode())
	}
}

// Whatever the umask, the file is 0600.
func TestWriteIgnoresTheUmask(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "cred")
	if err := Write(p, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", st.Mode())
	}
}
