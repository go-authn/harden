package redirect

import (
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

// oracle is Loopback written a second way, from the RFC's words, so that the
// fuzzer compares two readings instead of the function with itself.
func oracle(host string) bool {
	if strings.ContainsAny(host, "%[]/@ ") {
		return false
	}
	if a, err := netip.ParseAddr(host); err == nil {
		if a.Is4In6() {
			a = netip.AddrFrom4(a.As4())
		}
		return a.IsLoopback()
	}
	for i := 0; i < len(host); i++ {
		if host[i] >= 0x80 {
			return false
		}
	}
	h := strings.ToLower(host)
	if strings.HasSuffix(h, ".") {
		h = h[:len(h)-1]
	}
	if h == "localhost" {
		return true
	}
	if !strings.HasSuffix(h, ".localhost") || strings.HasPrefix(h, ".") {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(h, ".localhost"), ".") {
		if label == "" {
			return false
		}
		for _, r := range label {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789-_", r) {
				return false
			}
		}
	}
	return true
}

func FuzzLoopback(f *testing.F) {
	for _, s := range []string{
		"localhost", "LOCALHOST", "localhost.", "a.localhost", "127.0.0.1", "::1", "[::1]",
		"::ffff:127.0.0.1", "127.evil.example", "0x7f.1", "127.1", "2130706433",
		"::1%eth0", "", ".", ".localhost", "Kocalhost",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, host string) {
		got := Loopback(host)
		if got != oracle(host) {
			t.Fatalf("Loopback(%q) = %v, the oracle says %v", host, got, !got)
		}
		// Never a string prefix: a DNS name that starts like an address.
		if got && strings.HasPrefix(host, "127.") {
			if _, err := netip.ParseAddr(host); err != nil {
				t.Fatalf("%q is no IP literal and was taken for loopback", host)
			}
		}
		// Letter case does not matter, for ASCII.
		if Loopback(strings.ToUpper(host)) != got && !strings.ContainsFunc(host, func(r rune) bool { return r >= 0x80 }) {
			t.Fatalf("Loopback(%q) depends on letter case", host)
		}
	})
}

func FuzzHTTPSOrLoopback(f *testing.F) {
	for _, s := range []string{
		"https://example.com", "http://localhost:8080/x", "http://127.0.0.1/", "http://[::1]:9/",
		"http://[::ffff:127.0.0.1]/", "http://127.evil.example/", "http://0x7f.1/", "http://LOCALHOST./",
		"http://localhost@evil.example/", "http://user@evil.example@localhost/", "https:///x", "",
		"ftp://localhost/", "http://a.b.localhost/", "http://[::1%25eth0]/", "https://%zz/",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		err := HTTPSOrLoopback(raw)
		u, perr := url.Parse(raw)
		if perr != nil {
			if err == nil {
				t.Fatalf("%q does not parse and was accepted", raw)
			}
			return
		}
		want := u.Hostname() != "" && (u.Scheme == "https" || u.Scheme == "http" && oracle(u.Hostname()))
		if (err == nil) != want {
			t.Fatalf("HTTPSOrLoopback(%q) = %v, want accepted=%v", raw, err, want)
		}
		if pw, ok := u.User.Password(); err != nil && ok && len(pw) > 3 && !strings.Contains(u.Scheme+u.Hostname(), pw) && strings.Contains(err.Error(), pw) {
			t.Fatalf("the error for %q echoes the password: %v", raw, err)
		}
	})
}
