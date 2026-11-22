package redirect

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestLoopback(t *testing.T) {
	for _, c := range []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{"LocalHost", true},
		{"localhost.", true},
		{"app.localhost", true},
		{"a.b.localhost.", true},
		{"A_b-1.LOCALHOST", true},
		{"a.localhost", true},
		{"z.localhost", true},
		{"0.localhost", true},
		{"9.localhost", true},
		{"Z.localhost", true},
		{"`.localhost", false}, // 'a' - 1
		{"{.localhost", false}, // 'z' + 1
		{"/.localhost", false}, // '0' - 1
		{":.localhost", false}, // '9' + 1
		{"@.localhost", false}, // 'A' - 1
		{"[.localhost", false}, // 'Z' + 1
		{"a.é.localhost", false},
		{"localhost.é", false},
		{"127.0.0.1", true},
		{"127.255.255.254", true},
		{"::1", true},
		{"0:0:0:0:0:0:0:1", true},
		{"::ffff:127.0.0.1", true},

		{"", false},
		{".", false},
		{".localhost", false},
		{"a..localhost", false},
		{"a b.localhost", false},
		{"é.localhost", false},
		{"localhost.evil.example", false},
		{"evil-localhost", false},
		{"notlocalhost", false},
		{"xlocalhost.", false},
		{"localhost..", false},
		{"127.evil.example", false},
		{"127.0.0.1.evil.example", false},
		{"127.0.0.1.", false},
		{"127.1", false},
		{"0x7f.1", false},
		{"0x7f.0.0.1", false},
		{"2130706433", false},
		{"017700000001", false},
		{"[::1]", false},
		{"::1%eth0", false},
		{"::", false},
		{"0.0.0.0", false},
		{"10.0.0.1", false},
		{"128.0.0.1", false},
		{"::ffff:10.0.0.1", false},
		{"example.com", false},
		{"Kocalhost", false},  // Kelvin sign: folds to k, not part of "localhost" anyway
		{"locaLſhost", false}, // long s: EqualFold would call it localhost
	} {
		if got := Loopback(c.host); got != c.want {
			t.Errorf("Loopback(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

func TestHTTPSOrLoopback(t *testing.T) {
	for _, c := range []struct {
		url string
		ok  bool
	}{
		{"https://example.com", true},
		{"https://example.com:8443/x?y=z#f", true},
		{"HTTPS://Example.com/", true},
		{"https://127.evil.example/", true}, // https needs no loopback
		{"http://localhost", true},
		{"http://localhost:8080/x", true},
		{"http://127.0.0.1:9/x", true},
		{"http://[::1]:9/x", true},
		{"http://[::ffff:127.0.0.1]:9/x", true},
		{"http://app.localhost/", true},
		{"http://LOCALHOST./", true},
		{"http://evil.example@localhost/", true}, // what follows the last @ is the host Go dials

		{"", false},
		{"http", false},
		{"/relative", false},
		{"https://", false},
		{"https:///path", false},
		{"https:path", false},
		{"http://", false},
		{"http://example.com", false},
		{"http://127.evil.example/", false},
		{"http://127.0.0.1.evil.example/", false},
		{"http://0x7f.1/", false},
		{"http://[::1%25eth0]/", false},
		{"http://localhost@evil.example/", false}, // the host is what follows the LAST @
		{"http://localhost:80@evil.example/", false},
		{"ftp://localhost/", false},
		{"file:///etc/passwd", false},
		{"javascript:alert(1)", false},
		{"https://%zz/", false},
		{"http://localhost:notaport/", false},
	} {
		err := HTTPSOrLoopback(c.url)
		if (err == nil) != c.ok {
			t.Errorf("HTTPSOrLoopback(%q) = %v, want ok=%v", c.url, err, c.ok)
		}
	}
}

// An error is logged, and a URL may carry a password or a token.
func TestErrorsDoNotEchoTheURL(t *testing.T) {
	for _, raw := range []string{
		"http://user:hunter2@example.com/?token=SECRET",
		"ftp://user:hunter2@example.com/?token=SECRET",
		"https://user:hunter2@/?token=SECRET",
		"https://%zz/?token=SECRET:hunter2",
	} {
		err := HTTPSOrLoopback(raw)
		if err == nil {
			t.Fatalf("%q was accepted", raw)
		}
		if s := err.Error(); strings.Contains(s, "SECRET") || strings.Contains(s, "hunter2") {
			t.Errorf("the error for %q leaks it: %v", raw, err)
		}
	}
}

func req(t *testing.T, raw string) *http.Request {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Request{URL: u}
}

func TestCheckRedirect(t *testing.T) {
	via := func(n int) []*http.Request { return make([]*http.Request, n) }
	for _, c := range []struct {
		name string
		to   string
		hops int
		ok   bool
	}{
		{"https to https", "https://example.com/next", 1, true},
		{"https to loopback http", "http://127.0.0.1:1/x", 1, true},
		{"https to http", "http://example.com/x", 1, false},
		{"https to a DNS name that starts with 127", "http://127.evil.example/", 1, false},
		{"https to another scheme", "ftp://example.com/", 1, false},
		{"the ninth hop", "https://example.com/", MaxRedirects - 1, true},
		{"the tenth hop", "https://example.com/", MaxRedirects, false},
		{"far past it", "https://example.com/", 100, false},
	} {
		err := CheckRedirect(req(t, c.to), via(c.hops))
		if (err == nil) != c.ok {
			t.Errorf("%s: %v, want ok=%v", c.name, err, c.ok)
		}
	}
	if CheckRedirect(nil, nil) == nil || CheckRedirect(&http.Request{}, nil) == nil {
		t.Error("a redirect with no URL was followed")
	}
}

// The policy is wired into a real client: an https server that redirects to
// cleartext http on another host is not followed, and a redirect inside
// https is.
func TestClientRefusesCleartextRedirect(t *testing.T) {
	var hit bool
	plain := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer plain.Close()
	// 127.0.0.1 is loopback, so the redirect target is rewritten to a name that
	// is not: the test must not rely on this machine's resolver.
	target := strings.Replace(plain.URL, "127.0.0.1", "example.invalid", 1)

	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.Write([]byte("fine"))
		case "/hop":
			http.Redirect(w, r, "/ok", http.StatusFound)
		default:
			http.Redirect(w, r, target, http.StatusFound)
		}
	}))
	defer tls.Close()
	c := tls.Client()
	c.CheckRedirect = CheckRedirect

	if res, err := c.Get(tls.URL + "/hop"); err != nil || res.StatusCode != 200 {
		t.Fatalf("a redirect inside https: %v %v", res, err)
	}
	res, err := c.Get(tls.URL + "/leave")
	if err == nil {
		res.Body.Close()
		t.Fatal("a redirect to cleartext http was followed")
	}
	if !strings.Contains(err.Error(), "leaves https") {
		t.Errorf("unexpected error: %v", err)
	}
	if hit {
		t.Error("the cleartext server was reached")
	}
}

func TestRedirectLoopStops(t *testing.T) {
	var srv *httptest.Server
	hits := 0
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bounded, so that a policy that never stops fails here instead of
		// hanging the suite (the client's own limit is ten as well, but a
		// CheckRedirect that returns nil is the one under test).
		if hits++; hits > 50 {
			return
		}
		http.Redirect(w, r, srv.URL+"/again", http.StatusFound)
	}))
	defer srv.Close()
	c := srv.Client()
	c.CheckRedirect = CheckRedirect
	res, err := c.Get(srv.URL)
	if err == nil {
		res.Body.Close()
		t.Fatalf("an endless redirect was followed %d times", hits)
	}
	if !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Errorf("unexpected error: %v", err)
	}
	// The first request and nine redirects were made; the tenth redirect was refused.
	if hits != MaxRedirects {
		t.Errorf("the server was hit %d times, want %d", hits, MaxRedirects)
	}
}
