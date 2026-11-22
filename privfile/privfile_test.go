package privfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func names(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func TestWriteCreatesAndReplaces(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cred")
	for _, body := range []string{"first", "second, longer than the first", ""} {
		if err := Write(p, []byte(body)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(p)
		if err != nil || string(got) != body {
			t.Fatalf("read back %q, %v; want %q", got, err, body)
		}
		if st, _ := os.Stat(p); !modeOK(st.Mode()) {
			t.Errorf("mode %v", st.Mode())
		}
	}
	if got := names(t, dir); len(got) != 1 || got[0] != "cred" {
		t.Errorf("the directory holds %v: a temporary file was left", got)
	}
}

func TestWriteRefusesADirectoryAtThePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "d")
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Write(p, []byte("x")); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("got %v", err)
	}
	if got := names(t, dir); len(got) != 1 {
		t.Errorf("the directory holds %v", got)
	}
}

func TestWriteInAMissingOrNonDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := Write(filepath.Join(dir, "no", "such", "cred"), []byte("x")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing directory: %v", err)
	}
	f := filepath.Join(dir, "file")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(filepath.Join(f, "cred"), []byte("x")); err == nil {
		t.Error("a file where the directory goes was accepted")
	}
}

// Lstat of the path failing for another reason than absence is an error, not a
// licence to write.
func TestWriteLstatErrorOtherThanAbsent(t *testing.T) {
	dir := t.TempDir()
	long := filepath.Join(dir, strings.Repeat("n", 5000))
	if err := Write(long, []byte("x")); err == nil {
		t.Error("an unusable name was written")
	}
}

type fake struct {
	werr, cherr, serr, clerr error
	closed                   bool
	name                     string
}

func (f *fake) Write(b []byte) (int, error) { return len(b), f.werr }
func (f *fake) Chmod(fs.FileMode) error     { return f.cherr }
func (f *fake) Sync() error                 { return f.serr }
func (f *fake) Close() error                { f.closed = true; return f.clerr }
func (f *fake) Name() string                { return f.name }

func TestFillReportsEachStepAndAlwaysCloses(t *testing.T) {
	boom := errors.New("boom")
	cases := map[string]*fake{
		"write": {werr: boom}, "sync": {serr: boom}, "close": {clerr: boom},
	}
	// Only where protect asks the file for something can the file refuse.
	for name, f := range protectCases(boom) {
		cases[name] = f
	}
	for name, f := range cases {
		if err := fill(f, []byte("x")); !errors.Is(err, boom) {
			t.Errorf("%s: %v", name, err)
		}
		if !f.closed {
			t.Errorf("%s: the file was left open", name)
		}
	}
	if err := fill(&fake{}, []byte("x")); err != nil {
		t.Errorf("a clean fill: %v", err)
	}
}

func TestWriteFailureLeavesNothingBehind(t *testing.T) {
	boom := errors.New("boom")
	origTemp, origRename := createTemp, rename
	defer func() { createTemp, rename = origTemp, origRename }()

	dir := t.TempDir()
	p := filepath.Join(dir, "cred")
	if err := Write(p, []byte("old")); err != nil {
		t.Fatal(err)
	}

	createTemp = func(string, string) (namedFile, error) { return nil, boom }
	if err := Write(p, []byte("new")); !errors.Is(err, boom) {
		t.Errorf("createTemp: %v", err)
	}

	createTemp = func(d, pat string) (namedFile, error) {
		f, err := os.CreateTemp(d, pat)
		if err != nil {
			return nil, err
		}
		return failingWrite{f, boom}, nil
	}
	if err := Write(p, []byte("new")); !errors.Is(err, boom) {
		t.Errorf("write: %v", err)
	}

	createTemp = origTemp
	rename = func(string, string) error { return boom }
	if err := Write(p, []byte("new")); !errors.Is(err, boom) {
		t.Errorf("rename: %v", err)
	}

	got, _ := os.ReadFile(p)
	if string(got) != "old" {
		t.Errorf("a failed write changed the file: %q", got)
	}
	if n := names(t, dir); len(n) != 1 {
		t.Errorf("the directory holds %v", n)
	}
}

type failingWrite struct {
	*os.File
	err error
}

func (f failingWrite) Write([]byte) (int, error) { return 0, f.err }
