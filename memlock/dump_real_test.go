//go:build linux || darwin

package memlock

import (
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sys/unix"
)

// The real DisableDumps would end the test binary's own core file and, on
// darwin, its debuggability, so it runs in a child: this binary again, with
// one test selected.
func TestDisableDumpsReal(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestDisableDumpsChild$", "-test.v")
	cmd.Env = append(os.Environ(), "MEMLOCK_DUMPS_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	t.Logf("%s", out)
}

func TestDisableDumpsChild(t *testing.T) {
	if os.Getenv("MEMLOCK_DUMPS_CHILD") == "" {
		t.Skip("runs only as the child of TestDisableDumpsReal")
	}
	if err := DisableDumps(); err != nil {
		t.Fatalf("DisableDumps: %v", err)
	}
	var lim unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_CORE, &lim); err != nil || lim.Cur != 0 || lim.Max != 0 {
		t.Fatalf("RLIMIT_CORE after DisableDumps: %+v %v", lim, err)
	}
	if err := childChecks(); err != nil {
		t.Fatal(err)
	}
}
