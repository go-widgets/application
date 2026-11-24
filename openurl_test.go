package application

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckURLAccepts(t *testing.T) {
	for in, want := range map[string]string{
		"https://example.com":                  "https://example.com",
		"http://example.com/a?b=c#d":           "http://example.com/a?b=c#d",
		"HTTPS://Example.COM/Path":             "https://Example.COM/Path",
		"https://idp.example:8443/auth?x=%20y": "https://idp.example:8443/auth?x=%20y",
		"https://[::1]:3000/cb":                "https://[::1]:3000/cb",
		"https://user@example.com/":            "https://user@example.com/",
	} {
		got, err := checkURL(in)
		if err != nil || got != want {
			t.Errorf("checkURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestCheckURLRefusesOtherSchemes(t *testing.T) {
	for _, in := range []string{
		"file:///etc/passwd",
		"file://host/share/x",
		`\\server\share\x.exe`,
		"smb://host/share",
		"ftp://example.com/",
		"javascript:alert(1)",
		"data:text/html,<script>1</script>",
		"mailto:a@b.c",
		"ms-msdt:/id PCWDiagnostic",
		"steam://run/1",
		"x-apple.systempreferences:com.apple.preference",
		"vscode://file/etc/passwd",
		"hxxp://example.com",
		"httpx://example.com",
		"ssh://example.com",
		"//example.com/path", // scheme-relative
		"example.com/path",   // no scheme
		"-flag",              // option-shaped
		"HtTp:opaque",        // right scheme, not hierarchical -> no host
	} {
		_, err := checkURL(in)
		if err == nil {
			t.Errorf("checkURL(%q) accepted", in)
			continue
		}
		if !errors.Is(err, ErrURLScheme) && !errors.Is(err, ErrURLInvalid) {
			t.Errorf("checkURL(%q) err = %v is not a typed refusal", in, err)
		}
	}
	// The scheme refusals specifically carry ErrURLScheme.
	for _, in := range []string{"file:///x", "smb://h/s", "javascript:1", "example.com/x", "//e.com"} {
		if _, err := checkURL(in); !errors.Is(err, ErrURLScheme) {
			t.Errorf("checkURL(%q) = %v, want ErrURLScheme", in, err)
		}
	}
}

func TestCheckURLAcceptsTheHighestPort(t *testing.T) {
	if got, err := checkURL("http://example.com:65535/x"); err != nil || got != "http://example.com:65535/x" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestCheckURLRefusesMalformed(t *testing.T) {
	for _, in := range []string{
		"",
		"http://",
		"https:///path",
		"http:// example.com",
		"http://exa mple.com/",
		"http://example.com/\n",
		"http://example.com/\x00",
		"http://example.com/\x7f",
		"http://[::1",
		"http://example.com:99999999/",
		"http://example.com:65536/",
		"https://:443/",
		" https://example.com",
		"https://example.com/" + strings.Repeat("a", maxURLLength),
	} {
		if _, err := checkURL(in); !errors.Is(err, ErrURLInvalid) {
			t.Errorf("checkURL(%.40q) = %v, want ErrURLInvalid", in, err)
		}
	}
}

func TestOpenURLPassesTheCanonicalFormOnly(t *testing.T) {
	var got []string
	saved := openURLFunc
	openURLFunc = func(u string) error { got = append(got, u); return nil }
	defer func() { openURLFunc = saved }()

	if err := OpenURL("HTTPS://Example.com/x"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "https://Example.com/x" {
		t.Fatalf("OS saw %v", got)
	}
	// A refused URL never reaches the OS boundary.
	if err := OpenURL("file:///etc/passwd"); !errors.Is(err, ErrURLScheme) || len(got) != 1 {
		t.Fatalf("err = %v, OS calls = %d", err, len(got))
	}
	// An OS failure comes back as it is.
	boom := errors.New("boom")
	openURLFunc = func(string) error { return boom }
	if err := OpenURL("https://example.com"); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func FuzzCheckURL(f *testing.F) {
	for _, s := range []string{"https://example.com", "file:///x", "", "http://[::1", "HTTP://a/\x00", `\\h\s`, "javascript:1", "http://a b"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, err := checkURL(s)
		if err != nil {
			return
		}
		// The invariant the whole boundary rests on: whatever is accepted is an
		// http(s) URL with a host, and re-checking it changes nothing.
		if !strings.HasPrefix(got, "http://") && !strings.HasPrefix(got, "https://") {
			t.Fatalf("accepted %q as %q", s, got)
		}
		for _, r := range got {
			if r < 0x20 || r == 0x7f {
				t.Fatalf("control character in accepted %q", got)
			}
		}
		again, err := checkURL(got)
		if err != nil || again != got {
			t.Fatalf("not idempotent: %q -> %q, %v", got, again, err)
		}
	})
}
