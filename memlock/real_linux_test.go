//go:build linux

package memlock

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// vmLck is the kernel's count of this process's locked memory, in kB.
func vmLck(t *testing.T) int {
	t.Helper()
	f, err := os.Open("/proc/self/status")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "VmLck:"); ok {
			kb, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "kB")))
			if err != nil {
				t.Fatal(err)
			}
			return kb
		}
	}
	t.Fatal("no VmLck line in /proc/self/status")
	return 0
}

// assertLocked reads the state back from the kernel: VmLck counts the locked
// kilobytes of the whole process, and nothing else in this test binary locks.
func assertLocked(t *testing.T, b []byte, want bool) {
	t.Helper()
	got := vmLck(t)
	if want && got < len(b)/1024 || !want && got != 0 {
		t.Fatalf("VmLck is %d kB with a %d-byte buffer, locked=%v", got, len(b), want)
	}
	t.Logf("VmLck %d kB, locked=%v", got, want)
}
